package adapter

import (
	"context"
	"testing"
)

func TestClaudeAdapter_Image(t *testing.T) {
	t.Parallel()

	a := NewClaudeAdapter(&mockSandbox{})
	if got := a.Image(); got != "zynqel-claude:latest" {
		t.Fatalf("image = %q, want %q", got, "zynqel-claude:latest")
	}
}

func TestClaudeAdapter_StartCallsExec(t *testing.T) {
	t.Parallel()

	sb := &mockSandbox{}
	a := NewClaudeAdapter(sb)

	if _, err := a.Start(context.Background(), "container-1"); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	if len(sb.execCalls) != 1 {
		t.Fatalf("exec calls = %d, want 1", len(sb.execCalls))
	}
	got := sb.execCalls[0]
	if got.containerID != "container-1" {
		t.Fatalf("container = %q, want %q", got.containerID, "container-1")
	}
	if len(got.cmd) != 1 || got.cmd[0] != "/usr/local/bin/claude" {
		t.Fatalf("cmd = %v, want [/usr/local/bin/claude]", got.cmd)
	}
}

func TestClaudeAdapter_StopLifecycle(t *testing.T) {
	t.Parallel()

	sb := &mockSandbox{}
	a := NewClaudeAdapter(sb)

	// no-op when not started
	if err := a.Stop(); err != nil {
		t.Fatalf("stop before start failed: %v", err)
	}

	if _, err := a.Start(context.Background(), "container-2"); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := a.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if len(sb.execRunCall) == 0 {
		t.Fatalf("expected execRun calls during stop")
	}
}
