package models

import (
	"log/slog"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// RunningInputConfig holds the agent-side knobs that wrap a raw Input.
type RunningInputConfig struct {
	// Interval is how often the agent calls Gather on this input.
	// Different inputs can run at different cadences (CPU every
	// second, disk every 30 seconds, etc.).
	Interval time.Duration

	// Filter is the per-input namepass/namedrop/tagpass/tagdrop +
	// field/tag include/exclude policy. Applied to every metric
	// emitted by Gather() via the input's Accumulator. A zero-value
	// Filter is a no-op.
	Filter Filter
}

// RunningInput pairs an Input plugin with the configuration the agent
// needs to drive it. The agent owns the goroutine and ticker; this
// struct is just a config bundle.
//
// Log is set by Agent.New with `plugin=inputs.<name>` baked in via
// slog.Logger.With(); before agent construction it falls back to
// slog.Default() so test/CLI code that uses NewRunningInput
// standalone doesn't crash on nil. Callers should treat the field as
// read-after-Agent.New.
type RunningInput struct {
	Input  core.Input
	Config RunningInputConfig
	Log    *slog.Logger
}

func NewRunningInput(in core.Input, cfg RunningInputConfig) *RunningInput {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	return &RunningInput{Input: in, Config: cfg, Log: slog.Default()}
}
