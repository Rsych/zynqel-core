package adapter

import (
	"context"
	"testing"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
)

func TestGenericAdapter_Image(t *testing.T) {
	t.Parallel()

	a := NewGenericAdapter(&mockSandbox{}, agentcfg.AgentConfig{
		Name:    "custom",
		Command: []string{"my-agent"},
		Image:   "custom:latest",
	})
	if got := a.Image(); got != "custom:latest" {
		t.Fatalf("image = %q, want %q", got, "custom:latest")
	}
}

func TestGenericAdapter_StartCallsExec(t *testing.T) {
	t.Parallel()

	sb := &mockSandbox{}
	a := NewGenericAdapter(sb, agentcfg.AgentConfig{
		Name:    "custom",
		Command: []string{"my-agent", "--json"},
	})

	if _, err := a.Start(context.Background(), "container-3"); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if len(sb.execCalls) != 1 {
		t.Fatalf("exec calls = %d, want 1", len(sb.execCalls))
	}
	got := sb.execCalls[0]
	if got.containerID != "container-3" {
		t.Fatalf("container = %q, want %q", got.containerID, "container-3")
	}
	if len(got.cmd) != 2 || got.cmd[0] != "my-agent" || got.cmd[1] != "--json" {
		t.Fatalf("cmd = %v, want [my-agent --json]", got.cmd)
	}
}

func TestGenericAdapter_StopLifecycle(t *testing.T) {
	t.Parallel()

	sb := &mockSandbox{}
	a := NewGenericAdapter(sb, agentcfg.AgentConfig{
		Name:    "custom",
		Command: []string{"my-agent"},
	})

	// no-op when not started
	if err := a.Stop(); err != nil {
		t.Fatalf("stop before start failed: %v", err)
	}

	if _, err := a.Start(context.Background(), "container-4"); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := a.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if len(sb.execRunCall) == 0 {
		t.Fatalf("expected execRun calls during stop")
	}
}
