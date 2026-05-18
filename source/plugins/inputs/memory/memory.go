package memory

import (
	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/inputs"
)

type Memory struct {
	CollectExpended bool `toml:"collect_expended"` // emit expended memory
}

// Gather implements core.Input.
func (m *Memory) Gather(acc core.Accumulator) error {
	panic("unimplemented")
}

func (*Memory) Name() string { return "memory" }

func init() {
	inputs.Add("memory", func() core.Input {
		return &Memory{
			CollectExpended: true,
		}
	})
}
