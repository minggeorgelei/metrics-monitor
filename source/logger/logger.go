// Package logger is a small slog-based logging facade with pluggable
// output formats and a per-plugin sub-logger helper that mirrors
// Telegraf's logger.New(category, name, alias).
//
// Design:
//   - Format selection is registry-based. Each format ("text",
//     "structured", ...) registers a HandlerFactory in its file's
//     init(). Setup() looks up the factory by name and installs the
//     resulting slog.Handler as slog.Default. Add a new format by
//     dropping a new file with another init() — no edits here.
//   - Per-plugin context is just slog attributes. For(cat, name)
//     returns a sub-logger pre-decorated with `plugin=<cat>.<name>`,
//     so plugin-side and agent-side log calls don't repeat the
//     plugin identity each line.
//
// Output examples (msg="started" emitted via logger.For("inputs",
// "cpu").Info(...)):
//
//	text:        time=... level=INFO msg=started plugin=inputs.cpu
//	structured:  {"time":"...","level":"INFO","msg":"started","plugin":"inputs.cpu"}
//
// Telegraf parity scope: text + structured. Telegraf's event_logger
// (Windows event log), early-log buffer, callback registry, and
// stdlog redirector are intentionally NOT ported — see the design
// discussion in the project notes for rationale.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config is the input to Setup(). All fields are optional; zero
// values apply sensible defaults (text format, info level, stderr).
type Config struct {
	Format string // "text" | "structured"
	Level  string // "debug" | "info" | "warn" | "error"
	File   string // path; empty = stderr
}

// HandlerFactory builds a slog.Handler over the given writer at the
// given minimum level. Registered factories live in registry.
type HandlerFactory func(w io.Writer, level slog.Level) slog.Handler

var registry = map[string]HandlerFactory{}

// Register adds a handler factory under the given format name.
// Called from each format file's init().
func Register(name string, f HandlerFactory) {
	if _, exists := registry[name]; exists {
		panic("logger.Register: duplicate format " + name)
	}
	registry[name] = f
}

// Setup builds the root logger from cfg, installs it as slog.Default,
// and returns it for callers that want a direct handle. The returned
// io.Closer should be invoked on shutdown to flush/close any file
// handle; nil when the writer is stderr (no close needed).
func Setup(cfg Config) (*slog.Logger, io.Closer, error) {
	if cfg.Format == "" {
		cfg.Format = "text"
	}
	factory, ok := registry[cfg.Format]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported log_format %q (registered: %v)", cfg.Format, registeredNames())
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, nil, err
	}

	w, closer, err := openWriter(cfg.File)
	if err != nil {
		return nil, nil, err
	}

	log := slog.New(factory(w, level))
	slog.SetDefault(log)
	return log, closer, nil
}

// For returns a sub-logger pre-decorated with `plugin=<cat>.<name>`.
// Safe to call before Setup() — falls back to slog.Default() (stderr,
// text, info-level) until Setup installs the configured one.
func For(category, name string) *slog.Logger {
	return slog.Default().With("plugin", category+"."+name)
}

// --- internal helpers ---

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log_level %q (debug|info|warn|error)", s)
	}
}

// openWriter resolves Config.File into a writer. Empty path = stderr
// (no closer needed); any other path opens append+create.
func openWriter(path string) (io.Writer, io.Closer, error) {
	if path == "" {
		return os.Stderr, nil, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", path, err)
	}
	return f, f, nil
}

func registeredNames() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
