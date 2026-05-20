// Package agent contains the main pipeline driver. The pipeline is
// modelled as a chain of stages, each running its own goroutine(s)
// and connected to neighbours by a buffered channel:
//
//	inputs ──ch──▶ processors ──ch──▶ aggregators ──ch──▶ outputs
//
// Construction is right-to-left: startOutputs returns the channel the
// upstream feeds into, startAggregators consumes that channel as its
// output and exposes a new src channel, and so on. Shutdown is driven
// by channel-close cascade: the agent's ctx cancels inputs, inputs
// close their dst, each downstream stage sees EOF on its src, drains,
// and closes its own dst.
//
// Mirrors Telegraf's agent package, scoped to what Phase 1 needs.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/internal"
	"github.com/minggeorgelei/metrics-monitor/source/models"
)

// Config holds agent-level knobs that aren't specific to any plugin.
type Config struct {
	// MetricChannelSize is the buffer between adjacent pipeline
	// stages (input→processor, processor→aggregator, etc.). Larger
	// absorbs short stalls; too large just delays back-pressure.
	MetricChannelSize int

	// SkipProcessorsAfterAggregators bypasses the aggregator-side
	// processor chain at runtime even when [[aggregator_processors.*]]
	// blocks are configured. Use it to temporarily disable that stage
	// without deleting TOML. Default false: configured agg processors
	// run.
	SkipProcessorsAfterAggregators bool
}

func (c Config) withDefaults() Config {
	if c.MetricChannelSize <= 0 {
		c.MetricChannelSize = 1000
	}
	return c
}

// Agent owns the runtime of one pipeline instance.
type Agent struct {
	cfg           Config
	inputs        []*models.RunningInput
	processors    []*models.RunningProcessor // before aggregators
	aggregators   []*models.RunningAggregator
	aggProcessors []*models.RunningProcessor // after aggregators (between Push and outputs)
	outputs       []*models.RunningOutput
	log           *slog.Logger
}

// --- Pipeline stage units ---

// inputUnit is a group of input plugins and the shared channel they
// write to.
//
//	┌───────┐
//	│ Input │───┐
//	└───────┘   │
//	┌───────┐   │     ______
//	│ Input │───┼──▶ ()_____)
//	└───────┘   │
//	┌───────┐   │
//	│ Input │───┘
//	└───────┘
type inputUnit struct {
	dst    chan<- *core.Metric
	inputs []*models.RunningInput
}

// processorUnit is one processor with its src/dst channels.
//
//	 ______     ┌───────────┐     ______
//	()_____)──▶ │ Processor │──▶ ()_____)
//	            └───────────┘
type processorUnit struct {
	src       <-chan *core.Metric
	dst       chan<- *core.Metric
	processor *models.RunningProcessor
}

// aggregatorUnit groups aggregators with their source channel and
// the two sink channels: aggC for Push() output and outputC for
// originals that pass through. Phase 1 uses aggC == outputC, so
// Push emissions go directly to outputs (no aggregator-side processor
// chain).
//
//	             ┌────────────┐
//	        ┌──▶ │ Aggregator │───┐
//	        │    └────────────┘   │
//	______  │    ┌────────────┐   │     ______
//	()_____)┼──▶ │ Aggregator │───┼──▶ ()_____)
//	        │    └────────────┘   │
//	        │                     │
//	        └──── original ───────┘
type aggregatorUnit struct {
	src         <-chan *core.Metric
	aggC        chan<- *core.Metric
	outputC     chan<- *core.Metric
	aggregators []*models.RunningAggregator
}

// outputUnit is a group of Outputs and their source channel. Metrics
// on the channel are broadcast to every output's buffer.
//
//	                          ┌────────┐
//	                     ┌──▶ │ Output │
//	                     │    └────────┘
//	______     ┌─────┐   │    ┌────────┐
//	()_____)──▶│ Fan │──▶│──▶ │ Output │
//	           └─────┘   │    └────────┘
//	                     │    ┌────────┐
//	                     └──▶ │ Output │
//	                          └────────┘
type outputUnit struct {
	src     <-chan *core.Metric
	outputs []*models.RunningOutput
}

