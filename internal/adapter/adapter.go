package adapter

import (
	"context"
	"fmt"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
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
// Built-in agents (claude) have dedicated adapters. Custom agents
// registered in the store use GenericAdapter.
// Returns (nil, nil) for "shell" or "" — the caller should fall back to
// bare container attach when no adapter is returned.
func New(agent string, sb sandbox.Sandbox, store *agentcfg.Store) (AgentAdapter, error) {
	switch agent {
	case "claude":
		return NewClaudeAdapter(sb), nil
	case "opencode":
		return NewGenericAdapter(sb, agentcfg.AgentConfig{
			Name:    "opencode",
			Command: []string{"opencode"},
			Image:   "zynqel-opencode:latest",
		}), nil
	case "codex":
		return NewGenericAdapter(sb, agentcfg.AgentConfig{
			Name:    "codex",
			Command: []string{"codex"},
			Image:   "zynqel-codex:latest",
		}), nil
	case "shell", "":
		return nil, nil
	default:
		if store != nil {
			if cfg, ok := store.Get(agent); ok {
				return NewGenericAdapter(sb, cfg), nil
			}
		}
		return nil, fmt.Errorf("unsupported agent: %s", agent)
	}
}
