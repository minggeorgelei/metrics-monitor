// Package agent contains the main pipeline driver: it starts input
// gather loops, fans the produced metrics out to every output, and
// drives each output's buffered flush loop. The lifecycle mirrors
// Telegraf's agent package, scoped to what Phase 1 needs.
package agent

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/models"
	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// Config holds agent-level knobs that aren't specific to any plugin.
type Config struct {
	// MetricChannelSize is the buffer between accumulator emits and
	// the fan-out goroutine. Larger absorbs short stalls in outputs;
	// too large just delays back-pressure.
	MetricChannelSize int
}

func (c Config) withDefaults() Config {
	if c.MetricChannelSize <= 0 {
		c.MetricChannelSize = 1000
	}
	return c
}

// Agent owns the runtime of one pipeline instance.
type Agent struct {
	cfg     Config
	inputs  []*models.RunningInput
	outputs []*models.RunningOutput
	log     *slog.Logger
}

func New(cfg Config, inputs []*models.RunningInput, outputs []*models.RunningOutput, log *slog.Logger) *Agent {
	if log == nil {
		log = slog.Default()
	}
	return &Agent{
		cfg:     cfg.withDefaults(),
		inputs:  inputs,
		outputs: outputs,
		log:     log,
	}
}

// Run starts the pipeline and blocks until ctx is canceled, then
// performs a graceful shutdown:
//
//  1. Stop input tickers — no new metrics enter the pipeline.
//  2. Wait for any in-flight Gather call to finish.
//  3. Close the metric channel — fan-out drains and exits.
//  4. Trigger a final flush on every output, wait for them to exit.
//  5. Call Output.Close() on each output.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.connectOutputs(); err != nil {
		// Try to close anything we did connect, then bail.
		_ = a.closeOutputs()
		return err
	}

	metrics := make(chan *core.Metric, a.cfg.MetricChannelSize)
	// done is closed once we want inputs to stop emitting. It is a
	// separate signal from ctx.Done so that we can stop *inputs*
	// before stopping *outputs* — outputs still need to drain the
	// channel after inputs are quiet.
	inputDone := make(chan struct{})

	var inputWg, fanoutWg, outputWg sync.WaitGroup

	// --- inputs ---
	for _, ri := range a.inputs {
		inputWg.Add(1)
		go func(ri *models.RunningInput) {
			defer inputWg.Done()
			a.runInput(ctx, ri, metrics, inputDone)
		}(ri)
	}

	// --- fan-out ---
	fanoutWg.Add(1)
	go func() {
		defer fanoutWg.Done()
		a.runFanout(metrics)
	}()

	// --- outputs ---
	// Outputs use a separate context that we cancel only after the
	// fan-out has finished draining. This guarantees every emitted
	// metric reaches every output's buffer before the flush loops
	// quit.
	outputCtx, cancelOutputs := context.WithCancel(context.Background())
	defer cancelOutputs()
	for _, ro := range a.outputs {
		outputWg.Add(1)
		go func(ro *models.RunningOutput) {
			defer outputWg.Done()
			a.runOutput(outputCtx, ro)
		}(ro)
	}

	a.log.Info("agent started",
		"inputs", len(a.inputs),
		"outputs", len(a.outputs),
		"channel_size", a.cfg.MetricChannelSize)

	// Block until external shutdown signal.
	<-ctx.Done()
	a.log.Info("agent shutting down")

	// Drain order: inputs → fan-out → outputs.
	close(inputDone)
	inputWg.Wait()
	close(metrics)
	fanoutWg.Wait()

	cancelOutputs()
	outputWg.Wait()

	if err := a.closeOutputs(); err != nil {
		return err
	}
	a.log.Info("agent stopped")
	return nil
}

func (a *Agent) connectOutputs() error {
	for _, ro := range a.outputs {
		if err := ro.Output.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) closeOutputs() error {
	var firstErr error
	for _, ro := range a.outputs {
		if err := ro.Output.Close(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			a.log.Error("close output", "name", ro.Output.Name(), "err", err)
		}
	}
	return firstErr
}

// runInput drives one input on its configured interval. It exits on
// inputDone (graceful shutdown) or ctx.Done (hard abort).
func (a *Agent) runInput(ctx context.Context, ri *models.RunningInput, out chan<- *core.Metric, inputDone <-chan struct{}) {
	name := ri.Input.Name()
	acc := models.NewAccumulator(out, inputDone, func(err error) {
		a.log.Error("input error", "name", name, "err", err)
	}, name)

	ticker := time.NewTicker(ri.Config.Interval)
	defer ticker.Stop()

	a.log.Debug("input started", "name", name, "interval", ri.Config.Interval)

	// Gather once immediately so the first sample doesn't require
	// waiting one full interval — UI feels much snappier.
	a.gather(ri, acc)

	for {
		select {
		case <-inputDone:
			a.log.Debug("input stopping", "name", name)
			return
		case <-ctx.Done():
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

// runFanout broadcasts each emitted metric to every output's buffer.
// Sharing pointers is safe in Phase 1 because nothing in the pipeline
// mutates a metric after creation. Once processors land we'll need
// to Copy() per output.
func (a *Agent) runFanout(in <-chan *core.Metric) {
	for m := range in {
		for _, ro := range a.outputs {
			ro.AddMetric(m)
		}
	}
	a.log.Debug("fan-out drained")
}

// runOutput drives one output's flush loop. Triggers a flush every
// FlushInterval, plus one final flush after ctx is canceled so that
// metrics still in the buffer get written before exit.
func (a *Agent) runOutput(ctx context.Context, ro *models.RunningOutput) {
	name := ro.Output.Name()
	ticker := time.NewTicker(ro.Tick())
	defer ticker.Stop()

	a.log.Debug("output started", "name", name, "flush_interval", ro.Tick())

	for {
		select {
		case <-ctx.Done():
			// Final drain: keep flushing until the buffer is empty
			// or we hit a write error. This is best-effort.
			a.drainOutput(ro)
			a.log.Debug("output stopped", "name", name)
			return
		case <-ticker.C:
			if err := ro.Flush(); err != nil {
				a.log.Error("output flush", "name", name, "err", err)
			}
		}
	}
}

// drainOutput repeatedly flushes until the buffer is empty or a write
// error happens. Used at shutdown.
func (a *Agent) drainOutput(ro *models.RunningOutput) {
	for ro.Buffer.Len() > 0 {
		if err := ro.Flush(); err != nil {
			a.log.Error("output final flush", "name", ro.Output.Name(), "err", err)
			return
		}
	}
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
