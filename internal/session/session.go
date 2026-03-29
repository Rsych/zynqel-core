package session

import (
	"encoding/json"
	"sync/atomic"
	"time"
)

// Status represents the lifecycle state of a session.
// Using a named type (not raw string) gives us type safety —
// you can't accidentally pass "banana" where a Status is expected.
type Status string

const (
	StatusPending Status = "pending" // created, not yet running
	StatusRunning Status = "running" // container is up
	StatusStopped Status = "stopped" // gracefully stopped
	StatusError   Status = "error"   // something went wrong
)

// SessionSpec is the request payload — what the caller wants.
// Think of it as the "desired state" for a session.
// The actual running state lives in Session.
type SessionSpec struct {
	Agent       string            `json:"agent"`                  // e.g. "claude", "cursor", "shell"
	Image       string            `json:"image,omitempty"`        // custom Docker image (overrides agent default)
	WorkspaceID string            `json:"workspace_id,omitempty"` // persistent volume ID (empty = ephemeral)
	RepoURL     string            `json:"repo_url,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	GitToken    string            `json:"git_token,omitempty"`    // PAT for private HTTPS repos
	SSHKeyPath  string            `json:"ssh_key_path,omitempty"` // host path to SSH key dir (bind mount)
	Env         map[string]string `json:"env,omitempty"`
}

// MarshalJSON redacts sensitive fields from API responses.
func (s SessionSpec) MarshalJSON() ([]byte, error) {
	type Alias SessionSpec
	a := Alias(s)
	if a.GitToken != "" {
		a.GitToken = "***"
	}
	if a.Env != nil {
		redacted := make(map[string]string, len(a.Env))
		for k := range a.Env {
			redacted[k] = "***"
		}
		a.Env = redacted
	}
	return json.Marshal(a)
}

// Session is the runtime state — what actually exists.
// It's created from a SessionSpec and tracks everything
// about a running (or stopped) session.
type Session struct {
	ID          string      `json:"id"`
	Spec        SessionSpec `json:"spec"`
	Status      Status      `json:"status"`
	ContainerID string      `json:"container_id,omitempty"` // Docker container ID
	CreatedAt   time.Time   `json:"created_at"`
	StoppedAt   *time.Time  `json:"stopped_at,omitempty"`
	Error       string      `json:"error,omitempty"`

	// Unexported — managed by session.Manager, not serialized.
	broadcaster  *Broadcaster  // output fan-out + ring buffer
	lastActivity int64         // unix timestamp, updated atomically
	cleaned      int32         // atomic flag: 1 = cleanup done
	stopDone     chan struct{} // closed when background stop cleanup finishes
}

// TouchActivity records current time as last activity.
// Safe to call from any goroutine.
func (s *Session) TouchActivity() {
	atomic.StoreInt64(&s.lastActivity, time.Now().Unix())
}

// IdleSince returns the duration since the last activity.
func (s *Session) IdleSince() time.Duration {
	last := atomic.LoadInt64(&s.lastActivity)
	if last == 0 {
		return time.Since(s.CreatedAt)
	}
	return time.Since(time.Unix(last, 0))
}
