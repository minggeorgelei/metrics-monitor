package logger

import (
	"io"
	"log/slog"
)

// Text format uses slog.TextHandler — one line per log record with
// `key=value` attributes. Cheap, grep-friendly, and the default when
// log_format is empty.
//
// Sample output:
//
//	time=2026-05-20T10:00:00Z level=INFO msg="agent started" plugin=inputs.cpu interval=1s
func init() {
	Register("text", func(w io.Writer, level slog.Level) slog.Handler {
		return slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	})
}
