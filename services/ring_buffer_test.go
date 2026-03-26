package services

import (
	"fmt"
	"sync"
	"testing"
)

func TestRingBuffer_WriteAndReadAll(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write("line1")
	rb.Write("line2")
	rb.Write("line3")

	lines := rb.ReadAll()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestRingBuffer_ReadLast(t *testing.T) {
	rb := NewRingBuffer(10)
	for i := 0; i < 7; i++ {
		rb.Write(fmt.Sprintf("line%d", i))
	}

	lines := rb.ReadLast(3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line4" || lines[1] != "line5" || lines[2] != "line6" {
		t.Errorf("expected [line4 line5 line6], got %v", lines)
	}
}

func TestRingBuffer_ReadLast_MoreThanCount(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write("only")

	lines := rb.ReadLast(5)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "only" {
		t.Errorf("expected 'only', got %q", lines[0])
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Write("a")
	rb.Write("b")
	rb.Write("c")
	rb.Write("d") // overwrites "a"
	rb.Write("e") // overwrites "b"

	lines := rb.ReadAll()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "c" || lines[1] != "d" || lines[2] != "e" {
		t.Errorf("expected [c d e], got %v", lines)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(5)
	lines := rb.ReadAll()
	if lines != nil {
		t.Errorf("expected nil for empty buffer, got %v", lines)
	}
	if rb.Count() != 0 {
		t.Errorf("expected count 0, got %d", rb.Count())
	}
}

func TestRingBuffer_Count(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 3; i++ {
		rb.Write(fmt.Sprintf("%d", i))
	}
	if rb.Count() != 3 {
		t.Errorf("expected count 3, got %d", rb.Count())
	}

	// Overflow
	for i := 3; i < 10; i++ {
		rb.Write(fmt.Sprintf("%d", i))
	}
	if rb.Count() != 5 {
		t.Errorf("expected count 5 (capped at size), got %d", rb.Count())
	}
}

func TestRingBuffer_DefaultSize(t *testing.T) {
	rb := NewRingBuffer(0)
	if rb.size != 1000 {
		t.Errorf("expected default size 1000, got %d", rb.size)
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	// 10 writers, each writing 100 lines
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				rb.Write(fmt.Sprintf("w%d-l%d", writer, i))
			}
		}(w)
	}

	// Concurrent readers
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				rb.ReadAll()
				rb.ReadLast(10)
				rb.Count()
			}
		}()
	}

	wg.Wait()

	// After all writes, buffer should be at capacity
	if rb.Count() != 100 {
		t.Errorf("expected count 100 after overflow, got %d", rb.Count())
	}
}
