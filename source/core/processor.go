package core

// Processor transforms metrics one-at-a-time as they flow from inputs
// toward outputs. Examples in the Telegraf ecosystem include rename,
// filter, regex, defaults, converter, enum.
//
// Apply is allowed to return 0, 1, or many metrics for each input:
//   - 0 metrics => drop (e.g. filter plugin)
//   - 1 metric  => transform in place
//   - N metrics => fan-out (e.g. clone plugin)
//
// We define the interface in Phase 1 so the agent's processing chain
// can be wired through. Concrete plugins ship in later phases.
type Processor interface {
	Name() string
	Apply(in ...*Metric) []*Metric
}
