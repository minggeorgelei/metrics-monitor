package models

import (
	"sync"

	"github.com/minggeorgelei/metrics-monitor/source/core"
)

// MemoryBuffer is a fixed-capacity circular buffer. The data layout
// is a port of telegraf/models.MemoryBuffer (with the BeginTransaction
// + EndTransaction protocol but without the deep TrackingMetric
// integration we don't need).
//
// Indexing model:
//
//	buf:    [    A    B    C    _    _ ]
//	             ^         ^
//	             first     last        size = 3, cap = 5
//
// Wrap-around handled by the next/prevby helpers.
type MemoryBuffer struct {
	mu sync.Mutex

	buf   []*core.Metric
	first int // index of the oldest metric in the buffer proper
	last  int // index one past the newest metric (next write goes here)
	size  int // metrics currently in the buffer
	cap   int

	// When a Transaction is active these track the slot range that
	// was lent out. EndTransaction needs them so that "keep" metrics
	// can be put back in the right place at the head of the buffer.
	batchFirst int
	batchSize  int

	stats BufferStats
}

func NewMemoryBuffer(capacity int) *MemoryBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &MemoryBuffer{
		buf:   make([]*core.Metric, capacity),
		cap:   capacity,
		stats: BufferStats{Capacity: int64(capacity)},
	}
}

func (b *MemoryBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.length()
}

func (b *MemoryBuffer) Add(metrics ...*core.Metric) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	dropped := 0
	for _, m := range metrics {
		dropped += b.add(m)
	}
	b.stats.Size = int64(b.length())
	return dropped
}

func (b *MemoryBuffer) BeginTransaction(batchSize int) *Transaction {
	b.mu.Lock()
	defer b.mu.Unlock()

	outLen := min(batchSize, b.size)
	if outLen == 0 {
		return &Transaction{}
	}

	b.batchFirst = b.first
	b.batchSize = outLen

	batch := make([]*core.Metric, outLen)
	idx := b.batchFirst
	for i := range batch {
		batch[i] = b.buf[idx]
		b.buf[idx] = nil
		idx = b.next(idx)
	}

	b.first = b.nextby(b.first, b.batchSize)
	b.size -= outLen
	return &Transaction{Batch: batch, valid: true}
}

func (b *MemoryBuffer) EndTransaction(tx *Transaction) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !tx.valid {
		return
	}
	tx.valid = false

	// Accept: metrics gone for good.
	b.stats.Written += int64(len(tx.Accept))

	// Reject: metrics intentionally dropped.
	b.stats.Rejected += int64(len(tx.Reject))

	// Keep: anything else gets put back at the head of the buffer
	// so it's the next thing the output will see on the next flush.
	keep := tx.InferKeep()
	if len(keep) > 0 {
		// We can only restore as many as currently fit (size has
		// already been decremented by BeginTransaction's batch).
		restore := len(keep)
		if free := b.cap - b.size; restore > free {
			restore = free
		}
		b.first = b.prevby(b.first, restore)
		b.size += restore

		current := b.first
		for i := 0; i < restore; i++ {
			b.buf[current] = tx.Batch[keep[i]]
			current = b.next(current)
		}
		// Metrics that couldn't fit are dropped — Counted as Dropped
		if drop := len(keep) - restore; drop > 0 {
			b.stats.Dropped += int64(drop)
		}
	}

	b.batchFirst = 0
	b.batchSize = 0
	b.stats.Size = int64(b.length())
}

func (b *MemoryBuffer) Stats() BufferStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stats
}

func (*MemoryBuffer) Close() error { return nil }

// --- internal helpers, all called with b.mu held ---

func (b *MemoryBuffer) length() int {
	return b.size
}

func (b *MemoryBuffer) add(m *core.Metric) int {
	dropped := 0
	if b.size == b.cap {
		// Overwrite the oldest. Note: b.buf[b.last] is the slot
		// about to be reused, which (when full) is also b.first.
		b.stats.Dropped++
		dropped = 1
	}

	b.buf[b.last] = m
	b.last = b.next(b.last)
	if b.size == b.cap {
		b.first = b.next(b.first)
	} else {
		b.size++
	}
	b.stats.Added++
	return dropped
}

func (b *MemoryBuffer) next(i int) int {
	i++
	if i == b.cap {
		return 0
	}
	return i
}

func (b *MemoryBuffer) nextby(i, count int) int {
	return (i + count) % b.cap
}

func (b *MemoryBuffer) prevby(i, count int) int {
	i -= count
	for i < 0 {
		i += b.cap
	}
	return i % b.cap
}
