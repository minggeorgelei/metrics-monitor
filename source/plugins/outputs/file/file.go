// Package file is the simplest possible output: write each metric as
// one JSON object per line (NDJSON) to a file path or to a standard
// stream. Phase 1 deliberately skips rotation, compression, and
// multi-writer fan-in — those can be added later without touching the
// Output interface.
package file

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/outputs"
)

// File writes metrics as NDJSON. Path is one of:
//   - "stdout" — write to os.Stdout
//   - "stderr" — write to os.Stderr
//   - anything else — open as an append-only file
type File struct {
	Path string `toml:"path"`

	mu     sync.Mutex // guards writer; Write may be called from multiple flush loops in the future
	writer *bufio.Writer
	closer io.Closer // nil for stdout/stderr
}

func (*File) Name() string { return "file" }

func (f *File) Connect() error {
	switch f.Path {
	case "stdout":
		f.writer = bufio.NewWriter(os.Stdout)
	case "stderr":
		f.writer = bufio.NewWriter(os.Stderr)
	default:
		// O_APPEND so concurrent runs don't truncate each other;
		// O_CREATE so a missing path is fine on first run.
		fp, err := os.OpenFile(f.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open %q: %w", f.Path, err)
		}
		f.writer = bufio.NewWriter(fp)
		f.closer = fp
	}
	return nil
}

func (f *File) Close() error {
	var firstErr error
	if f.writer != nil {
		if err := f.writer.Flush(); err != nil {
			firstErr = err
		}
	}
	if f.closer != nil {
		if err := f.closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (f *File) Write(metrics []*core.Metric) error {
	if f.writer == nil {
		return errors.New("file output not connected")
	}

	enc := json.NewEncoder(f.writer)
	for _, m := range metrics {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("encode metric: %w", err)
		}
	}
	// Flush per Write call so live tailing (`tail -f metrics.ndjson`)
	// stays responsive. A buffered writer in bufio is only useful
	// inside a single Write — between calls we want bytes on disk.
	return f.writer.Flush()
}

// init self-registers the plugin with the outputs registry. Imported
// indirectly from main.go via a blank import; runs before main().
func init() {
	outputs.Add("file", func() core.Output {
		return &File{Path: "stdout"}
	})
}