func New(
	cfg Config,
	inputs []*models.RunningInput,
	processors []*models.RunningProcessor,
	aggregators []*models.RunningAggregator,
	aggProcessors []*models.RunningProcessor,
	outputs []*models.RunningOutput,
	log *slog.Logger,
) *Agent {
	if log == nil {
		log = slog.Default()
	}
	// Inject per-plugin loggers. Each Running* gets a sub-logger
	// derived from the agent's root logger with `plugin=<cat>.<name>`
	// baked in via With(). Subsequent log calls from inside the
	// wrapper or from agent code can use it without manually
	// repeating the plugin identity.
	for _, ri := range inputs {
		ri.Log = log.With("plugin", "inputs."+ri.Input.Name())
	}
	for _, rp := range processors {
		rp.Log = log.With("plugin", "processors."+rp.Processor.Name())
	}
	for _, ra := range aggregators {
		ra.Log = log.With("plugin", "aggregators."+ra.Aggregator.Name())
	}
	for _, rap := range aggProcessors {
		rap.Log = log.With("plugin", "aggregator_processors."+rap.Processor.Name())
	}
	for _, ro := range outputs {
		ro.Log = log.With("plugin", "outputs."+ro.Output.Name())
	}

	return &Agent{
		cfg:           cfg.withDefaults(),
		inputs:        inputs,
		processors:    sortByOrder(processors),
		aggregators:   aggregators,
		aggProcessors: sortByOrder(aggProcessors),
		outputs:       outputs,
		log:           log,
	}
}

// sortByOrder returns a copy sorted by RunningProcessorConfig.Order
// (ascending, stable). Used for both main and agg processor chains.
func sortByOrder(ps []*models.RunningProcessor) []*models.RunningProcessor {
	out := make([]*models.RunningProcessor, len(ps))
	copy(out, ps)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Config.Order < out[j].Config.Order
	})
	return out
}

// Run starts the pipeline and blocks until ctx is canceled. Shutdown
// is via channel-close cascade — see package doc.
func (a *Agent) Run(ctx context.Context) error {
	// Pipeline construction (right-to-left). At each step `next` is
	// the channel the upstream stage should write into.
	outputC, ou, err := a.startOutputs(ctx, a.outputs)
	if err != nil {
		return err
	}
	next := outputC

	// Optional agg-processor chain: sits between aggregator.aggC and
	// outputs. Aggregator Push() metrics traverse this chain;
	// pass-through originals bypass it (they go straight to outputC).
	// We build it BEFORE startAggregators so the aggregator unit's
	// aggC points at the agg-processor chain's input.
	var apu []*processorUnit
	aggProcEnabled := len(a.aggregators) > 0 &&
		len(a.aggProcessors) > 0 &&
		!a.cfg.SkipProcessorsAfterAggregators
	aggC := outputC
	if aggProcEnabled {
		aggC, apu = a.startProcessors(outputC, a.aggProcessors)
	}

	var au *aggregatorUnit
	if len(a.aggregators) != 0 {
		next, au = a.startAggregators(aggC, outputC)
	}

	var pu []*processorUnit
	if len(a.processors) != 0 {
		next, pu = a.startProcessors(next, a.processors)
	}

	iu := a.startInputs(next)

	a.log.Info("agent started",
		"inputs", len(a.inputs),
		"processors", len(a.processors),
		"aggregators", len(a.aggregators),
		"agg_processors", len(apu),
		"outputs", len(a.outputs),
		"channel_size", a.cfg.MetricChannelSize)

	// Spawn stage goroutines. Each goroutine reads from its src,
	// processes, and closes its dst on EOF so the next stage sees
	// the close.
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { defer wg.Done(); a.runOutputs(ou) }()

	if apu != nil {
		wg.Add(1)
		go func() { defer wg.Done(); a.runProcessors(apu) }()
	}
	if au != nil {
		wg.Add(1)
		go func() { defer wg.Done(); a.runAggregators(au) }()
	}
	if pu != nil {
		wg.Add(1)
		go func() { defer wg.Done(); a.runProcessors(pu) }()
	}

	wg.Add(1)
	go func() { defer wg.Done(); a.runInputs(ctx, iu) }()

	wg.Wait()

	a.log.Info("agent stopped successfully")
	return nil
}

// --- Pipeline construction helpers (right-to-left) ---
func (a *Agent) startOutputs(ctx context.Context, outputs []*models.RunningOutput) (chan *core.Metric, *outputUnit, error) {
	ch := make(chan *core.Metric, a.cfg.MetricChannelSize)
	unit := &outputUnit{src: ch}
	for _, ro := range outputs {
		if err := a.connectOutput(ctx, ro); err != nil {
			// Close any outputs that did connect before returning the error.
			for _, unitOutput := range unit.outputs {
				unitOutput.Close()
			}
			return nil, nil, fmt.Errorf("connecting output %s: %w", ro.LogName(), err)
		}
		unit.outputs = append(unit.outputs, ro)
	}
	return ch, unit, nil
}

