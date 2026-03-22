package adapter

import (
	"context"
	"log"
	"sync"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

// GenericAdapter launches a custom agent CLI inside a container.
// Configured via AgentConfig — no code changes needed for new agents.
type GenericAdapter struct {
	sb          sandbox.Sandbox
	cfg         agentcfg.AgentConfig
	conn        sandbox.PTYConn
	containerID string
	mu          sync.Mutex
}

// NewGenericAdapter creates a GenericAdapter from a custom agent config.
func NewGenericAdapter(sb sandbox.Sandbox, cfg agentcfg.AgentConfig) *GenericAdapter {
	return &GenericAdapter{sb: sb, cfg: cfg}
}

// Image returns the configured image, or empty to use the default.
func (a *GenericAdapter) Image() string {
	return a.cfg.Image
}

// Start launches the custom agent command via docker exec with a PTY.
func (a *GenericAdapter) Start(ctx context.Context, containerID string) (sandbox.PTYConn, error) {
	conn, err := a.sb.Exec(ctx, containerID, a.cfg.Command)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.conn = conn
	a.containerID = containerID
	a.mu.Unlock()

	log.Printf("generic adapter %q started in container %s", a.cfg.Name, shortid.Format(containerID))
	return conn, nil
}

// Stop gracefully terminates the custom agent process.
func (a *GenericAdapter) Stop() error {
	a.mu.Lock()
	containerID := a.containerID
	a.conn = nil
	a.containerID = ""
	a.mu.Unlock()

	if containerID == "" {
		return nil
	}

	gracefulStop(a.sb, containerID, a.cfg.Command[0], stopTimeout)
	return nil
}
