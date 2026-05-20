package logger

import (
	"io"
	"log/slog"
)

// Structured format uses slog.JSONHandler — one JSON object per log
// record. Drop-in for log aggregators like Loki / Datadog / ELK that
// index by attribute.
//
// Sample output:
//
//	{"time":"2026-05-20T10:00:00Z","level":"INFO","msg":"agent started","plugin":"inputs.cpu","interval":"1s"}
func init() {
	Register("structured", func(w io.Writer, level slog.Level) slog.Handler {
		return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	})
}
