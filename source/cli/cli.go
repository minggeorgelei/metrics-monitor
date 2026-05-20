// Package cli is the user-facing entry point. After the move to a
// TOML-driven plugin pipeline, the only knob worth a CLI flag is the
// config file location — everything else (plugin choice, intervals,
// log level/format/file) lives in the config.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/minggeorgelei/metrics-monitor/source/agent"
	"github.com/minggeorgelei/metrics-monitor/source/config"
	"github.com/minggeorgelei/metrics-monitor/source/logger"
)

// CLI is the declarative kong root.
type CLI struct {
	Run RunCmd `cmd:"" help:"Start the metrics agent."`
}

type RunCmd struct {
	Config string `help:"Path to the TOML config file." default:"etc/metrics-monitor.toml" short:"c"`
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

	// Install the configured logger as slog.Default BEFORE agent.New
	// runs — agent.New derives per-plugin sub-loggers via With(), so
	// the format/level chosen here propagates to every plugin.
	log, closer, err := logger.Setup(logger.Config{
		Format: cfg.LogFormat,
		Level:  cfg.LogLevel,
		File:   cfg.LogFile,
	})
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	log.Info("config loaded",
		"path", c.Config,
		"inputs", len(cfg.Inputs),
		"processors", len(cfg.Processors),
		"aggregators", len(cfg.Aggregators),
		"agg_processors", len(cfg.AggProcessors),
		"outputs", len(cfg.Outputs))

	a := agent.New(cfg.Agent, cfg.Inputs, cfg.Processors, cfg.Aggregators, cfg.AggProcessors, cfg.Outputs, log)
	if err := a.Validate(); err != nil {
		return err
	}

	// Catch SIGINT/SIGTERM so a Ctrl-C drains the buffer cleanly
	// instead of dropping metrics on the floor.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return a.Run(ctx)
}
