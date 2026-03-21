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
	conn := a.conn
	containerID := a.containerID
	a.conn = nil
	a.mu.Unlock()

	if containerID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout+5*time.Second)
	defer cancel()

	// Send SIGTERM to all claude/node processes.
	_, _ = a.sb.ExecRun(ctx, containerID, []string{"sh", "-c", "pkill -TERM -f claude || true"})

	// Wait for process to exit gracefully.
	exited := a.waitForExit(ctx, containerID, stopTimeout)

	if !exited {
		// Force kill if still running.
		log.Printf("claude process did not exit after SIGTERM, sending SIGKILL in container %s", shortid.Format(containerID))
		_, _ = a.sb.ExecRun(ctx, containerID, []string{"sh", "-c", "pkill -KILL -f claude || true"})
	}

	// Close PTY connection.
	if conn != nil {
		_ = conn.Close()
	}

	return nil
}

// waitForExit checks if the claude process has exited within the timeout.
func (a *ClaudeAdapter) waitForExit(ctx context.Context, containerID string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			return false
		case <-ctx.Done():
			return false
		case <-tick.C:
			// Check if any claude process is still running.
			_, err := a.sb.ExecRun(ctx, containerID, []string{"sh", "-c", "pgrep -f claude > /dev/null 2>&1"})
			if err != nil {
				// pgrep returns non-zero when no process found — process has exited.
				return true
			}
		}
	}
}