func (a *Agent) startAggregators(aggC, outputC chan *core.Metric) (chan *core.Metric, *aggregatorUnit) {
	src := make(chan *core.Metric, a.cfg.MetricChannelSize)
	return src, &aggregatorUnit{
		src:         src,
		aggC:        aggC,
		outputC:     outputC,
		aggregators: a.aggregators,
	}
}

// startProcessors constructs a processor chain over `procs`. The
// chain is built right-to-left: the last processor writes to `dst`,
// the second-to-last writes to the last's src, etc. Returns the
// channel the upstream stage should feed into (the first processor's
// src).
//
// Used for both the main chain (a.processors, before aggregators)
// and the agg-processor chain (a.aggProcessors, after aggregators).
func (a *Agent) startProcessors(dst chan *core.Metric, procs []*models.RunningProcessor) (chan *core.Metric, []*processorUnit) {
	units := make([]*processorUnit, 0, len(procs))
	var src chan *core.Metric
	for i := len(procs) - 1; i >= 0; i-- {
		src = make(chan *core.Metric, a.cfg.MetricChannelSize)
		units = append(units, &processorUnit{
			src:       src,
			dst:       dst,
			processor: procs[i],
		})
		dst = src
	}
	return src, units
}

func (a *Agent) startInputs(dst chan *core.Metric) *inputUnit {
	return &inputUnit{dst: dst, inputs: a.inputs}
}

// --- Stage runners ---

// runInputs spawns one gatherLoop per input. When ctx is canceled all
// gather loops exit, and the shared dst channel is closed so the next
// stage sees EOF.
func (a *Agent) runInputs(ctx context.Context, unit *inputUnit) {
	var wg sync.WaitGroup
	for _, ri := range unit.inputs {
		wg.Add(1)
		go func(ri *models.RunningInput) {
			defer wg.Done()
			a.gatherLoop(ctx, ri, unit.dst)
		}(ri)
	}
	wg.Wait()
	close(unit.dst)
	a.log.Debug("input channel closed")
}

func (a *Agent) gatherLoop(ctx context.Context, ri *models.RunningInput, dst chan<- *core.Metric) {
	name := ri.Input.Name()
	acc := models.NewAccumulator(dst, func(err error) {
		ri.Log.Error("gather error", "err", err)
	}, "input", name).WithFilter(&ri.Config.Filter)

	ticker := time.NewTicker(ri.Config.Interval)
	defer ticker.Stop()

	ri.Log.Debug("input started", "interval", ri.Config.Interval)

	// Gather once immediately so the first sample doesn't require
	// waiting one full interval — UI feels much snappier.
	a.gather(ri, acc)

	for {
		select {
		case <-ctx.Done():
			ri.Log.Debug("input stopping")
			return
		case <-ticker.C:
			a.gather(ri, acc)
		}
	}
}

func (a *Agent) gather(ri *models.RunningInput, acc core.Accumulator) {
	if err := ri.Input.Gather(acc); err != nil {
		acc.AddError(err)
	}
}

// runProcessors spawns one goroutine per processor. Each reads from
// its src, applies its per-processor Filter (Select first — rejected
// metrics bypass the processor unchanged; selected metrics are
// Modify'd on a copy), runs Apply(), forwards results to dst, and on
// src EOF closes dst — cascading shutdown downstream.
func (a *Agent) runProcessors(units []*processorUnit) {
	var wg sync.WaitGroup
	for _, u := range units {
		wg.Add(1)
		go func(u *processorUnit) {
			defer wg.Done()
			f := &u.processor.Config.Filter
			for m := range u.src {
				if !f.Select(m) {
					// Not this processor's concern — pass through.
					u.dst <- m
					continue
				}
				if f.IsActive() {
					m = m.Copy()
					f.Modify(m)
					if len(m.Fields) == 0 {
						continue
					}
				}
				for _, x := range u.processor.Processor.Apply(m) {
					u.dst <- x
				}
			}
			close(u.dst)
			u.processor.Log.Debug("processor channel closed")
		}(u)
	}
	wg.Wait()
}

