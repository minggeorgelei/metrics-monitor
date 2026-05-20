// Package processors is the processor-plugin registry. Concrete
// processor plugins live in subpackages (rename, regex, filter, ...)
// and self-register via an init() function that calls Add(). See
// source/plugins/inputs for the rationale.
package processors

import "github.com/minggeorgelei/metrics-monitor/source/core"

// Creator builds a fresh plugin instance with sensible defaults.
type Creator func() core.Processor

// Processors is the global name→Creator map.
var Processors = map[string]Creator{}

// Add registers a plugin Creator under the given name. Panics on
// duplicate registration to surface mistakes at startup.
func Add(name string, creator Creator) {
	if _, exists := Processors[name]; exists {
		panic("processors.Add: duplicate plugin name " + name)
	}
	Processors[name] = creator
}
