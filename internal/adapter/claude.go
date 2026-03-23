package adapter

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

const claudeImage = "zynqel-claude:latest"

// stopTimeout is how long to wait for SIGTERM before sending SIGKILL.
const stopTimeout = 5 * time.Second

// ClaudeAdapter launches the Claude CLI inside a container.
type ClaudeAdapter struct {
	sb          sandbox.Sandbox
	conn        sandbox.PTYConn
	containerID string
	mu          sync.Mutex
}

// NewClaudeAdapter creates a new ClaudeAdapter.
func NewClaudeAdapter(sb sandbox.Sandbox) *ClaudeAdapter {
	return &ClaudeAdapter{sb: sb}
}

// Image returns the Docker image with Claude Code pre-installed.
func (a *ClaudeAdapter) Image() string {
	return claudeImage
}

// Start launches the Claude CLI via docker exec with a PTY.
func (a *ClaudeAdapter) Start(ctx context.Context, containerID string) (sandbox.PTYConn, error) {
	conn, err := a.sb.Exec(ctx, containerID, []string{"/usr/local/bin/claude"})
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.conn = conn
	a.containerID = containerID
	a.mu.Unlock()

	log.Printf("claude adapter started in container %s", shortid.Format(containerID))
	return conn, nil
}

// Stop gracefully terminates the Claude process.
// Sends SIGTERM first, waits up to 5 seconds, then SIGKILL.
func (a *ClaudeAdapter) Stop() error {
	a.mu.Lock()
	containerID := a.containerID
	a.conn = nil
	a.containerID = ""
	a.mu.Unlock()

	if containerID == "" {
		return nil
	}

	gracefulStop(a.sb, containerID, "/usr/local/bin/claude", stopTimeout)
	return nil
}
