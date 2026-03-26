package services

import (
	"sync"

	"github.com/AxeForging/yoink/domain"
)

// RingBuffer is a thread-safe circular buffer for storing log entries.
type RingBuffer struct {
	entries []domain.LogEntry
	size    int
	head    int
	count   int
	mu      sync.RWMutex
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1000
	}
	return &RingBuffer{
		entries: make([]domain.LogEntry, size),
		size:    size,
	}
}

// Write appends a log entry to the buffer, overwriting the oldest if full.
func (rb *RingBuffer) Write(entry domain.LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// ReadLast returns the last n entries in chronological order.
func (rb *RingBuffer) ReadLast(n int) []domain.LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if n > rb.count {
		n = rb.count
	}
	if n == 0 {
		return nil
	}
	result := make([]domain.LogEntry, n)
	start := (rb.head - n + rb.size) % rb.size
	for i := 0; i < n; i++ {
		result[i] = rb.entries[(start+i)%rb.size]
	}
	return result
}

// ReadAll returns all entries in the buffer in chronological order.
func (rb *RingBuffer) ReadAll() []domain.LogEntry {
	rb.mu.RLock()
	count := rb.count
	rb.mu.RUnlock()
	return rb.ReadLast(count)
}

// Count returns the number of entries currently in the buffer.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}
