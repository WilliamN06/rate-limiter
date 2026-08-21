package memory

import (
	"sync"
	"time"
)

// RingBuffer stores timestamps in a fixed-size circular buffer
type RingBuffer struct {
	mu         sync.RWMutex
	timestamps []time.Time
	head       int
	size       int
	capacity   int
}

// NewRingBuffer creates a new ring buffer with the given capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		timestamps: make([]time.Time, capacity),
		head:       0,
		size:       0,
		capacity:   capacity,
	}
}

// Add adds a timestamp to the buffer
func (rb *RingBuffer) Add(ts time.Time) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.timestamps[rb.head] = ts
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetValid returns all timestamps after the given time
// The returned slice is sorted from oldest to newest
func (rb *RingBuffer) GetValid(after time.Time) []time.Time {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return []time.Time{}
	}

	// Collect valid timestamps
	var valid []time.Time
	for i := 0; i < rb.size; i++ {
		// Read in chronological order (oldest first)
		idx := (rb.head - i - 1 + rb.capacity) % rb.capacity
		ts := rb.timestamps[idx]
		if ts.After(after) {
			valid = append(valid, ts)
		}
	}

	return valid
}

// Prune removes all timestamps before the given time
func (rb *RingBuffer) Prune(before time.Time) int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return 0
	}

	// Find how many timestamps to keep
	var keep []time.Time
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - i - 1 + rb.capacity) % rb.capacity
		ts := rb.timestamps[idx]
		if ts.After(before) {
			keep = append(keep, ts)
		}
	}

	removed := rb.size - len(keep)
	
	// Reset buffer with kept timestamps
	rb.timestamps = make([]time.Time, rb.capacity)
	rb.head = 0
	rb.size = len(keep)
	
	// Copy kept timestamps back
	for i, ts := range keep {
		rb.timestamps[i] = ts
		rb.head = i + 1
	}
	if rb.head == rb.capacity {
		rb.head = 0
	}
	
	return removed
}

// Size returns the current number of timestamps in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// Clear removes all timestamps
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.timestamps = make([]time.Time, rb.capacity)
	rb.head = 0
	rb.size = 0
}

// IsEmpty returns true if the buffer has no timestamps
func (rb *RingBuffer) IsEmpty() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == 0
}