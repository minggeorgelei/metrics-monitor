package models

import (
	"maps"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// Accumulator is the agent-side implementation of core.Accumulator.
// Inputs invoke its AddFields / AddGauge / AddCounter methods during
// Gather; each call produces a *core.Metric that is forwarded into
// the agent's metric pipeline via the `out` channel.
//
// The accumulator is intentionally not the place where fan-out to
// outputs happens — that's the agent's job. This keeps Accumulator
// thin and lets us swap fan-out strategies (broadcast / round-robin /
// per-output filtering) without touching every input.
type Accumulator struct {
	out      chan<- *core.Metric
	done     <-chan struct{} // closed during shutdown; emit drops rather than blocking
	errSink  func(error)
	inputTag string // optional; if non-empty, added as "input" tag on every metric
}

// NewAccumulator builds an Accumulator that pushes metrics into `out`.
// `done` is the signal that the agent is shutting down: once closed,
// further emits become drops rather than blocking sends. errSink
// receives any non-fatal errors reported by inputs via AddError;
// pass nil to discard them.
func NewAccumulator(out chan<- *core.Metric, done <-chan struct{}, errSink func(error), inputTag string) *Accumulator {
	return &Accumulator{out: out, done: done, errSink: errSink, inputTag: inputTag}
}

func (a *Accumulator) AddFields(measurement string, fields map[string]any, tags map[string]string, t ...time.Time) {
	a.emit(measurement, fields, tags, core.Untyped, t...)
}

func (a *Accumulator) AddGauge(measurement string, fields map[string]any, tags map[string]string, t ...time.Time) {
	a.emit(measurement, fields, tags, core.Gauge, t...)
}

func (a *Accumulator) AddCounter(measurement string, fields map[string]any, tags map[string]string, t ...time.Time) {
	a.emit(measurement, fields, tags, core.Counter, t...)
}

func (a *Accumulator) AddMetric(m *core.Metric) {
	if m == nil {
		return
	}
	a.send(m)
}

func (a *Accumulator) AddError(err error) {
	if err == nil || a.errSink == nil {
		return
	}
	a.errSink(err)
}

func (a *Accumulator) emit(measurement string, fields map[string]any, tags map[string]string, vt core.ValueType, t ...time.Time) {
	if len(fields) == 0 {
		return
	}
	tm := time.Now()
	if len(t) > 0 {
		tm = t[0]
	}

	// Copy maps so the input can reuse its own scratch buffers.
	// Add the "input" tag so consumers can demultiplex without
	// relying on metric-name conventions alone.
	outTags := make(map[string]string, len(tags)+1)
	maps.Copy(outTags, tags)
	if a.inputTag != "" {
		outTags["input"] = a.inputTag
	}
	outFields := make(map[string]any, len(fields))
	maps.Copy(outFields, fields)

	a.send(&core.Metric{
		Name:   measurement,
		Tags:   outTags,
		Fields: outFields,
		Time:   tm,
		Type:   vt,
	})
}

// send delivers a metric to the agent, falling back to drop if the
// agent has already signalled shutdown. This prevents inputs from
// hanging on a closed/full channel during the brief window between
// "ctx canceled" and "input goroutine actually returns".
func (a *Accumulator) send(m *core.Metric) {
	if a.done == nil {
		a.out <- m
		return
	}
	select {
	case a.out <- m:
	case <-a.done:
		// shutting down — drop
	}
}
