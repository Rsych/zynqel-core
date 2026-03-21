package pty

import (
	"bytes"
	"sync"
	"testing"
)

func TestRingBuffer_WriteLessThanCapacity(t *testing.T) {
	r := NewRingBuffer(10)
	_, _ = r.Write([]byte("hello"))
	got := r.Bytes()
	if !bytes.Equal(got, []byte("hello")) {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestRingBuffer_WriteExactCapacity(t *testing.T) {
	r := NewRingBuffer(5)
	_, _ = r.Write([]byte("abcde"))
	got := r.Bytes()
	if !bytes.Equal(got, []byte("abcde")) {
		t.Errorf("got %q, want %q", got, "abcde")
	}
}

func TestRingBuffer_WriteWraps(t *testing.T) {
	r := NewRingBuffer(5)
	_, _ = r.Write([]byte("abc"))  // buf: [a b c _ _], w=3
	_, _ = r.Write([]byte("defg")) // wraps: buf: [f g c d e], w=2
	got := r.Bytes()
	if !bytes.Equal(got, []byte("cdefg")) {
		t.Errorf("got %q, want %q", got, "cdefg")
	}
}

func TestRingBuffer_WriteLargerThanCapacity(t *testing.T) {
	r := NewRingBuffer(5)
	_, _ = r.Write([]byte("abcdefghij"))
	got := r.Bytes()
	if !bytes.Equal(got, []byte("fghij")) {
		t.Errorf("got %q, want %q", got, "fghij")
	}
}

func TestRingBuffer_MultipleSmallWrites(t *testing.T) {
	r := NewRingBuffer(5)
	_, _ = r.Write([]byte("ab"))
	_, _ = r.Write([]byte("cd"))
	_, _ = r.Write([]byte("ef"))
	_, _ = r.Write([]byte("gh"))
	got := r.Bytes()
	// Total written: abcdefgh (8 bytes), buffer keeps last 5: defgh
	if !bytes.Equal(got, []byte("defgh")) {
		t.Errorf("got %q, want %q", got, "defgh")
	}
}

func TestRingBuffer_EmptyWrite(t *testing.T) {
	r := NewRingBuffer(5)
	_, _ = r.Write([]byte{})
	got := r.Bytes()
	if len(got) != 0 {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestRingBuffer_EmptyRead(t *testing.T) {
	r := NewRingBuffer(5)
	got := r.Bytes()
	if len(got) != 0 {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestRingBuffer_DefaultSize(t *testing.T) {
	r := NewRingBuffer(0)
	if r.size != defaultBufSize {
		t.Errorf("got size %d, want %d", r.size, defaultBufSize)
	}
}

func TestRingBuffer_ConcurrentReadWrite(t *testing.T) {
	r := NewRingBuffer(1024)
	var wg sync.WaitGroup

	// Writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, _ = r.Write([]byte("hello world "))
		}
	}()

	// Reader.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = r.Bytes()
		}
	}()

	wg.Wait()
	// No race or panic = pass.
	got := r.Bytes()
	if len(got) != 1024 {
		t.Errorf("expected full buffer (1024 bytes), got %d", len(got))
	}
}
