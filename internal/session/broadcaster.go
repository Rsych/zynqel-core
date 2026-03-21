package session

import (
	"io"
	"log"
	"sync"

	"github.com/Rsych/zynqel-core/internal/intercept"
	"github.com/Rsych/zynqel-core/internal/pty"
	"github.com/Rsych/zynqel-core/internal/sandbox"
)

const (
	// DefaultBufferSize is the default ring buffer size for PTY output.
	DefaultBufferSize = 65536 // 64KB

	// subscriberChanSize is the channel buffer for each subscriber.
	// Non-blocking sends drop data if the subscriber can't keep up.
	subscriberChanSize = 64
)

// Subscriber receives live PTY output via Ch and detected prompts via Events.
// Both channels are closed when the PTY stream ends or the subscriber is removed.
type Subscriber struct {
	Ch     chan []byte
	Events chan intercept.Prompt
}

// Broadcaster reads from a single PTYConn, buffers output in a ring buffer,
// and fans out to multiple subscribers. Safe for concurrent use.
type Broadcaster struct {
	conn        sandbox.PTYConn
	ring        *pty.RingBuffer
	intercepter *intercept.Intercepter
	mu          sync.Mutex
	subs        map[*Subscriber]struct{}
	stopped     chan struct{}
	once        sync.Once
}

// NewBroadcaster creates a Broadcaster and starts reading from conn.
// bufSize is the ring buffer size in bytes (0 = default 64KB).
func NewBroadcaster(conn sandbox.PTYConn, bufSize int) *Broadcaster {
	b := &Broadcaster{
		conn:        conn,
		ring:        pty.NewRingBuffer(bufSize),
		intercepter: intercept.New(),
		subs:        make(map[*Subscriber]struct{}),
		stopped:     make(chan struct{}),
	}
	go b.readLoop()
	return b
}

// Subscribe returns buffered output for replay and a Subscriber for live output.
// The caller must call Unsubscribe when done.
func (b *Broadcaster) Subscribe() (replay []byte, sub *Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	replay = b.ring.Bytes()
	sub = &Subscriber{
		Ch:     make(chan []byte, subscriberChanSize),
		Events: make(chan intercept.Prompt, 16),
	}
	b.subs[sub] = struct{}{}
	return replay, sub
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Broadcaster) Unsubscribe(sub *Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subs[sub]; ok {
		delete(b.subs, sub)
		close(sub.Ch)
		close(sub.Events)
	}
}

// Write sends input to the underlying PTYConn.
func (b *Broadcaster) Write(p []byte) error {
	_, err := b.conn.Write(p)
	return err
}

// Stopped returns a channel that closes when the PTY read loop ends.
func (b *Broadcaster) Stopped() <-chan struct{} {
	return b.stopped
}

// Close stops the broadcaster and closes the PTYConn.
func (b *Broadcaster) Close() {
	b.once.Do(func() {
		_ = b.conn.Close()
		<-b.stopped // wait for read loop to finish
	})
}

// readLoop reads from the PTYConn, writes to the ring buffer,
// and fans out to all subscribers.
func (b *Broadcaster) readLoop() {
	defer close(b.stopped)
	defer b.closeAllSubscribers()

	buf := make([]byte, 4096)
	for {
		n, err := b.conn.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			// Scan for prompts before fan-out.
			prompts := b.intercepter.Scan(chunk)

			b.mu.Lock()
			_, _ = b.ring.Write(chunk)
			for sub := range b.subs {
				select {
				case sub.Ch <- chunk:
				default:
				}
				// Send detected prompts.
				for _, p := range prompts {
					select {
					case sub.Events <- p:
					default:
					}
				}
			}
			b.mu.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("broadcaster read error: %v", err)
			}
			return
		}
	}
}

// closeAllSubscribers closes all subscriber channels.
func (b *Broadcaster) closeAllSubscribers() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sub := range b.subs {
		close(sub.Ch)
		close(sub.Events)
		delete(b.subs, sub)
	}
}
