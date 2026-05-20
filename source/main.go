package main

import (
	"github.com/minggeorgelei/metrics-monitor/source/cli"

	// Each `all` package fans out to one blank import per plugin in its
	// category, with a per-plugin build tag. The default build pulls
	// everything in; `-tags custom` excludes all plugins, and adding
	// e.g. `inputs.cpu outputs.file` re-enables a specific subset.
	// See source/plugins/inputs/all for the full convention.
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/aggregators/all"
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/inputs/all"
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/outputs/all"
	_ "github.com/minggeorgelei/metrics-monitor/source/plugins/processors/all"
)

func main() {
	cli.Run()
}
