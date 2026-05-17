// Package outputs is the output-plugin registry. Concrete output
// plugins live in subpackages (file, websocket, ...) and self-register
// via an init() function that calls Add(). See source/plugins/inputs
// for the rationale.
package outputs

import "github.com/minggeorgelei/metrics-monitor/source/core"

// Creator builds a fresh plugin instance with sensible defaults.
type Creator func() core.Output

// Outputs is the global name→Creator map.
var Outputs = map[string]Creator{}

// Add registers a plugin Creator under the given name. Panics on
// duplicate registration to surface mistakes at startup.
func Add(name string, creator Creator) {
	if _, exists := Outputs[name]; exists {
		panic("outputs.Add: duplicate plugin name " + name)
	}
	Outputs[name] = creator
}
