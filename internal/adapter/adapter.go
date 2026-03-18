package adapter

import (
	"context"
	"fmt"

	"github.com/Rsych/zynqel-core/internal/sandbox"
)

// AgentAdapter launches and manages an agent CLI inside a container.
type AgentAdapter interface {
	// Start launches the agent process inside the container.
	// Returns a PTYConn connected to the agent's stdin/stdout.
	Start(ctx context.Context, containerID string) (sandbox.PTYConn, error)

	// Stop terminates the agent process.
	Stop() error
}

// New returns an AgentAdapter for the given agent name.
// Returns an error if the agent is not supported.
func New(agent string, sb sandbox.Sandbox) (AgentAdapter, error) {
	switch agent {
	case "claude":
		return NewClaudeAdapter(sb), nil
	case "shell", "":
		return nil, nil // No adapter — use bare shell
	default:
		return nil, fmt.Errorf("unsupported agent: %s", agent)
	}
}
