// Package models contains the runtime wrappers that sit between the
// pure interfaces in source/core and the agent's main loop:
//
//	Buffer          — bounded queue with Transaction semantics
//	RunningInput    — pairs an Input with its config (interval, ...)
//	RunningOutput   — pairs an Output with its buffer and flush logic
//	Accumulator impl — routes metrics from inputs to the agent
package models

import (
	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// Transaction is the unit of work between Buffer and Output. The
// output reads tx.Batch, attempts to deliver each metric, then fills
// in:
//
//	Accept: indices of metrics successfully delivered (gone for good)
//	Reject: indices of metrics that are definitively bad (drop, no retry)
//
// Anything in Batch not listed in either slice is implicitly "Keep" —
// re-queued by the buffer for a later retry. This three-way protocol
// (port of Telegraf's models.Transaction) lets outputs express partial
// success without the buffer needing to know each output's failure
// taxonomy.
type Transaction struct {
	Batch  []*core.Metric
	Accept []int
	Reject []int

	// valid is flipped to false in EndTransaction so that a stale
	// transaction object cannot be applied twice.
	valid bool
}

// AcceptAll marks every metric in the batch as successfully written.
// Outputs whose Write() returned nil typically use this shortcut.
func (tx *Transaction) AcceptAll() {
	tx.Accept = make([]int, len(tx.Batch))
	for i := range tx.Batch {
		tx.Accept[i] = i
	}
}

// InferKeep returns the indices of metrics that are neither in Accept
// nor Reject — i.e. metrics that need to be re-queued for retry.
// This is the "default is safe" rule: forgetting to mark a metric
// keeps it (re-tries) rather than silently losing it.
func (tx *Transaction) InferKeep() []int {
	used := make([]bool, len(tx.Batch))
	for _, i := range tx.Accept {
		used[i] = true
	}
	for _, i := range tx.Reject {
		used[i] = true
	}
	keep := make([]int, 0, len(tx.Batch))
	for i := range tx.Batch {
		if !used[i] {
			keep = append(keep, i)
		}
	}
	return keep
}

// BufferStats tracks the lifetime accounting for a buffer. Counters
// are monotonically increasing; Size and Capacity reflect the current
// state.
type BufferStats struct {
	Added    int64
	Written  int64 // matched an Accept index
	Rejected int64 // matched a Reject index OR dropped on EndTransaction overflow
	Dropped  int64 // evicted by Add() because the buffer was full
	Size     int64
	Capacity int64
}

type Buffer interface {
	Len() int

	// Add appends metrics. Returns the number of metrics dropped due
	// to capacity overflow (which is also reflected in Stats().Dropped).
	Add(metrics ...*core.Metric) int

	// BeginTransaction reserves up to batchSize of the oldest metrics
	// for an output to attempt delivery. The returned Batch is owned
	// by the caller until EndTransaction is invoked.
	BeginTransaction(batchSize int) *Transaction

	// EndTransaction closes the transaction: Accept-listed metrics
	// are released, Reject-listed metrics are dropped, anything else
	// is re-queued for later retry.
	EndTransaction(tx *Transaction)

	Stats() BufferStats
	Close() error
}
