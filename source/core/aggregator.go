package core

// Aggregator collects metrics across a time window and emits summary
// metrics at the end of each period (mean/min/max/p99/histogram/...).
// Lifecycle per period:
//
//   for each incoming metric: Add(m)
//   on period boundary:       Push(acc)  // emit aggregated metrics
//                             Reset()    // clear internal state
//
// We define the interface in Phase 1 so the agent's aggregation chain
// can be wired through. Concrete plugins (basicstats, histogram, ...)
// ship in later phases.
type Aggregator interface {
	Name() string
	Add(in *Metric)
	Push(acc Accumulator)
	Reset()
}
