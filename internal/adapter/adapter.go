package adapter

import (
	"context"
	"fmt"

	"github.com/Rsych/zynqel-core/internal/sandbox"
)

// AgentAdapter launches and manages an agent CLI inside a container.
type AgentAdapter interface {
	// Image returns the Docker image required for this agent.
	Image() string

	// Start launches the agent process inside the container.
	// Returns a PTYConn connected to the agent's stdin/stdout.
	Start(ctx context.Context, containerID string) (sandbox.PTYConn, error)

	// Stop terminates the agent process.
	Stop() error
}

// New returns an AgentAdapter for the given agent name.
// Returns (nil, nil) for "shell" or "" — the caller should fall back to
// bare container attach when no adapter is returned.
func New(agent string, sb sandbox.Sandbox) (AgentAdapter, error) {
	switch agent {
	case "claude":
		return NewClaudeAdapter(sb), nil
	case "shell", "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", agent)
	}
}

// IsSupported returns true if the agent name is recognized.
func IsSupported(agent string) bool {
	switch agent {
	case "claude", "shell", "":
		return true
	default:
		return false
	}
}
