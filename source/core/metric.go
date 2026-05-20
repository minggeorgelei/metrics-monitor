// Package core defines the central interfaces and value types for the
// metrics-monitor pipeline (Metric, Input, Output, Processor, Aggregator,
// Accumulator, Initializer). The shapes deliberately mirror
// influxdata/telegraf so readers familiar with that project can map
// concepts directly, but the package name is generic so it doesn't
// pretend to be Telegraf itself.
package core

import (
	"maps"
	"time"
)

// ValueType describes how a metric's value should be interpreted by
// downstream consumers (outputs, aggregators).
type ValueType int

const (
	Untyped ValueType = iota
	Counter
	Gauge
	Summary
	Histogram
)

// Metric is the unit of data flowing through the pipeline:
// inputs produce them, processors/aggregators may transform them,
// outputs serialize them somewhere. The fields are exported so that
// outputs can JSON-encode the struct directly.
type Metric struct {
	Name   string            `json:"name"`
	Tags   map[string]string `json:"tags,omitempty"`
	Fields map[string]any    `json:"fields"`
	Time   time.Time         `json:"time"`
	Type   ValueType         `json:"type,omitempty"`
}

// Copy returns a deep copy. The buffer + transaction protocol can
// retain references across goroutines, so callers that need to keep
// a snapshot must Copy rather than store the original pointer.
func (m *Metric) Copy() *Metric {
	tags := make(map[string]string, len(m.Tags))
	maps.Copy(tags, m.Tags)
	fields := make(map[string]any, len(m.Fields))
	maps.Copy(fields, m.Fields)
	return &Metric{
		Name:   m.Name,
		Tags:   tags,
		Fields: fields,
		Time:   m.Time,
		Type:   m.Type,
	}
}
