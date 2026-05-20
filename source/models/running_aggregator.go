package models

import (
	"log/slog"
	"sync"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// RunningAggregatorConfig describes how the agent drives an Aggregator.
type RunningAggregatorConfig struct {
	// Period is the aggregation window size. Push() fires at every
	// periodEnd to emit a summary, then advances [periodStart,
	// periodEnd] forward by Period and resets plugin state.
	Period time.Duration

	// DropOriginal: when true, metrics consumed by this aggregator
	// do NOT continue on to outputs — only the aggregated summary
	// does. With multiple aggregators, if any one says true the
	// original is dropped for everyone (aggregator filtering is
	// not per-output).
	DropOriginal bool

	// Grace lets Add accept metrics whose timestamps fall up to
	// Grace BEFORE the current periodStart. Useful when upstream
	// stages introduce small latency that pushes "fresh" metrics
	// into the just-closed window.
	Grace time.Duration

	// Delay is the mirror of Grace for metrics whose timestamps fall
	// up to Delay AFTER the current periodEnd. Useful for cross-host
	// scenarios where clocks aren't perfectly synced.
	Delay time.Duration

	// Filter is the per-aggregator selector. Metrics that fail
	// Select() are NOT aggregated (the stream isn't this
	// aggregator's concern); the original still flows on to
	// outputs regardless of DropOriginal. Selected metrics are
	// Modify'd on the aggregator's private copy.
	Filter Filter
}

// RunningAggregator wraps an Aggregator with the configuration the
// agent needs to drive its Add/Push lifecycle plus the window state
// (periodStart, periodEnd) that decides which metrics belong to this
// aggregation round. Add() is called from the fanout goroutine; Push()
// runs in its own pushLoop goroutine. The mutex guards both the
// underlying plugin's state and the window timestamps.
//
// Log is set by Agent.New with `plugin=aggregators.<name>` baked in;
// defaults to slog.Default() for standalone use.
type RunningAggregator struct {
	Aggregator core.Aggregator
	Config     RunningAggregatorConfig
	Log        *slog.Logger

	mu             sync.Mutex
	periodStart    time.Time
	periodEnd      time.Time
	metricsDropped int64 // metrics rejected by Add() for falling outside the window
}

func NewRunningAggregator(a core.Aggregator, cfg RunningAggregatorConfig) *RunningAggregator {
	if cfg.Period <= 0 {
		cfg.Period = 30 * time.Second
	}
	return &RunningAggregator{Aggregator: a, Config: cfg, Log: slog.Default()}
}

// Period exposes the configured aggregation interval.
func (r *RunningAggregator) Period() time.Duration {
	return r.Config.Period
}

// EndPeriod is the wall-clock time at which the current window
// closes. The agent's pushLoop reads this every iteration to compute
// `time.Until(EndPeriod())` as the next sleep — this is what gives
// the loop drift-free behavior and natural recovery from clock jumps
// (sleep / hibernate).
func (r *RunningAggregator) EndPeriod() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.periodEnd
}

// UpdateWindow sets [periodStart, periodEnd] for the round about to
// begin. Called once at startup before any Add() lands, and again
// inside Push() to advance the window.
func (r *RunningAggregator) UpdateWindow(start, end time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.periodStart = start
	r.periodEnd = end
}

// MetricsDropped returns the lifetime count of metrics rejected by
// Add() for falling outside [periodStart-Grace, periodEnd+Delay].
func (r *RunningAggregator) MetricsDropped() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.metricsDropped
}

// Add submits a metric to the aggregator. Returns true if the original
// metric should be dropped from continuing to outputs.
//
// Metrics whose timestamp falls outside [periodStart-Grace,
// periodEnd+Delay] are NOT passed to the plugin — they're counted in
// MetricsDropped — but DropOriginal still applies. The aggregator's
// "this stream is mine" decision is independent of whether any one
// metric was inside the current window.
func (r *RunningAggregator) Add(m *core.Metric) bool {
	// Filter Select runs BEFORE the window/state lock — Select is
	// read-only on m and on the (frozen-after-Compile) filter, so
	// no synchronization is needed. A "not selected" metric isn't
	// this aggregator's concern: leave the original alone (return
	// false, ignoring DropOriginal).
	if !r.Config.Filter.Select(m) {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Skip the window check before the first UpdateWindow lands so
	// metrics arriving in the brief startup window aren't all dropped.
	if !r.periodEnd.IsZero() {
		t := m.Time
		if t.Before(r.periodStart.Add(-r.Config.Grace)) || t.After(r.periodEnd.Add(r.Config.Delay)) {
			r.metricsDropped++
			return r.Config.DropOriginal
		}
	}

	// Copy before handing to the plugin: the original pointer also
	// flows to outputs (when DropOriginal=false), and the plugin may
	// hold this reference until the next Push — far past the point
	// where outputs would have serialized and released it. Copying
	// here also gives us a metric we own so Modify() is safe.
	mc := m.Copy()
	r.Config.Filter.Modify(mc)
	if len(mc.Fields) == 0 {
		// All fields were excluded; nothing to aggregate.
		return r.Config.DropOriginal
	}
	r.Aggregator.Add(mc)
	return r.Config.DropOriginal
}

// Push emits the current window's aggregated metrics via acc, advances
// the window by Period, and resets plugin state. Wall-clock recovery:
// if the agent was suspended (laptop sleep, VM hibernate) the
// configured next window may already be in the past — we detect that
// by comparing time.Now to [periodEnd, periodEnd+Period] and snap to
// the current wall-clock window if we're outside it.
//
// Truncate(-1) strips the monotonic clock reading from time values.
// Without it, comparing pre-sleep periodEnd (with mono clock) to a
// post-sleep nowWall gives misleading "before/after" results because
// the mono clock pauses during sleep.
func (r *RunningAggregator) Push(acc core.Accumulator) {
	r.mu.Lock()
	defer r.mu.Unlock()

	since := r.periodEnd
	until := r.periodEnd.Add(r.Config.Period)

	nowWall := time.Now().Truncate(-1)
	if nowWall.Before(since.Truncate(-1)) || nowWall.After(until.Truncate(-1)) {
		since = nowWall.Truncate(r.Config.Period)
		until = since.Add(r.Config.Period)
	}
	r.periodStart = since
	r.periodEnd = until

	r.Aggregator.Push(acc)
	r.Aggregator.Reset()
}
