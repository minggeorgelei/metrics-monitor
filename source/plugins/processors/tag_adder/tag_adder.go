// Package tag_adder is the simplest possible processor: copy a
// configured tag set onto every metric that flows through it. Useful
// as an integration test fixture for the processor chain (and the
// aggregator-side processor chain — wire it under
// [[aggregator_processors.tag_adder]] to mark which metrics went
// through that path).
package tag_adder

import (
	"maps"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/processors"
)

type TagAdder struct {
	// Tags is the set of key=value pairs copied onto every metric.
	// TOML:
	//   [[processors.tag_adder]]
	//     [processors.tag_adder.tags]
	//     env = "prod"
	//     region = "us-east-1"
	Tags map[string]string `toml:"tags"`
}

func (*TagAdder) Name() string { return "tag_adder" }

func (t *TagAdder) Apply(in ...*core.Metric) []*core.Metric {
	for _, m := range in {
		if m.Tags == nil {
			m.Tags = make(map[string]string, len(t.Tags))
		}
		maps.Copy(m.Tags, t.Tags)
	}
	return in
}

func init() {
	processors.Add("tag_adder", func() core.Processor {
		return &TagAdder{Tags: map[string]string{}}
	})
}
