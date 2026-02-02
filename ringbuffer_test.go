package streamsh

import (
	"fmt"
	"testing"
)

func TestRingBufferAppendAndLen(t *testing.T) {
	rb := NewRingBuffer(5)
	if rb.Len() != 0 {
		t.Fatalf("expected len 0, got %d", rb.Len())
	}

	for i := range 3 {
		seq := rb.Append(fmt.Sprintf("line %d", i))
		if seq != uint64(i) {
			t.Fatalf("expected seq %d, got %d", i, seq)
		}
	}
	if rb.Len() != 3 {
		t.Fatalf("expected len 3, got %d", rb.Len())
	}
}

func TestRingBufferEviction(t *testing.T) {
	rb := NewRingBuffer(3)
	for i := range 5 {
		rb.Append(fmt.Sprintf("line %d", i))
	}
	if rb.Len() != 3 {
		t.Fatalf("expected len 3, got %d", rb.Len())
	}
	if rb.TotalSeq() != 5 {
		t.Fatalf("expected totalSeq 5, got %d", rb.TotalSeq())
	}
	lines := rb.LastN(10)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for i, want := range []string{"line 2", "line 3", "line 4"} {
		if lines[i] != want {
			t.Errorf("LastN[%d] = %q, want %q", i, lines[i], want)
		}
	}
}

func TestRingBufferLastN(t *testing.T) {
	rb := NewRingBuffer(10)
	for i := range 7 {
		rb.Append(fmt.Sprintf("line %d", i))
	}

	lines := rb.LastN(3)
	if len(lines) != 3 {
		t.Fatalf("expected 3, got %d", len(lines))
	}
	for i, want := range []string{"line 4", "line 5", "line 6"} {
		if lines[i] != want {
			t.Errorf("LastN[%d] = %q, want %q", i, lines[i], want)
		}
	}

	// Request more than available
	lines = rb.LastN(100)
	if len(lines) != 7 {
		t.Fatalf("expected 7, got %d", len(lines))
	}
}

func TestRingBufferReadRange(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := range 8 {
		rb.Append(fmt.Sprintf("line %d", i))
	}
	// Buffer has lines 3,4,5,6,7 (seqs 3-7), totalSeq=8

	// Read from seq 5, count 2
	lines, next, hasMore := rb.ReadRange(5, 2)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line 5" || lines[1] != "line 6" {
		t.Errorf("got %v", lines)
	}
	if next != 7 {
		t.Errorf("expected next=7, got %d", next)
	}
	if !hasMore {
		t.Error("expected hasMore=true")
	}

	// Read from seq 0 (older than oldest) -> clamps to 3
	lines, next, _ = rb.ReadRange(0, 2)
	if lines[0] != "line 3" || lines[1] != "line 4" {
		t.Errorf("clamped read got %v", lines)
	}
	if next != 5 {
		t.Errorf("expected next=5, got %d", next)
	}

	// Read from seq 8 (beyond end)
	lines, _, hasMore = rb.ReadRange(8, 10)
	if len(lines) != 0 {
		t.Errorf("expected empty, got %v", lines)
	}
	if hasMore {
		t.Error("expected hasMore=false")
	}
}

func TestRingBufferSearch(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Append("hello world")
	rb.Append("foo bar")
	rb.Append("Hello Again")
	rb.Append("baz qux")
	rb.Append("HELLO FINAL")

	results := rb.Search("hello", 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Line != "hello world" {
		t.Errorf("results[0] = %q", results[0].Line)
	}
	if results[2].Line != "HELLO FINAL" {
		t.Errorf("results[2] = %q", results[2].Line)
	}

	// Max results cap
	results = rb.Search("hello", 1)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestRingBufferDefaultCapacity(t *testing.T) {
	rb := NewRingBuffer(0)
	if rb.cap != 10000 {
		t.Errorf("expected default cap 10000, got %d", rb.cap)
	}
}
