package models

import (
	"log/slog"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// RunningOutputConfig describes how the agent drives an Output.
type RunningOutputConfig struct {
	Name  string
	Alias string
	// FlushInterval is the upper bound on how long a metric will sit
	// in the buffer before the agent attempts to write it.
	FlushInterval time.Duration

	// MetricBatchSize is the maximum number of metrics passed to a
	// single Output.Write call. Larger batches amortise per-call
	// overhead at the cost of write latency.
	MetricBatchSize int

	// MetricBufferLimit caps the in-memory buffer size. Once full,
	// new metrics evict the oldest (FIFO drop).
	MetricBufferLimit int

	// Filter is the per-output selector. Applied in AddMetric:
	// rejected metrics aren't buffered. Filtered modifications run
	// on a private copy so they don't affect what other outputs see.
	Filter Filter
}

// RunningOutput wraps an Output with its dedicated buffer and the
// configuration the agent needs to drive a flush loop.
//
// The agent owns the flush goroutine; this struct only exposes:
//   - AddMetric: ingest from the accumulator/fan-out
//   - Flush:     drive one Transaction (Begin → Write → End)
//
// Log is set by Agent.New with `plugin=outputs.<name>` baked in;
// defaults to slog.Default() for standalone use.
type RunningOutput struct {
	Output core.Output
	Config RunningOutputConfig
	Buffer Buffer
	Log    *slog.Logger
}

func NewRunningOutput(out core.Output, cfg RunningOutputConfig) *RunningOutput {
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MetricBatchSize <= 0 {
		cfg.MetricBatchSize = 1000
	}
	if cfg.MetricBufferLimit <= 0 {
		cfg.MetricBufferLimit = 10000
	}
	return &RunningOutput{
		Output: out,
		Config: cfg,
		Buffer: NewMemoryBuffer(cfg.MetricBufferLimit),
		Log:    slog.Default(),
	}
}

// AddMetric enqueues a metric for eventual write. Applies the
// per-output filter first: rejected metrics never reach the buffer,
// and the modification step runs on a private copy so it doesn't
// affect what other outputs see for the same underlying metric.
func (r *RunningOutput) AddMetric(m *core.Metric) {
	if !r.Config.Filter.Select(m) {
		return
	}
	if r.Config.Filter.IsActive() {
		// Copy iff there's any chance Modify will mutate. IsActive
		// covers both selectActive and modifyActive — selectActive
		// alone returns from the Select branch above, so reaching
		// here means modifyActive is on (or both are).
		m = m.Copy()
		r.Config.Filter.Modify(m)
		if len(m.Fields) == 0 {
			return
		}
	}
	r.Buffer.Add(m)
}

func (r *RunningOutput) LogName() string {
	return logName("output", r.Config.Name, r.Config.Alias)
}

// Flush moves one batch from buffer to output. Returns the error from
// Output.Write so the caller can log/decide on retry policy. Metrics
// are Accepted on success, Kept (re-queued) on error — matching
// Telegraf's default semantics. Outputs that want to express "this
// metric is permanently bad" can populate tx.Reject themselves by
// implementing a custom Output that owns its own Buffer interaction,
// but Phase 1 doesn't need that.
func (r *RunningOutput) Flush() error {
	tx := r.Buffer.BeginTransaction(r.Config.MetricBatchSize)
	if len(tx.Batch) == 0 {
		r.Buffer.EndTransaction(tx)
		return nil
	}

	err := r.Output.Write(tx.Batch)
	if err == nil {
		tx.AcceptAll()
	}
	// On error: leave Accept/Reject empty; EndTransaction will
	// re-queue everything via InferKeep.
	r.Buffer.EndTransaction(tx)
	return err
}

// Tick computes the next flush time from the configured interval.
// Exposed for the agent's ticker setup.
func (r *RunningOutput) Tick() time.Duration { return r.Config.FlushInterval }

func (r *RunningOutput) Close() {
	if err := r.Output.Close(); err != nil {
		r.Log.Error("close output", "err", err)
	}

	if err := r.Buffer.Close(); err != nil {
		r.Log.Error("close output buffer", "err", err)
	}
}
