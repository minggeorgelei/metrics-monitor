// Package inputs is the input-plugin registry. Concrete input plugins
// live in subpackages (cpu, mem, disk, ...) and self-register via an
// init() function that calls Add().
//
// The registry is the bridge between the string name in a TOML config
// (e.g. [[inputs.cpu]]) and a fresh Go instance of the corresponding
// plugin struct. Each Creator returns a new instance with sensible
// defaults; the config loader then overlays the user's TOML values.
package inputs

import "github.com/minggeorgelei/metrics-monitor/source/core"

// Creator builds a fresh plugin instance with sensible defaults. The
// config loader calls this once per [[inputs.<name>]] block found in
// the TOML config, then decodes the block into the returned value.
type Creator func() core.Input

// Inputs is the global name→Creator map. Plugin packages mutate this
// from their init() function during package initialization (before
// main() runs).
var Inputs = map[string]Creator{}

// Add registers a plugin Creator under the given name. Called from
// each plugin package's init(). Panics on duplicate registration to
// catch typos and accidental double-imports at startup.
func Add(name string, creator Creator) {
	if _, exists := Inputs[name]; exists {
		panic("inputs.Add: duplicate plugin name " + name)
	}
	Inputs[name] = creator
}