// runAggregators runs the aggregator stage. One goroutine reads from
// the upstream src and feeds Add() on every aggregator, then passes
// the original to outputC unless any aggregator's Add returns true
// (DropOriginal). One goroutine per aggregator drives its Push() at
// every periodEnd, emitting to aggC.
//
// Window initialization happens here (not in NewRunningAggregator) so
// that the window starts when the pipeline is actually live — closer
// to "first Add call" than to plugin construction time.
//
// Shutdown: src closes → reader exits → aggCtx cancel → per-agg Push
// goroutines do final push and exit → close(aggC) cascades downstream.
func (a *Agent) runAggregators(unit *aggregatorUnit) {
	startTime := time.Now()
	for _, ag := range unit.aggregators {
		ag.UpdateWindow(startTime, startTime.Add(ag.Period()))
	}

	aggCtx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	// Reader: pull from src, feed all aggregators, pass original to
	// outputC unless dropped.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for m := range unit.src {
			var dropOriginal bool
			for _, ag := range unit.aggregators {
				if ag.Add(m) {
					dropOriginal = true
				}
			}
			if !dropOriginal {
				unit.outputC <- m
			}
		}
		cancel()
	}()

	// Per-aggregator Push goroutine.
	for _, ag := range unit.aggregators {
		wg.Add(1)
		go func(ag *models.RunningAggregator) {
			defer wg.Done()
			name := ag.Aggregator.Name()
			acc := models.NewAccumulator(unit.aggC, func(err error) {
				ag.Log.Error("aggregator error", "err", err)
			}, "aggregator", name)
			a.pushLoop(aggCtx, ag, acc)
		}(ag)
	}

	wg.Wait()
	// aggC == outputC in Phase 1 — closing aggC closes the channel
	// that feeds outputs, completing the cascade.
	close(unit.aggC)
	a.log.Debug("aggregator channel closed")
}

// pushLoop drives one aggregator's Push() at every periodEnd. We
// recompute `time.Until(EndPeriod())` each iteration rather than
// using a fixed ticker — this gives:
//
//   - drift-free intervals: a slow Push catches up on the next sleep
//     without accumulating lag the way NewTicker would.
//   - clock-jump recovery: after sleep/hibernate, Push's wall-clock
//     check snaps EndPeriod to the new wall-clock window; the next
//     time.Until is computed against that fresh value.
func (a *Agent) pushLoop(ctx context.Context, ag *models.RunningAggregator, acc core.Accumulator) {
	ag.Log.Debug("aggregator started", "period", ag.Period())
	for {
		until := time.Until(ag.EndPeriod())
		select {
		case <-ctx.Done():
			ag.Push(acc) // final push so the in-progress window isn't lost
			ag.Log.Debug("aggregator stopped")
			return
		case <-time.After(until):
			ag.Push(acc)
		}
	}
}

// runOutputs reads from src and broadcasts each metric to every
// output's buffer. Per-output flush goroutines run in parallel and
// are canceled once src closes, doing one final drain.
func (a *Agent) runOutputs(unit *outputUnit) {
	flushCtx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	for _, ro := range unit.outputs {
		wg.Add(1)
		go func(ro *models.RunningOutput) {
			defer wg.Done()
			a.flushLoop(flushCtx, ro)
		}(ro)
	}

	for m := range unit.src {
		for _, ro := range unit.outputs {
			ro.AddMetric(m)
		}
	}
	a.log.Debug("output fan-in drained")

	cancel()
	wg.Wait()
}

func (a *Agent) flushLoop(ctx context.Context, ro *models.RunningOutput) {
	ticker := time.NewTicker(ro.Tick())
	defer ticker.Stop()
	ro.Log.Debug("output started", "flush_interval", ro.Tick())

	for {
		select {
		case <-ctx.Done():
			a.drainOutput(ro)
			ro.Log.Debug("output stopped")
			return
		case <-ticker.C:
			if err := ro.Flush(); err != nil {
				ro.Log.Error("output flush", "err", err)
			}
		}
	}
}

func (a *Agent) drainOutput(ro *models.RunningOutput) {
	for ro.Buffer.Len() > 0 {
		if err := ro.Flush(); err != nil {
			ro.Log.Error("output final flush", "err", err)
			return
		}
	}
}

// --- Output lifecycle ---

func (a *Agent) connectOutput(ctx context.Context, output *models.RunningOutput) error {
	if err := output.Output.Connect(); err != nil {
		if err := internal.SleepContext(ctx, 15*time.Second); err != nil {
			return err
		}

		if err = output.Output.Connect(); err != nil {
			return fmt.Errorf("error connecting to output %q: %w", output.LogName(), err)
		}
	}
	return nil
}

// ErrNoInputs / ErrNoOutputs are returned by validators; exported so
// callers (e.g. the CLI) can wrap them with a friendlier message.
var (
	ErrNoInputs  = errors.New("agent has no inputs configured")
	ErrNoOutputs = errors.New("agent has no outputs configured")
)

// Validate sanity-checks the configuration before Run.
func (a *Agent) Validate() error {
	if len(a.inputs) == 0 {
		return ErrNoInputs
	}
	if len(a.outputs) == 0 {
		return ErrNoOutputs
	}
	return nil
}
