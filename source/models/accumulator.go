package models

import (
	"maps"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// Accumulator is the agent-side implementation of core.Accumulator.
// Plugins (inputs, aggregators) invoke its AddFields / AddGauge /
// AddCounter methods to emit metrics; each call produces a
// *core.Metric and forwards it on `out`.
//
// With the channel-per-stage pipeline, `out` is the dst channel of
// the stage the plugin sits in (input.dst, aggregator.aggC, ...). The
// caller is expected to drain that channel — sends are unconditional.
// Shutdown is driven by closing upstream channels, not by a "done"
// signal on the producer side.
//
// `tagKey` / `tagValue`: when both are non-empty, every emitted metric
// is auto-tagged with that pair (e.g. tagKey="input", tagValue="cpu").
// Empty key means "no auto-tag".
//
// `filter`: optional Filter applied to every emit. Rejected metrics
// are silently dropped; modifications run on the freshly-built
// metric so there's no aliasing concern.
type Accumulator struct {
	out      chan<- *core.Metric
	errSink  func(error)
	tagKey   string
	tagValue string
	filter   *Filter
}

func NewAccumulator(out chan<- *core.Metric, errSink func(error), tagKey, tagValue string) *Accumulator {
	return &Accumulator{out: out, errSink: errSink, tagKey: tagKey, tagValue: tagValue}
}

// WithFilter attaches a per-plugin filter. The returned Accumulator
// drops metrics rejected by Select and Modify's the rest in place.
// Pass nil to clear.
func (a *Accumulator) WithFilter(f *Filter) *Accumulator {
	a.filter = f
	return a
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
	if a.filter != nil {
		if !a.filter.Select(m) {
			return
		}
		if a.filter.IsActive() {
			// Defensive copy in case the caller still holds m.
			m = m.Copy()
			a.filter.Modify(m)
			if len(m.Fields) == 0 {
				return
			}
		}
	}
	a.out <- m
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

	// Copy maps so the plugin can reuse its own scratch buffers.
	outTags := make(map[string]string, len(tags)+1)
	maps.Copy(outTags, tags)
	if a.tagKey != "" {
		outTags[a.tagKey] = a.tagValue
	}
	outFields := make(map[string]any, len(fields))
	maps.Copy(outFields, fields)

	m := &core.Metric{
		Name:   measurement,
		Tags:   outTags,
		Fields: outFields,
		Time:   tm,
		Type:   vt,
	}
	if a.filter != nil {
		if !a.filter.Select(m) {
			return
		}
		a.filter.Modify(m) // safe: we just built m, no aliasing
		if len(m.Fields) == 0 {
			return
		}
	}
	a.out <- m
}
