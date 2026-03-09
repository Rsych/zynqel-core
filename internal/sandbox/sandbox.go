package sandbox

// Sandbox defines the contract for an execution backend.
// Right now there's only DockerSandbox, but this interface
// means we could add others (e.g., Firecracker) without
// changing the session manager.
//
// Note: we define this interface in the sandbox package because
// it's the primary abstraction here. The session manager will
// depend on this interface, not on the concrete DockerSandbox.
type Sandbox interface {
	// Create provisions a new sandbox (e.g., creates a container).
	// Returns a unique sandbox ID.
	Create(spec Spec) (string, error)

	// Start starts the sandbox (e.g., starts the container).
	Start(id string) error

	// Stop stops the sandbox gracefully.
	Stop(id string) error

	// Remove destroys the sandbox and cleans up resources.
	Remove(id string) error
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
