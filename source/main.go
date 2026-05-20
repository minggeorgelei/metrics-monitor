package main

import (
	"github.com/minggeorgelei/metrics-monitor/source/cli"

	// Blank imports trigger each plugin's init() function which
	// registers the plugin with its registry (inputs.Inputs or
	// outputs.Outputs). To add a new plugin, drop a line here and
	// the TOML config can immediately reference it.
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/inputs/cpu"
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/inputs/memory"

	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/outputs/file"
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/outputs/http_snapshot"
)

func main() {
	cli.Run()
}
