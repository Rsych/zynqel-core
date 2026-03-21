package sandbox

import (
	"context"
	"io"
)

// LabelManaged is the label key used to identify containers created by Zynqel.
// Sweep uses this to find and remove orphaned containers.
const LabelManaged = "zynqel.managed"

// Sandbox defines the contract for an execution backend.
// All methods take a context.Context — this is standard Go
// for anything that does I/O. It lets callers control
// cancellation (e.g., server shutdown, request timeout).
type Sandbox interface {
	Create(ctx context.Context, spec Spec) (string, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	Attach(ctx context.Context, id string) (PTYConn, error)
	Exec(ctx context.Context, id string, cmd []string) (PTYConn, error)
	ExecRun(ctx context.Context, id string, cmd []string) ([]byte, error)
	Resize(ctx context.Context, id string, cols, rows int) error
	Commit(ctx context.Context, containerID, imageName string) error
	ImageExists(ctx context.Context, imageName string) bool
	ListVolumes(ctx context.Context, prefix string) ([]VolumeInfo, error)
	RemoveVolume(ctx context.Context, name string) error
}

// VolumeInfo describes a Docker volume.
type VolumeInfo struct {
	Name      string
	CreatedAt string
	Image     string // image used to create this workspace
	Agent     string // agent type
}

// PTYConn is a bidirectional connection to a container's PTY.
// Read returns terminal output, Write sends keyboard input.
// Close detaches from the container.
type PTYConn interface {
	io.ReadWriteCloser
}

// Spec describes what the sandbox should look like.
// Kept separate from session.SessionSpec — the sandbox
// doesn't need to know about agents or repos, just the
// container config.
type Spec struct {
	Image        string            // Docker image to run
	Cmd          []string          // Command to run (nil = use image default)
	Env          map[string]string // Environment variables
	Labels       map[string]string // Container labels for identification
	MemoryBytes  int64             // Memory limit in bytes (0 = no limit)
	NanoCPUs     int64             // CPU limit in Docker NanoCPU units (1e9 = 1 core, 0 = no limit)
	VolumeName   string            // Docker volume to mount at /workspace (empty = no volume)
	VolumeLabels map[string]string // Labels for the volume (set on first creation)
}
