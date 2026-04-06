package session

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
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

var (
	validAgentName     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	validWorkspaceName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	scpLikeRepoURL     = regexp.MustCompile(`^git@[A-Za-z0-9._-]+:[A-Za-z0-9._~/\-]+(\.git)?$`)
	validBranchPattern = regexp.MustCompile(`^[A-Za-z0-9._/+~-]+$`)
	validImageRefChars = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:-]*$`)
	validEnvKey        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

const (
	maxEnvVars         = 128
	maxEnvKeyLength    = 128
	maxEnvValueLength  = 4096
	maxBranchNameBytes = 255
)

// Validate checks whether a session request is safe and well-formed.
func (s SessionSpec) Validate() error {
	if !validAgentName.MatchString(s.Agent) {
		return fmt.Errorf("invalid agent %q: must be lowercase alphanumeric, hyphens, underscores", s.Agent)
	}

	if s.RepoURL != "" {
		if err := validateRepoURL(s.RepoURL); err != nil {
			return err
		}
	}

	if s.Branch != "" {
		if err := validateBranchName(s.Branch); err != nil {
			return err
		}
	}

	if s.Image != "" {
		if !isValidDockerReference(s.Image) {
			return fmt.Errorf("invalid image %q", s.Image)
		}
	}

	if s.WorkspaceID != "" && !validWorkspaceName.MatchString(s.WorkspaceID) {
		return fmt.Errorf("invalid workspace_id %q: must be lowercase alphanumeric, hyphens, underscores", s.WorkspaceID)
	}
	if err := validateEnv(s.Env); err != nil {
		return err
	}

	return nil
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

func validateRepoURL(raw string) error {
	if scpLikeRepoURL.MatchString(raw) {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 || hasDotDotPathSegment(parts[1]) {
			return fmt.Errorf("invalid repo_url %q", raw)
		}
		return nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid repo_url %q", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ssh" {
		return fmt.Errorf("invalid repo_url %q: unsupported scheme", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid repo_url %q: missing host", raw)
	}
	return nil
}

func validateBranchName(branch string) error {
	if len(branch) > maxBranchNameBytes {
		return fmt.Errorf("invalid branch %q", branch)
	}
	if !validBranchPattern.MatchString(branch) {
		return fmt.Errorf("invalid branch %q", branch)
	}
	if strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return fmt.Errorf("invalid branch %q", branch)
	}
	if strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.Contains(branch, "//") {
		return fmt.Errorf("invalid branch %q", branch)
	}
	if strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("invalid branch %q", branch)
	}
	return nil
}

func isValidDockerReference(ref string) bool {
	if len(ref) == 0 || len(ref) > 255 {
		return false
	}
	if strings.Contains(ref, "://") || strings.ContainsAny(ref, " \t\r\n") {
		return false
	}
	if strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") || strings.Contains(ref, "//") {
		return false
	}
	if !validImageRefChars.MatchString(ref) {
		return false
	}
	for _, p := range strings.Split(ref, "/") {
		if p == "" {
			return false
		}
	}
	return true
}

func hasDotDotPathSegment(path string) bool {
	for _, p := range strings.Split(path, "/") {
		if p == ".." {
			return true
		}
	}
	return false
}

func validateEnv(env map[string]string) error {
	if len(env) > maxEnvVars {
		return fmt.Errorf("too many env vars: max %d", maxEnvVars)
	}
	for k, v := range env {
		if len(k) == 0 || len(k) > maxEnvKeyLength || !validEnvKey.MatchString(k) {
			return fmt.Errorf("invalid env key %q", k)
		}
		if len(v) > maxEnvValueLength {
			return fmt.Errorf("env value too long for key %q", k)
		}
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("invalid env value for key %q", k)
		}
	}
	return nil
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
