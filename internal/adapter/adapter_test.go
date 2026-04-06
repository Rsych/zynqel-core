package adapter

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/sandbox"
)

type testPTY struct{}

func (testPTY) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (testPTY) Write(p []byte) (int, error) { return len(p), nil }
func (testPTY) Close() error                { return nil }

type execCall struct {
	containerID string
	cmd         []string
}

type mockSandbox struct {
	mu          sync.Mutex
	execCalls   []execCall
	execRunCall []execCall
}

func (m *mockSandbox) Create(context.Context, sandbox.Spec) (string, error)    { return "", nil }
func (m *mockSandbox) Start(context.Context, string) error                     { return nil }
func (m *mockSandbox) Stop(context.Context, string) error                      { return nil }
func (m *mockSandbox) Remove(context.Context, string) error                    { return nil }
func (m *mockSandbox) Attach(context.Context, string) (sandbox.PTYConn, error) { return nil, nil }
func (m *mockSandbox) Resize(context.Context, string, int, int) error          { return nil }
func (m *mockSandbox) Stats(context.Context, string) (*sandbox.ContainerStats, error) {
	return &sandbox.ContainerStats{}, nil
}
func (m *mockSandbox) Commit(context.Context, string, string) error     { return nil }
func (m *mockSandbox) ImageExists(context.Context, string) bool         { return false }
func (m *mockSandbox) BuildImage(context.Context, string, string) error { return nil }
func (m *mockSandbox) ListVolumes(context.Context, string) ([]sandbox.VolumeInfo, error) {
	return nil, nil
}
func (m *mockSandbox) RemoveVolume(context.Context, string) error       { return nil }
func (m *mockSandbox) CopyVolume(context.Context, string, string) error { return nil }
func (m *mockSandbox) TagImage(context.Context, string, string) error   { return nil }
func (m *mockSandbox) RemoveImage(context.Context, string) error        { return nil }

func (m *mockSandbox) Exec(_ context.Context, containerID string, cmd []string) (sandbox.PTYConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls = append(m.execCalls, execCall{containerID: containerID, cmd: append([]string(nil), cmd...)})
	return testPTY{}, nil
}

func (m *mockSandbox) ExecRun(_ context.Context, containerID string, cmd []string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execRunCall = append(m.execRunCall, execCall{containerID: containerID, cmd: append([]string(nil), cmd...)})
	if len(cmd) >= 3 && cmd[0] == "sh" && cmd[1] == "-c" {
		if cmd[2] == "pgrep -f '/usr/local/bin/claude' > /dev/null 2>&1 && echo running || echo exited" {
			return []byte("exited\n"), nil
		}
		if cmd[2] == "pgrep -f 'my-agent' > /dev/null 2>&1 && echo running || echo exited" {
			return []byte("exited\n"), nil
		}
	}
	return nil, nil
}

func TestNew(t *testing.T) {
	sb := &mockSandbox{}

	store := agentcfg.NewStore(filepath.Join(t.TempDir(), "agents.json"))
	if err := store.Put(agentcfg.AgentConfig{
		Name:    "custom",
		Command: []string{"my-agent"},
		Image:   "custom:latest",
	}); err != nil {
		t.Fatalf("store put: %v", err)
	}

	tests := []struct {
		name    string
		agent   string
		wantNil bool
		wantErr bool
	}{
		{name: "claude", agent: "claude"},
		{name: "shell", agent: "shell", wantNil: true},
		{name: "empty", agent: "", wantNil: true},
		{name: "custom", agent: "custom"},
		{name: "unsupported", agent: "nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.agent, sb, store)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && got != nil {
				t.Fatalf("expected nil adapter")
			}
			if !tt.wantNil && got == nil {
				t.Fatalf("expected adapter, got nil")
			}
		})
	}
}
