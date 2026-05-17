package models

import (
	"log"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// RunningOutputConfig describes how the agent drives an Output.
type RunningOutputConfig struct {
	// FlushInterval is the upper bound on how long a metric will sit
	// in the buffer before the agent attempts to write it.
	FlushInterval time.Duration

	// MetricBatchSize is the maximum number of metrics passed to a
	// single Output.Write call. Larger batches amortise per-call
	// overhead at the cost of write latency.
	MetricBatchSize int

	// MetricBufferLimit caps the in-memory buffer size. Once full,
	// new metrics evict the oldest (FIFO drop).
	MetricBufferLimit int
}

// RunningOutput wraps an Output with its dedicated buffer and the
// configuration the agent needs to drive a flush loop.
//
// The agent owns the flush goroutine; this struct only exposes:
//   - AddMetric: ingest from the accumulator/fan-out
//   - Flush:     drive one Transaction (Begin → Write → End)
type RunningOutput struct {
	Output core.Output
	Config RunningOutputConfig
	Buffer Buffer
}

func NewRunningOutput(out core.Output, cfg RunningOutputConfig) *RunningOutput {
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MetricBatchSize <= 0 {
		cfg.MetricBatchSize = 1000
	}
	if cfg.MetricBufferLimit <= 0 {
		cfg.MetricBufferLimit = 10000
	}
	return &RunningOutput{
		Output: out,
		Config: cfg,
		Buffer: NewMemoryBuffer(cfg.MetricBufferLimit),
	}
}

// AddMetric enqueues a metric for eventual write.
func (r *RunningOutput) AddMetric(m *core.Metric) {
	r.Buffer.Add(m)
}

// Flush moves one batch from buffer to output. Returns the error from
// Output.Write so the caller can log/decide on retry policy. Metrics
// are Accepted on success, Kept (re-queued) on error — matching
// Telegraf's default semantics. Outputs that want to express "this
// metric is permanently bad" can populate tx.Reject themselves by
// implementing a custom Output that owns its own Buffer interaction,
// but Phase 1 doesn't need that.
func (r *RunningOutput) Flush() error {
	tx := r.Buffer.BeginTransaction(r.Config.MetricBatchSize)
	if len(tx.Batch) == 0 {
		r.Buffer.EndTransaction(tx)
		return nil
	}

	err := r.Output.Write(tx.Batch)
	if err == nil {
		tx.AcceptAll()
	}
	// On error: leave Accept/Reject empty; EndTransaction will
	// re-queue everything via InferKeep.
	r.Buffer.EndTransaction(tx)
	return err
}

// Tick computes the next flush time from the configured interval.
// Exposed for the agent's ticker setup.
func (r *RunningOutput) Tick() time.Duration { return r.Config.FlushInterval }

func (r *RunningOutput) Close() {
	if err := r.Output.Close(); err != nil {
		log.Printf("Error closing output: %v", err)
	}

	if err := r.Buffer.Close(); err != nil {
		log.Printf("Error closing output buffer: %v", err)
	}
}
