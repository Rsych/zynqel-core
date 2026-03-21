package adapter

import (
	"context"
	"log"
	"strings"
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
	a.containerID = ""
	a.mu.Unlock()

	if containerID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout+5*time.Second)
	defer cancel()

	// Send SIGTERM to the claude process and its children.
	_, _ = a.sb.ExecRun(ctx, containerID, []string{"sh", "-c", "pkill -TERM -f '/usr/local/bin/claude' || true"})

	// Wait for process to exit gracefully.
	exited := a.waitForExit(ctx, containerID, stopTimeout)

	if !exited {
		// Force kill if still running.
		log.Printf("claude process did not exit after SIGTERM, sending SIGKILL in container %s", shortid.Format(containerID))
		_, _ = a.sb.ExecRun(ctx, containerID, []string{"sh", "-c", "pkill -KILL -f '/usr/local/bin/claude' || true"})
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
			// pgrep exits 0 if process found, 1 if not found.
			// Use a two-command chain: pgrep succeeds → echo "running",
			// pgrep fails → echo "exited". This way ExecRun only errors
			// on real failures (container gone, network), not on process exit.
			output, err := a.sb.ExecRun(ctx, containerID,
				[]string{"sh", "-c", "pgrep -f '/usr/local/bin/claude' > /dev/null 2>&1 && echo running || echo exited"})
			if err != nil {
				// Real error (container gone, etc.) — treat as not exited,
				// let the caller escalate to SIGKILL.
				log.Printf("warning: failed to check claude process: %v", err)
				return false
			}
			if strings.TrimSpace(string(output)) == "exited" {
				return true
			}
		}
	}
}
