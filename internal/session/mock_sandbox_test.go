package session

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/Rsych/zynqel-core/internal/sandbox"
)

// mockSandbox implements sandbox.Sandbox for fast, deterministic unit tests.
type mockSandbox struct {
	nextID atomic.Int64
}

func newMockSandbox() *mockSandbox {
	return &mockSandbox{}
}

func (m *mockSandbox) Create(_ context.Context, _ sandbox.Spec) (string, error) {
	return fmt.Sprintf("mock-%d", m.nextID.Add(1)), nil
}

func (m *mockSandbox) Start(_ context.Context, _ string) error { return nil }

func (m *mockSandbox) Stop(_ context.Context, _ string) error { return nil }

func (m *mockSandbox) Remove(_ context.Context, _ string) error { return nil }

func (m *mockSandbox) Attach(_ context.Context, _ string) (sandbox.PTYConn, error) {
	r, w := io.Pipe()
	return &pipePTYConn{r: r, w: w}, nil
}

func (m *mockSandbox) Exec(_ context.Context, _ string, _ []string) (sandbox.PTYConn, error) {
	r, w := io.Pipe()
	return &pipePTYConn{r: r, w: w}, nil
}

func (m *mockSandbox) ExecRun(_ context.Context, _ string, _ []string) ([]byte, error) {
	return nil, nil
}

func (m *mockSandbox) Resize(_ context.Context, _ string, _, _ int) error { return nil }

func (m *mockSandbox) Stats(_ context.Context, _ string) (*sandbox.ContainerStats, error) {
	return &sandbox.ContainerStats{}, nil
}

func (m *mockSandbox) Commit(_ context.Context, _, _ string) error { return nil }

func (m *mockSandbox) ImageExists(_ context.Context, _ string) bool { return false }

func (m *mockSandbox) BuildImage(_ context.Context, _, _ string) error { return nil }

func (m *mockSandbox) ListVolumes(_ context.Context, _ string) ([]sandbox.VolumeInfo, error) {
	return nil, nil
}

func (m *mockSandbox) RemoveVolume(_ context.Context, _ string) error { return nil }

// pipePTYConn adapts io.Pipe into a sandbox.PTYConn.
// Closing the reader unblocks the broadcaster's readLoop.
type pipePTYConn struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (c *pipePTYConn) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *pipePTYConn) Write(p []byte) (int, error) { return c.w.Write(p) }

func (c *pipePTYConn) Close() error {
	_ = c.r.Close()
	_ = c.w.Close()
	return nil
}
