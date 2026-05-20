// Package aggregators is the aggregator-plugin registry. Concrete
// aggregator plugins live in subpackages (basicstats, histogram, ...)
// and self-register via an init() function that calls Add(). See
// source/plugins/inputs for the rationale.
package aggregators

import "github.com/minggeorgelei/metrics-monitor/source/core"

// Creator builds a fresh plugin instance with sensible defaults.
type Creator func() core.Aggregator

// Aggregators is the global name→Creator map.
var Aggregators = map[string]Creator{}

// Add registers a plugin Creator under the given name. Panics on
// duplicate registration to surface mistakes at startup.
func Add(name string, creator Creator) {
	if _, exists := Aggregators[name]; exists {
		panic("aggregators.Add: duplicate plugin name " + name)
	}
	Aggregators[name] = creator
}
