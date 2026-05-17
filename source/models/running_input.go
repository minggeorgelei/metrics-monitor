package models

import (
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// RunningInputConfig holds the agent-side knobs that wrap a raw Input.
type RunningInputConfig struct {
	// Interval is how often the agent calls Gather on this input.
	// Different inputs can run at different cadences (CPU every
	// second, disk every 30 seconds, etc.).
	Interval time.Duration
}

// RunningInput pairs an Input plugin with the configuration the agent
// needs to drive it. The agent owns the goroutine and ticker; this
// struct is just a config bundle.
type RunningInput struct {
	Input  core.Input
	Config RunningInputConfig
}

func NewRunningInput(in core.Input, cfg RunningInputConfig) *RunningInput {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	return &RunningInput{Input: in, Config: cfg}
}
