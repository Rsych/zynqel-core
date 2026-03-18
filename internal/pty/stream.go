package pty

import (
	"context"
	"io"
	"log"
	"sync"

	"github.com/Rsych/zynqel-core/internal/sandbox"
)

// Stream bridges a container PTY connection with consumer callbacks.
// It reads output from the container and forwards input to it.
type Stream struct {
	conn   sandbox.PTYConn
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// New creates a Stream from a PTYConn.
// The caller must call Close when done.
func New(conn sandbox.PTYConn) *Stream {
	return &Stream{
		conn: conn,
		done: make(chan struct{}),
	}
}

// Run starts reading from the container PTY and calls onOutput for each chunk.
// It blocks until the context is cancelled, the connection closes, or an error occurs.
// onOutput is called synchronously from the read loop — do not block in it.
func (s *Stream) Run(ctx context.Context, onOutput func([]byte)) {
	ctx, s.cancel = context.WithCancel(ctx)
	defer close(s.done)

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := s.conn.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			onOutput(chunk)
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("pty read error: %v", err)
			}
			return
		}
	}
}

// Write sends input to the container's stdin.
func (s *Stream) Write(data []byte) error {
	_, err := s.conn.Write(data)
	return err
}

// Close stops the stream and releases the PTY connection.
func (s *Stream) Close() {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		_ = s.conn.Close()
		<-s.done
	})
}
