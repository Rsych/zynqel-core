package session

import "time"

// Status represents the lifecycle state of a session.
// Using a named type (not raw string) gives us type safety —
// you can't accidentally pass "banana" where a Status is expected.
type Status string

const (
	StatusPending  Status = "pending"  // created, not yet running
	StatusRunning  Status = "running"  // container is up
	StatusStopped  Status = "stopped"  // gracefully stopped
	StatusError    Status = "error"    // something went wrong
)

// SessionSpec is the request payload — what the caller wants.
// Think of it as the "desired state" for a session.
// The actual running state lives in Session.
type SessionSpec struct {
	Agent    string            `json:"agent"`              // e.g. "claude", "cursor"
	RepoURL  string           `json:"repo_url,omitempty"`
	Branch   string           `json:"branch,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// Session is the runtime state — what actually exists.
// It's created from a SessionSpec and tracks everything
// about a running (or stopped) session.
type Session struct {
	ID        string    `json:"id"`
	Spec      SessionSpec `json:"spec"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"` // pointer = nullable in JSON
	Error     string    `json:"error,omitempty"`
}
