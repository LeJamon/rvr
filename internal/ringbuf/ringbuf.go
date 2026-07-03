// Package ringbuf is a fixed-capacity byte ring that keeps the most recent
// bytes written. The supervisor uses it as the scrollback replayed to a
// client on attach (SPEC.md §4).
package ringbuf

import "sync"

// Ring keeps the last Cap bytes written to it. It is safe for concurrent use.
type Ring struct {
	mu     sync.Mutex
	data   []byte
	start  int // index of the oldest byte
	length int // number of valid bytes (<= len(data))
}

// New returns a ring that retains up to size bytes.
func New(size int) *Ring {
	if size < 1 {
		size = 1
	}
	return &Ring{data: make([]byte, size)}
}

// Write appends p, discarding the oldest bytes once capacity is exceeded. It
// never returns an error and always reports len(p) written.
func (r *Ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	total := len(p)
	c := len(r.data)
	if total >= c {
		// Only the last c bytes can survive.
		copy(r.data, p[total-c:])
		r.start, r.length = 0, c
		return total, nil
	}

	end := (r.start + r.length) % c
	n1 := copy(r.data[end:], p)
	if n1 < total {
		copy(r.data, p[n1:])
	}
	r.length += total
	if r.length > c {
		over := r.length - c
		r.start = (r.start + over) % c
		r.length = c
	}
	return total, nil
}

// Snapshot returns a copy of the current contents, oldest byte first.
func (r *Ring) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]byte, r.length)
	c := len(r.data)
	n := copy(out, r.data[r.start:min(r.start+r.length, c)])
	if n < r.length {
		copy(out[n:], r.data[:r.length-n])
	}
	return out
}

// Len reports how many bytes are currently retained.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.length
}
