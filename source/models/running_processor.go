package models

import (
	"log/slog"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// RunningProcessorConfig holds the agent-side knobs that wrap a raw
// Processor.
type RunningProcessorConfig struct {
	// Order determines execution order within the processor chain.
	// Lower runs first. Ties are broken by config-file iteration
	// order — which is non-deterministic across runs in Go map
	// iteration, so set Order explicitly when sequence matters.
	Order int

	// Filter is the per-processor selector. Metrics that fail
	// Select() bypass this processor unchanged and continue down
	// the pipeline; selected metrics are Modify'd (on a copy) and
	// then handed to Processor.Apply.
	Filter Filter
}

// RunningProcessor pairs a Processor plugin with its agent-side knobs.
// The agent applies the chain in ascending Order during fanout,
// between inputs and outputs.
//
// Log is set by Agent.New with the appropriate plugin category baked
// in (`processors.<name>` for main chain, `aggregator_processors.<name>`
// for the agg chain). Defaults to slog.Default() for standalone use.
type RunningProcessor struct {
	Processor core.Processor
	Config    RunningProcessorConfig
	Log       *slog.Logger
}

func NewRunningProcessor(p core.Processor, cfg RunningProcessorConfig) *RunningProcessor {
	return &RunningProcessor{Processor: p, Config: cfg, Log: slog.Default()}
}
