package streamsh

import (
	"strings"
	"sync"
)

// SearchResult holds a matched line and its global sequence number.
type SearchResult struct {
	Seq  uint64 `json:"seq"`
	Line string `json:"line"`
}

// RingBuffer is a fixed-capacity circular buffer of lines.
// Each appended line is assigned a monotonically increasing sequence number,
// enabling cursor-based pagination even after old lines are evicted.
// All methods are safe for concurrent use.
type RingBuffer struct {
	mu       sync.RWMutex
	lines    []string
	cap      int
	head     int    // next write position
	count    int    // current number of stored lines
	totalSeq uint64 // total lines ever written
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100000
	}
	return &RingBuffer{
		lines: make([]string, capacity),
		cap:   capacity,
	}
}

// Append adds a line to the buffer and returns its global sequence number.
func (rb *RingBuffer) Append(line string) uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	seq := rb.totalSeq
	rb.lines[rb.head] = line
	rb.head = (rb.head + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}
	rb.totalSeq++
	return seq
}

// Len returns the number of lines currently stored.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// TotalSeq returns the total number of lines ever appended.
func (rb *RingBuffer) TotalSeq() uint64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.totalSeq
}

// LastN returns the most recent n lines. Returns fewer if the buffer has less.
func (rb *RingBuffer) LastN(n int) []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.count {
		n = rb.count
	}
	if n <= 0 {
		return nil
	}

	result := make([]string, n)
	// Start index: head is the next write position, so the most recent line is at head-1.
	start := (rb.head - n + rb.cap) % rb.cap
	for i := 0; i < n; i++ {
		result[i] = rb.lines[(start+i)%rb.cap]
	}
	return result
}

// ReadRange returns lines starting at global sequence `from`, up to `count` lines.
// Returns the lines, the next cursor for pagination, and whether more lines exist.
// If `from` is older than the oldest retained line, reading starts from the oldest available.
func (rb *RingBuffer) ReadRange(from uint64, count int) ([]string, uint64, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 || count <= 0 {
		return nil, from, false
	}

	oldestSeq := rb.totalSeq - uint64(rb.count)

	// Clamp to oldest available
	if from < oldestSeq {
		from = oldestSeq
	}

	// If from is beyond what we have, nothing to return
	if from >= rb.totalSeq {
		return nil, from, false
	}

	available := int(rb.totalSeq - from)
	if count > available {
		count = available
	}

	// Calculate the buffer index for `from`
	offset := int(from - oldestSeq)
	startIdx := (rb.head - rb.count + offset + rb.cap) % rb.cap

	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = rb.lines[(startIdx+i)%rb.cap]
	}

	nextCursor := from + uint64(count)
	hasMore := nextCursor < rb.totalSeq
	return result, nextCursor, hasMore
}

// Cap returns the buffer's capacity.
func (rb *RingBuffer) Cap() int {
	return rb.cap
}

// AllLines returns all lines currently in the buffer, from oldest to newest.
func (rb *RingBuffer) AllLines() []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]string, rb.count)
	start := (rb.head - rb.count + rb.cap) % rb.cap
	for i := 0; i < rb.count; i++ {
		result[i] = rb.lines[(start+i)%rb.cap]
	}
	return result
}

// Clear resets the ring buffer to an empty state.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.count = 0
	rb.totalSeq = 0
	for i := range rb.lines {
		rb.lines[i] = ""
	}
}

// Search returns lines matching a case-insensitive substring search.
// Results are ordered from oldest to newest, capped at maxResults.
func (rb *RingBuffer) Search(pattern string, maxResults int) []SearchResult {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 || maxResults <= 0 {
		return nil
	}

	lowerPattern := strings.ToLower(pattern)
	oldestSeq := rb.totalSeq - uint64(rb.count)
	startIdx := (rb.head - rb.count + rb.cap) % rb.cap

	var results []SearchResult
	for i := 0; i < rb.count && len(results) < maxResults; i++ {
		idx := (startIdx + i) % rb.cap
		if strings.Contains(strings.ToLower(rb.lines[idx]), lowerPattern) {
			results = append(results, SearchResult{
				Seq:  oldestSeq + uint64(i),
				Line: rb.lines[idx],
			})
		}
	}
	return results
}
