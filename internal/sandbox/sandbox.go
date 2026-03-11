package sandbox

import "context"

// Sandbox defines the contract for an execution backend.
// All methods take a context.Context — this is standard Go
// for anything that does I/O. It lets callers control
// cancellation (e.g., server shutdown, request timeout).
type Sandbox interface {
	Create(ctx context.Context, spec Spec) (string, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
}

// Spec describes what the sandbox should look like.
// Kept separate from session.SessionSpec — the sandbox
// doesn't need to know about agents or repos, just the
// container config.
type Spec struct {
	Image  string            // Docker image to run
	Env    map[string]string // Environment variables
	Labels map[string]string // Container labels for identification
}
