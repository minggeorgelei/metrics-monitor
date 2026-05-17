// Package cli is the user-facing entry point. After the move to a
// TOML-driven plugin pipeline, the only knob worth a CLI flag is the
// config file location — everything else (plugin choice, intervals,
// log level) lives in the config.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/minggeorgelei/metrics-monitor/source/agent"
	"github.com/minggeorgelei/metrics-monitor/source/config"
)

// CLI is the declarative kong root.
type CLI struct {
	Run RunCmd `cmd:"" help:"Start the metrics agent."`
}

type RunCmd struct {
	Config string `help:"Path to the TOML config file." default:"config/metrics-monitor.toml" short:"c"`
}

func Run() {
	var root CLI
	ctx := kong.Parse(&root,
		kong.Name("metrics-monitor"),
		kong.Description("Cross-platform host metrics agent (telegraf-style pipeline)."),
		kong.UsageOnError(),
	)
	if err := ctx.Run(&root); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func (c *RunCmd) Run(*CLI) error {
	cfg, err := config.Load(c.Config)
	if err != nil {
		return err
	}

	log := buildLogger(cfg.LogLevel)
	log.Info("config loaded",
		"path", c.Config,
		"inputs", len(cfg.Inputs),
		"outputs", len(cfg.Outputs))

	a := agent.New(cfg.Agent, cfg.Inputs, cfg.Outputs, log)
	if err := a.Validate(); err != nil {
		return err
	}

	// Catch SIGINT/SIGTERM so a Ctrl-C drains the buffer cleanly
	// instead of dropping metrics on the floor.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return a.Run(ctx)
}

func buildLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	// stderr so that file outputs configured for stdout (the default!)
	// do not collide with the agent's own log lines.
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
