package services

import "sync"

// RingBuffer is a thread-safe circular buffer for storing output lines.
type RingBuffer struct {
	lines []string
	size  int
	head  int
	count int
	mu    sync.RWMutex
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1000
	}
	return &RingBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

// Write appends a line to the buffer, overwriting the oldest if full.
func (rb *RingBuffer) Write(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.lines[rb.head] = line
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// ReadLast returns the last n lines in chronological order.
func (rb *RingBuffer) ReadLast(n int) []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if n > rb.count {
		n = rb.count
	}
	if n == 0 {
		return nil
	}
	result := make([]string, n)
	start := (rb.head - n + rb.size) % rb.size
	for i := 0; i < n; i++ {
		result[i] = rb.lines[(start+i)%rb.size]
	}
	return result
}

// ReadAll returns all lines in the buffer in chronological order.
func (rb *RingBuffer) ReadAll() []string {
	rb.mu.RLock()
	count := rb.count
	rb.mu.RUnlock()
	return rb.ReadLast(count)
}

// Count returns the number of lines currently in the buffer.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}
