package core

import "time"

// Accumulator is the handle that inputs use to emit metrics during a
// Gather call. The agent passes a concrete implementation that knows
// where to route the metric (to outputs, processors, etc.).
//
// We mirror Telegraf's shape but drop the SetPrecision / WithTracking
// methods we don't have a use case for yet.
type Accumulator interface {
	AddFields(measurement string, fields map[string]any, tags map[string]string, t ...time.Time)
	AddGauge(measurement string, fields map[string]any, tags map[string]string, t ...time.Time)
	AddCounter(measurement string, fields map[string]any, tags map[string]string, t ...time.Time)

	// AddMetric is the lower-level entry point — useful when a plugin
	// already has a fully-formed Metric (e.g. after Copy()).
	AddMetric(m *Metric)

	// AddError records a non-fatal collection error. The agent logs
	// these but keeps the pipeline running.
	AddError(err error)
}
