package pty

import "sync"

const defaultBufSize = 65536 // 64KB

// RingBuffer is a fixed-size circular byte buffer.
// Thread-safe via mutex. Writes overwrite oldest data when full.
type RingBuffer struct {
	mu   sync.Mutex
	buf  []byte
	size int
	w    int  // next write position
	full bool // true once buffer has wrapped
}

// NewRingBuffer creates a ring buffer with the given capacity in bytes.
// If size <= 0, defaults to 64KB.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = defaultBufSize
	}
	return &RingBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write appends data to the buffer, overwriting the oldest bytes if full.
func (r *RingBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(p)
	if n == 0 {
		return 0, nil
	}

	// If input is larger than buffer, only keep the last 'size' bytes.
	if n >= r.size {
		copy(r.buf, p[n-r.size:])
		r.w = 0
		r.full = true
		return n, nil
	}

	// Check if this write will cause wrapping.
	if r.w+n > r.size {
		// Split write across the boundary.
		first := r.size - r.w
		copy(r.buf[r.w:], p[:first])
		copy(r.buf, p[first:])
		r.full = true
	} else {
		copy(r.buf[r.w:], p)
		if r.w+n == r.size {
			r.full = true
		}
	}

	r.w = (r.w + n) % r.size
	return n, nil
}

// Bytes returns the buffered contents in chronological order.
// Returns a copy — the caller owns the returned slice.
func (r *RingBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.full {
		out := make([]byte, r.w)
		copy(out, r.buf[:r.w])
		return out
	}

	out := make([]byte, r.size)
	n := copy(out, r.buf[r.w:])
	copy(out[n:], r.buf[:r.w])
	return out
}
