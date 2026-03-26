package services

import (
	"fmt"
	"sync"
	"testing"

	"github.com/AxeForging/yoink/domain"
)

func outEntry(text string) domain.LogEntry {
	return domain.LogEntry{Source: domain.SourceStdout, Text: text}
}

func inEntry(text string) domain.LogEntry {
	return domain.LogEntry{Source: domain.SourceStdin, Text: text}
}

func TestRingBuffer_WriteAndReadAll(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write(outEntry("line1"))
	rb.Write(outEntry("line2"))
	rb.Write(inEntry("input1"))

	entries := rb.ReadAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Text != "line1" || entries[0].Source != domain.SourceStdout {
		t.Errorf("unexpected entry 0: %+v", entries[0])
	}
	if entries[2].Text != "input1" || entries[2].Source != domain.SourceStdin {
		t.Errorf("unexpected entry 2: %+v", entries[2])
	}
}

func TestRingBuffer_ReadLast(t *testing.T) {
	rb := NewRingBuffer(10)
	for i := 0; i < 7; i++ {
		rb.Write(outEntry(fmt.Sprintf("line%d", i)))
	}

	entries := rb.ReadLast(3)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Text != "line4" || entries[1].Text != "line5" || entries[2].Text != "line6" {
		t.Errorf("expected [line4 line5 line6], got %v", entries)
	}
}

func TestRingBuffer_ReadLast_MoreThanCount(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write(outEntry("only"))

	entries := rb.ReadLast(5)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Text != "only" {
		t.Errorf("expected 'only', got %q", entries[0].Text)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Write(outEntry("a"))
	rb.Write(outEntry("b"))
	rb.Write(outEntry("c"))
	rb.Write(outEntry("d"))
	rb.Write(outEntry("e"))

	entries := rb.ReadAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Text != "c" || entries[1].Text != "d" || entries[2].Text != "e" {
		t.Errorf("expected [c d e], got texts: [%s %s %s]", entries[0].Text, entries[1].Text, entries[2].Text)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(5)
	entries := rb.ReadAll()
	if entries != nil {
		t.Errorf("expected nil for empty buffer, got %v", entries)
	}
	if rb.Count() != 0 {
		t.Errorf("expected count 0, got %d", rb.Count())
	}
}

func TestRingBuffer_Count(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 3; i++ {
		rb.Write(outEntry(fmt.Sprintf("%d", i)))
	}
	if rb.Count() != 3 {
		t.Errorf("expected count 3, got %d", rb.Count())
	}

	for i := 3; i < 10; i++ {
		rb.Write(outEntry(fmt.Sprintf("%d", i)))
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

func TestRingBuffer_RedactedEntry(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write(outEntry("Enter password:"))
	rb.Write(domain.LogEntry{Source: domain.SourceStdin, Text: "secret123", Redacted: true})
	rb.Write(outEntry("Authenticated"))

	entries := rb.ReadAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if !entries[1].Redacted {
		t.Error("expected entry 1 to be redacted")
	}
	if entries[1].Source != domain.SourceStdin {
		t.Errorf("expected stdin source, got %s", entries[1].Source)
	}
}

func TestRingBuffer_MixedSources(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write(outEntry("prompt>"))
	rb.Write(inEntry("my-answer"))
	rb.Write(outEntry("got: my-answer"))

	entries := rb.ReadAll()
	sources := make([]domain.LineSource, len(entries))
	for i, e := range entries {
		sources[i] = e.Source
	}
	expected := []domain.LineSource{domain.SourceStdout, domain.SourceStdin, domain.SourceStdout}
	for i, s := range sources {
		if s != expected[i] {
			t.Errorf("entry %d: expected source %s, got %s", i, expected[i], s)
		}
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				rb.Write(outEntry(fmt.Sprintf("w%d-l%d", writer, i)))
			}
		}(w)
	}

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

	if rb.Count() != 100 {
		t.Errorf("expected count 100 after overflow, got %d", rb.Count())
	}
}
