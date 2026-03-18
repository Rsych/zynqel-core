package adapter

import (
	"context"
	"log"
	"sync"

	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

// ClaudeAdapter launches the Claude CLI inside a container.
type ClaudeAdapter struct {
	sb   sandbox.Sandbox
	conn sandbox.PTYConn
	mu   sync.Mutex
}

// NewClaudeAdapter creates a new ClaudeAdapter.
func NewClaudeAdapter(sb sandbox.Sandbox) *ClaudeAdapter {
	return &ClaudeAdapter{sb: sb}
}

// Start launches the Claude CLI via docker exec with a PTY.
func (a *ClaudeAdapter) Start(ctx context.Context, containerID string) (sandbox.PTYConn, error) {
	conn, err := a.sb.Exec(ctx, containerID, []string{"claude"})
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()

	log.Printf("claude adapter started in container %s", shortid.Format(containerID))
	return conn, nil
}

// Stop closes the PTY connection to the Claude process.
func (a *ClaudeAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn != nil {
		err := a.conn.Close()
		a.conn = nil
		return err
	}
	return nil
}
