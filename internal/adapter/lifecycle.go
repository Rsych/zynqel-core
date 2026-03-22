package adapter

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

// gracefulStop sends SIGTERM to a process matching pattern, waits up to
// timeout for it to exit, then sends SIGKILL if still running.
func gracefulStop(sb sandbox.Sandbox, containerID, processPattern string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+5*time.Second)
	defer cancel()

	// SIGTERM
	killCmd := fmt.Sprintf("pkill -TERM -f '%s' || true", processPattern)
	_, _ = sb.ExecRun(ctx, containerID, []string{"sh", "-c", killCmd})

	// Wait for exit
	if waitForExit(sb, ctx, containerID, processPattern, timeout) {
		return
	}

	// SIGKILL
	log.Printf("process %q did not exit after SIGTERM, sending SIGKILL in container %s", processPattern, shortid.Format(containerID))
	forceCmd := fmt.Sprintf("pkill -KILL -f '%s' || true", processPattern)
	_, _ = sb.ExecRun(ctx, containerID, []string{"sh", "-c", forceCmd})
}

// waitForExit polls for a process to exit within the timeout.
func waitForExit(sb sandbox.Sandbox, ctx context.Context, containerID, processPattern string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	checkCmd := fmt.Sprintf("pgrep -f '%s' > /dev/null 2>&1 && echo running || echo exited", processPattern)

	for {
		select {
		case <-deadline:
			return false
		case <-ctx.Done():
			return false
		case <-tick.C:
			output, err := sb.ExecRun(ctx, containerID, []string{"sh", "-c", checkCmd})
			if err != nil {
				log.Printf("warning: failed to check process %q: %v", processPattern, err)
				return false
			}
			if strings.TrimSpace(string(output)) == "exited" {
				return true
			}
		}
	}
}
