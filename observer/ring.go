// Package observer implements process tailers, file watchers, and
// other observation sources that feed data into Claude Code's context.
package observer

import "sync"

// Ring is a thread-safe fixed-size circular buffer of strings.
// When full, new entries overwrite the oldest.
type Ring struct {
	mu    sync.Mutex
	buf   []string
	size  int
	head  int // next write position
	count int // number of items currently stored
}

// NewRing creates a ring buffer that holds up to `size` strings.
func NewRing(size int) *Ring {
	return &Ring{
		buf:  make([]string, size),
		size: size,
	}
}

// Write appends a line to the ring buffer.
func (r *Ring) Write(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = line
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// ReadAll returns all lines in chronological order.
func (r *Ring) ReadAll() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readLocked(r.count)
}

// ReadN returns the last n lines in chronological order.
func (r *Ring) ReadN(n int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n > r.count {
		n = r.count
	}
	return r.readLocked(n)
}

// Drain returns all lines and clears the buffer.
func (r *Ring) Drain() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := r.readLocked(r.count)
	r.head = 0
	r.count = 0
	return lines
}

// Len returns the current number of items in the buffer.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *Ring) readLocked(n int) []string {
	if n == 0 {
		return nil
	}
	result := make([]string, n)
	// Start reading from the oldest of the n entries.
	start := (r.head - n + r.size) % r.size
	for i := 0; i < n; i++ {
		result[i] = r.buf[(start+i)%r.size]
	}
	return result
}
