// Package count is a minimal aggregator: tally incoming metrics by
// name and emit "<name>_count" with the running tally at every Push.
// Doubles as an integration test fixture — Push() goroutine firing is
// directly observable in the output, and (when wired with
// [[aggregator_processors.X]]) those emissions traversing the
// agg-processor chain are also observable.
package count

import (
	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/aggregators"
)

type Count struct {
	counts map[string]int64
}

func (*Count) Name() string { return "count" }

// Init satisfies core.Initializer so the agent can pre-allocate the
// state map before the first Add() lands.
func (c *Count) Init() error {
	c.counts = map[string]int64{}
	return nil
}

func (c *Count) Add(m *core.Metric) {
	c.counts[m.Name]++
}

func (c *Count) Push(acc core.Accumulator) {
	for name, n := range c.counts {
		acc.AddCounter(name+"_count", map[string]any{"count": n}, nil)
	}
}

func (c *Count) Reset() {
	c.counts = map[string]int64{}
}

func init() {
	aggregators.Add("count", func() core.Aggregator {
		return &Count{}
	})
}
