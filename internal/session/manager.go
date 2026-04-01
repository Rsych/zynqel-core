package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rsych/zynqel-core/internal/adapter"
	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/sessionlog"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

const defaultImage = "zynqel-base:latest"

const idleCheckInterval = 30 * time.Second

const volumePrefix = "zynqel-ws-"

const stopCleanupTimeout = 35 * time.Second // slightly longer than the 30s cleanup context

// validWorkspaceID matches lowercase alphanumeric, hyphens, underscores.
var validWorkspaceID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// waitStopDone waits for a session's background stop cleanup to finish, with a timeout.
func waitStopDone(s *Session) {
	if s.stopDone == nil {
		return
	}
	select {
	case <-s.stopDone:
	case <-time.After(stopCleanupTimeout):
		log.Printf("warning: stop cleanup timed out for session %s", s.ID)
	}
}

// ErrAtCapacity is returned when the maximum number of concurrent sessions is reached.
var ErrAtCapacity = errors.New("session capacity exceeded")

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	creating    map[string]struct{} // workspace IDs currently being created (prevents races)
	sandbox     sandbox.Sandbox
	policy      policy.ResourcePolicy
	agents      *agentcfg.Store
	logStore    *sessionlog.Store
	idleTimeout time.Duration
	hardTimeout time.Duration
}

func NewManager(sb sandbox.Sandbox, p policy.ResourcePolicy, agents *agentcfg.Store, logStore *sessionlog.Store) *Manager {
	return &Manager{
		sessions:    make(map[string]*Session),
		creating:    make(map[string]struct{}),
		sandbox:     sb,
		policy:      p,
		agents:      agents,
		logStore:    logStore,
		idleTimeout: time.Duration(p.IdleTimeoutSec) * time.Second,
		hardTimeout: time.Duration(p.HardTimeoutSec) * time.Second,
	}
}

func (m *Manager) Create(ctx context.Context, spec SessionSpec) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	// Use write lock for all checks + creating guard to prevent race conditions
	// where two requests create containers for the same workspace concurrently.
	m.mu.Lock()
	if spec.WorkspaceID != "" {
		for _, s := range m.sessions {
			if s.Spec.WorkspaceID == spec.WorkspaceID && s.Status == StatusRunning {
				m.mu.Unlock()
				return s, nil
			}
		}
		// Clean up stopped sessions with same workspace.
		for sid, s := range m.sessions {
			if s.Spec.WorkspaceID == spec.WorkspaceID && s.Status == StatusStopped {
				delete(m.sessions, sid)
				go func() {
					waitStopDone(s)
					_ = m.sandbox.Remove(context.Background(), s.ContainerID)
				}()
				break
			}
		}
		// Check if another goroutine is already creating a session for this workspace.
		if _, ok := m.creating[spec.WorkspaceID]; ok {
			m.mu.Unlock()
			return nil, fmt.Errorf("workspace %s is already being created", spec.WorkspaceID)
		}
		m.creating[spec.WorkspaceID] = struct{}{}
	}
	if m.policy.MaxSessions > 0 && len(m.sessions) >= m.policy.MaxSessions {
		if spec.WorkspaceID != "" {
			delete(m.creating, spec.WorkspaceID)
		}
		m.mu.Unlock()
		return nil, fmt.Errorf("at capacity (%d/%d): %w", len(m.sessions), m.policy.MaxSessions, ErrAtCapacity)
	}
	m.mu.Unlock()

	// Release the creating guard when we're done (success or failure).
	if spec.WorkspaceID != "" {
		defer func() {
			m.mu.Lock()
			delete(m.creating, spec.WorkspaceID)
			m.mu.Unlock()
		}()
	}

	// Validate agent and create adapter (nil for bare shell).
	agentAdapter, err := adapter.New(spec.Agent, m.sandbox, m.agents)
	if err != nil {
		return nil, fmt.Errorf("create adapter: %w", err)
	}

	if spec.Env == nil {
		spec.Env = make(map[string]string)
	}

	// Resolve workspace ID first (needed for committed image check).
	if spec.WorkspaceID == "" {
		spec.WorkspaceID = id[:8]
	}
	spec.WorkspaceID = strings.ToLower(spec.WorkspaceID)
	if !validWorkspaceID.MatchString(spec.WorkspaceID) {
		return nil, fmt.Errorf("invalid workspace_id %q: must be lowercase alphanumeric, hyphens, underscores", spec.WorkspaceID)
	}
	volumeName := volumePrefix + spec.WorkspaceID

	// Image priority: committed workspace (has auth/packages) > spec.Image > adapter > default
	img := defaultImage
	cmd := []string{"/bin/bash"} // Always start as shell — user launches agent manually.
	if agentAdapter != nil {
		img = agentAdapter.Image()
	}
	if spec.Image != "" {
		img = spec.Image
	}
	// Committed workspace image wins — it has installed packages + auth tokens.
	committedImage := volumePrefix + spec.WorkspaceID + ":latest"
	if m.sandbox.ImageExists(ctx, committedImage) {
		img = committedImage
		log.Printf("using committed workspace image %s", committedImage)
	}

	// Inject git token as GITHUB_TOKEN env var (used by Claude Code, gh CLI, etc.).
	if spec.GitToken != "" {
		spec.Env["GITHUB_TOKEN"] = spec.GitToken
	}

	sbSpec := sandbox.Spec{
		Image: img,
		Cmd:   cmd,
		Env:   spec.Env,
		Labels: map[string]string{
			"zynqel.session-id": id,
		},
		MemoryBytes: m.policy.MemoryBytes(),
		NanoCPUs:    m.policy.NanoCPUs(),
		VolumeName:  volumeName,
		VolumeLabels: map[string]string{
			"zynqel.managed": "true",
			"zynqel.image":   img,
			"zynqel.agent":   spec.Agent,
		},
	}

	// Mount host SSH key directory to a temp path (read-only).
	// setupGitCredentials copies keys to /root/.ssh with correct permissions.
	if spec.SSHKeyPath != "" {
		sbSpec.BindMounts = append(sbSpec.BindMounts, sandbox.BindMount{
			Source:   spec.SSHKeyPath,
			Target:   "/tmp/.ssh-host",
			ReadOnly: true,
		})
	}

	containerID, err := m.sandbox.Create(ctx, sbSpec)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}

	if err := m.sandbox.Start(ctx, containerID); err != nil {
		m.cleanupSession(ctx, &Session{ID: id, ContainerID: containerID})
		return nil, fmt.Errorf("start sandbox: %w", err)
	}

	// Setup workspace: clone repo and checkout branch before agent starts.
	if spec.RepoURL != "" {
		if err := m.setupWorkspace(ctx, containerID, spec); err != nil {
			log.Printf("workspace setup failed, cleaning up container %s: %v", shortid.Format(containerID), err)
			m.cleanupSession(ctx, &Session{ID: id, ContainerID: containerID})
			// Remove the empty volume so failed workspaces don't linger.
			_ = m.sandbox.RemoveVolume(ctx, volumeName)
			return nil, fmt.Errorf("setup workspace: %w", err)
		}
	}

	// Show welcome banner with available agent info (idempotent — checks before appending).
	if spec.Agent != "" && spec.Agent != "shell" {
		marker := fmt.Sprintf("# zynqel-agent:%s", spec.Agent)
		welcomeCmd := fmt.Sprintf(
			`grep -q '%s' /root/.bashrc 2>/dev/null || (`+
				`echo '%s' >> /root/.bashrc; `+
				`echo 'echo ""' >> /root/.bashrc; `+
				`echo 'echo "  \033[1;32m┌────────────────────────────────────────┐\033[0m"' >> /root/.bashrc; `+
				`echo 'echo "  \033[1;32m│\033[0m  \033[1;32m▸ %s\033[0m is ready.%-*s\033[1;32m│\033[0m"' >> /root/.bashrc; `+
				`echo 'echo "  \033[1;32m│\033[0m    Run \033[1m%s\033[0m to start.%-*s\033[1;32m│\033[0m"' >> /root/.bashrc; `+
				`echo 'echo "  \033[1;32m└────────────────────────────────────────┘\033[0m"' >> /root/.bashrc; `+
				`echo 'echo ""' >> /root/.bashrc)`,
			marker, marker,
			spec.Agent, 27-len(spec.Agent), "",
			spec.Agent, 22-len(spec.Agent), "")
		_, _ = m.sandbox.ExecRun(ctx, containerID, []string{"sh", "-c", welcomeCmd})
	}

	// Always start as shell — user launches agent tools manually.
	broadcastConn, err := m.sandbox.Attach(ctx, containerID)
	if err != nil {
		m.cleanupSession(ctx, &Session{ID: id, ContainerID: containerID})
		return nil, fmt.Errorf("attach for broadcast: %w", err)
	}

	// Open PTY log writer if session logging is enabled.
	var logWriter io.WriteCloser
	if m.logStore != nil && m.logStore.LogPTY() {
		w, err := m.logStore.OpenLogWriter(id)
		if err != nil {
			log.Printf("warning: failed to open PTY log for session %s: %v", id, err)
		} else {
			logWriter = w
		}
	}

	s := &Session{
		ID:          id,
		Spec:        spec,
		Status:      StatusRunning,
		ContainerID: containerID,
		CreatedAt:   time.Now(),
	}
	s.TouchActivity()
	s.broadcaster = NewBroadcaster(broadcastConn, DefaultBufferSize, s.TouchActivity, nil, logWriter)

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	return s, nil
}

func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return s, nil
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Stop gracefully stops a running session but keeps it in the list.
// Sets status to stopped immediately, then cleans up in the background.
func (m *Manager) Stop(_ context.Context, id string) error {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	if s.Status != StatusRunning {
		return fmt.Errorf("session %s is not running", id)
	}

	// Mark stopped immediately so the API responds fast.
	now := time.Now()
	s.Status = StatusStopped
	s.StoppedAt = &now
	s.stopDone = make(chan struct{})

	// Heavy cleanup in background — commit, then stop container.
	// Note: broadcaster.Close() triggers readLoop exit, which closes the PTY logWriter
	// via its defer chain (see readLoop in broadcaster.go).
	go func() {
		defer close(s.stopDone)
		if !atomic.CompareAndSwapInt32(&s.cleaned, 0, 1) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if s.broadcaster != nil {
			s.broadcaster.Close()
		}
		if s.Spec.WorkspaceID != "" {
			commitImage := volumePrefix + s.Spec.WorkspaceID + ":latest"
			if err := m.sandbox.Commit(ctx, s.ContainerID, commitImage); err != nil {
				log.Printf("warning: failed to commit workspace %s: %v", s.Spec.WorkspaceID, err)
			}
		}
		if err := m.sandbox.Stop(ctx, s.ContainerID); err != nil {
			log.Printf("warning: failed to stop container %s: %v", shortid.Format(s.ContainerID), err)
		}
		m.saveSessionRecord(s)
		log.Printf("session %s stopped", s.ID)
	}()

	return nil
}

// Restart creates a new session from a stopped session's spec.
// Removes the old session and creates a fresh one with the same workspace.
func (m *Manager) Restart(ctx context.Context, id string) (*Session, error) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("session not found: %s", id)
	}

	spec := s.Spec
	delete(m.sessions, id)
	m.mu.Unlock()

	// Wait for background stop cleanup to finish (commit must complete before remove).
	waitStopDone(s)

	if s.Status == StatusStopped {
		_ = m.sandbox.Remove(ctx, s.ContainerID)
	} else {
		m.cleanupSession(ctx, s)
	}

	return m.Create(ctx, spec)
}

// Delete removes a session entirely — stops it if running, then removes the container.
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	// Wait for background stop cleanup to finish before removing.
	waitStopDone(s)

	if s.Status == StatusStopped {
		// Stop goroutine already saved the session record.
		_ = m.sandbox.Remove(ctx, s.ContainerID)
	} else {
		m.cleanupSession(ctx, s)
		m.saveSessionRecord(s)
	}
	return nil
}

// Subscribe returns a broadcast subscription for the session's PTY output.
// replay contains buffered output for reconnect. sub.Ch receives live output.
// Caller must call Unsubscribe(id, sub) when done.
func (m *Manager) Subscribe(id string) (replay []byte, sub *Subscriber, err error) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return nil, nil, fmt.Errorf("session not found: %s", id)
	}
	if s.Status != StatusRunning {
		return nil, nil, fmt.Errorf("session %s is not running", id)
	}

	replay, sub = s.broadcaster.Subscribe()
	return replay, sub, nil
}

// Unsubscribe removes a subscriber from the session's broadcaster.
func (m *Manager) Unsubscribe(id string, sub *Subscriber) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return
	}
	s.broadcaster.Unsubscribe(sub)
}

// WriteInput sends input to the session's PTY.
func (m *Manager) WriteInput(id string, data []byte) error {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	return s.broadcaster.Write(data)
}

// Resize updates the PTY dimensions for the session's container.
func (m *Manager) Resize(id string, cols, rows int) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok || s.Status != StatusRunning {
		return
	}

	if err := m.sandbox.Resize(context.Background(), s.ContainerID, cols, rows); err != nil {
		log.Printf("warning: failed to resize session %s: %v", id, err)
	}
}

// Workspace represents a saved workspace volume.
type Workspace struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Image     string `json:"image,omitempty"`
	Agent     string `json:"agent,omitempty"`
}

// ListWorkspaces returns all saved workspace volumes.
func (m *Manager) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	vols, err := m.sandbox.ListVolumes(ctx, volumePrefix)
	if err != nil {
		return nil, err
	}
	workspaces := make([]Workspace, 0, len(vols))
	for _, v := range vols {
		wsID := strings.TrimPrefix(v.Name, volumePrefix)
		if wsID == v.Name {
			continue // not a zynqel workspace volume
		}
		workspaces = append(workspaces, Workspace{
			ID:        wsID,
			CreatedAt: v.CreatedAt,
			Image:     v.Image,
			Agent:     v.Agent,
		})
	}
	return workspaces, nil
}

// DeleteWorkspace removes a workspace volume.
func (m *Manager) DeleteWorkspace(ctx context.Context, wsID string) error {
	return m.sandbox.RemoveVolume(ctx, volumePrefix+wsID)
}

// RenameWorkspace renames a workspace by copying its volume and committed image.
// Returns an error if the workspace has a running session or the new ID already exists.
// Uses the creating guard to prevent new sessions from starting on the workspace during rename.
func (m *Manager) RenameWorkspace(ctx context.Context, oldID, newID string) error {
	newID = strings.ToLower(newID)
	if !validWorkspaceID.MatchString(newID) {
		return fmt.Errorf("invalid workspace_id %q: must be lowercase alphanumeric, hyphens, underscores", newID)
	}
	if oldID == newID {
		return nil
	}

	// Hold write lock for check + creating guard to prevent TOCTOU race:
	// no new session can start on oldID while the rename is in progress.
	m.mu.Lock()
	for _, s := range m.sessions {
		if s.Spec.WorkspaceID == oldID && s.Status == StatusRunning {
			m.mu.Unlock()
			return fmt.Errorf("cannot rename workspace with active session")
		}
	}
	if _, ok := m.creating[oldID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("workspace %s is currently being created", oldID)
	}
	m.creating[oldID] = struct{}{}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.creating, oldID)
		m.mu.Unlock()
	}()

	// Check target volume doesn't already exist.
	oldVolume := volumePrefix + oldID
	newVolume := volumePrefix + newID
	vols, err := m.sandbox.ListVolumes(ctx, newVolume)
	if err != nil {
		return fmt.Errorf("check target volume: %w", err)
	}
	for _, v := range vols {
		if v.Name == newVolume {
			return fmt.Errorf("workspace %q already exists", newID)
		}
	}

	// Copy volume data.
	if err := m.sandbox.CopyVolume(ctx, oldVolume, newVolume); err != nil {
		return fmt.Errorf("copy volume: %w", err)
	}

	// Rename committed image if it exists.
	oldImage := oldVolume + ":latest"
	newImage := newVolume + ":latest"
	if m.sandbox.ImageExists(ctx, oldImage) {
		if err := m.sandbox.TagImage(ctx, oldImage, newImage); err != nil {
			// Rollback: remove the new volume.
			_ = m.sandbox.RemoveVolume(ctx, newVolume)
			return fmt.Errorf("tag image: %w", err)
		}
		// Best-effort removal of old tag; new tag already exists.
		if err := m.sandbox.RemoveImage(ctx, oldImage); err != nil {
			log.Printf("warning: failed to remove old image tag %s (new tag exists): %v", oldImage, err)
		}
	}

	// Remove old volume.
	if err := m.sandbox.RemoveVolume(ctx, oldVolume); err != nil {
		log.Printf("warning: failed to remove old volume %s: %v", oldVolume, err)
	}

	// Update any stopped sessions referencing the old workspace ID.
	m.mu.Lock()
	for _, s := range m.sessions {
		if s.Spec.WorkspaceID == oldID {
			s.Spec.WorkspaceID = newID
		}
	}
	m.mu.Unlock()

	log.Printf("renamed workspace %s → %s", oldID, newID)
	return nil
}

// Stats returns container resource usage for a session.
func (m *Manager) Stats(ctx context.Context, id string) (*sandbox.ContainerStats, error) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if s.Status != StatusRunning {
		return nil, fmt.Errorf("session %s is not running", id)
	}

	return m.sandbox.Stats(ctx, s.ContainerID)
}

// ActiveCount returns the number of active sessions.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Policy returns the resource policy.
func (m *Manager) Policy() policy.ResourcePolicy {
	return m.policy
}

// Shutdown stops and removes all active sessions.
// Called during server shutdown to clean up containers.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, s := range sessions {
		waitStopDone(s)
		if s.Status == StatusStopped {
			// Stop goroutine already saved the session record.
			_ = m.sandbox.Remove(ctx, s.ContainerID)
		} else {
			m.cleanupSession(ctx, s)
			m.saveSessionRecord(s)
		}
		log.Printf("cleaned up session %s", s.ID)
	}
}

// cleanupSession stops the broadcaster and container.
// Safe to call multiple times — guarded by atomic flag.
// Does NOT save a session history record; callers that need persistence
// should call saveSessionRecord separately (early-error cleanup during Create
// passes incomplete sessions that shouldn't be recorded).
func (m *Manager) cleanupSession(ctx context.Context, s *Session) {
	if !atomic.CompareAndSwapInt32(&s.cleaned, 0, 1) {
		_ = m.sandbox.Remove(ctx, s.ContainerID)
		return
	}
	if s.broadcaster != nil {
		s.broadcaster.Close()
	}
	// Commit container state as workspace image (preserves installed packages).
	if s.Spec.WorkspaceID != "" {
		commitImage := volumePrefix + s.Spec.WorkspaceID + ":latest"
		if err := m.sandbox.Commit(ctx, s.ContainerID, commitImage); err != nil {
			log.Printf("warning: failed to commit workspace %s: %v", s.Spec.WorkspaceID, err)
		}
	}
	if err := m.sandbox.Stop(ctx, s.ContainerID); err != nil {
		log.Printf("warning: failed to stop container %s: %v", shortid.Format(s.ContainerID), err)
	}
	if err := m.sandbox.Remove(ctx, s.ContainerID); err != nil {
		log.Printf("warning: failed to remove container %s: %v", shortid.Format(s.ContainerID), err)
	}
}

// StartTimeoutChecker runs a background goroutine that terminates idle
// and expired sessions. Returns immediately. Stops when ctx is cancelled.
// Does nothing if both idle and hard timeouts are disabled (0).
func (m *Manager) StartTimeoutChecker(ctx context.Context) {
	if m.idleTimeout <= 0 && m.hardTimeout <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(idleCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.reapSessions(ctx)
			}
		}
	}()
	log.Printf("timeout checker started (idle=%v, hard=%v, interval=%v)",
		m.idleTimeout, m.hardTimeout, idleCheckInterval)
}

func (m *Manager) reapSessions(ctx context.Context) {
	type reapEntry struct {
		id     string
		reason string
	}

	m.mu.RLock()
	var toReap []reapEntry
	for id, s := range m.sessions {
		if s.Status != StatusRunning {
			continue
		}
		// Hard timeout takes priority — cannot be extended by activity.
		if m.hardTimeout > 0 && time.Since(s.CreatedAt) > m.hardTimeout {
			toReap = append(toReap, reapEntry{id, fmt.Sprintf("hard-timeout (alive %v)", time.Since(s.CreatedAt).Truncate(time.Second))})
		} else if m.idleTimeout > 0 && s.IdleSince() > m.idleTimeout {
			toReap = append(toReap, reapEntry{id, fmt.Sprintf("idle-timeout (idle %v)", s.IdleSince().Truncate(time.Second))})
		}
	}
	m.mu.RUnlock()

	for _, e := range toReap {
		log.Printf("session %s %s", e.id, e.reason)
		if err := m.Delete(ctx, e.id); err != nil {
			log.Printf("warning: failed to delete session %s: %v", e.id, err)
		}
	}
}

// setupWorkspace clones a repo and checks out the specified branch inside the container.
func (m *Manager) setupWorkspace(ctx context.Context, containerID string, spec SessionSpec) error {
	// If workspace already has a git repo (persistent volume), skip clone.
	if _, err := m.sandbox.ExecRun(ctx, containerID, []string{"test", "-d", "/workspace/.git"}); err == nil {
		log.Printf("workspace already populated, skipping clone")
		if spec.Branch != "" {
			checkoutCmd := []string{"git", "-C", "/workspace", "checkout", spec.Branch}
			if _, err := m.sandbox.ExecRun(ctx, containerID, checkoutCmd); err != nil {
				return fmt.Errorf("git checkout %s: %w", spec.Branch, err)
			}
			log.Printf("checked out branch %s", spec.Branch)
		}
		return nil
	}

	// Check if /workspace is non-empty (populated volume but no .git).
	output, _ := m.sandbox.ExecRun(ctx, containerID, []string{"sh", "-c", "ls -A /workspace | head -1"})
	if len(output) > 0 {
		log.Printf("workspace has files but no .git, skipping clone")
		return nil
	}

	// Set up git credentials before clone.
	if err := m.setupGitCredentials(ctx, containerID, spec); err != nil {
		log.Printf("warning: git credential setup: %v", err)
	}

	// Clone the repo into /workspace.
	// Credentials are handled by the credential helper set up in setupGitCredentials.
	cloneCmd := []string{"git", "clone", spec.RepoURL, "/workspace"}
	if _, err := m.sandbox.ExecRun(ctx, containerID, cloneCmd); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	log.Printf("cloned %s into /workspace", spec.RepoURL)

	// Checkout branch if specified.
	if spec.Branch != "" {
		checkoutCmd := []string{"git", "-C", "/workspace", "checkout", spec.Branch}
		if _, err := m.sandbox.ExecRun(ctx, containerID, checkoutCmd); err != nil {
			return fmt.Errorf("git checkout %s: %w", spec.Branch, err)
		}
		log.Printf("checked out branch %s", spec.Branch)
	}

	return nil
}

// setupGitCredentials configures git auth inside the container.
func (m *Manager) setupGitCredentials(ctx context.Context, containerID string, spec SessionSpec) error {
	// Token-based auth: set up git credential store.
	if spec.GitToken != "" {
		u, err := url.Parse(spec.RepoURL)
		if err != nil {
			return fmt.Errorf("invalid repo URL for token auth: %w", err)
		}
		if u.Host == "" {
			return fmt.Errorf("repo URL missing host for token auth")
		}
		// Configure credential helper so git push/pull works for the agent.
		cmds := [][]string{
			{"git", "config", "--global", "credential.helper", "store"},
			// Write credentials using GITHUB_TOKEN env var (already injected) to avoid shell injection.
			{"sh", "-c", fmt.Sprintf(`printf 'https://x-access-token:%%s@%s\n' "$GITHUB_TOKEN" >> /root/.git-credentials`, u.Host)},
		}
		for _, cmd := range cmds {
			if _, err := m.sandbox.ExecRun(ctx, containerID, cmd); err != nil {
				return fmt.Errorf("credential setup: %w", err)
			}
		}
		log.Printf("configured git token credentials in container")
	}

	// SSH key mount: copy from read-only bind mount to /root/.ssh with correct permissions.
	if spec.SSHKeyPath != "" {
		cmds := [][]string{
			{"sh", "-c", "cp -r /tmp/.ssh-host /root/.ssh"},
			{"chmod", "700", "/root/.ssh"},
			{"sh", "-c", "chmod 600 /root/.ssh/* 2>/dev/null || true"},
			{"sh", "-c", "ssh-keyscan github.com gitlab.com bitbucket.org >> /root/.ssh/known_hosts 2>/dev/null"},
		}
		for _, cmd := range cmds {
			if _, err := m.sandbox.ExecRun(ctx, containerID, cmd); err != nil {
				return fmt.Errorf("ssh setup: %w", err)
			}
		}
		log.Printf("configured SSH keys in container")
	}

	return nil
}

// saveSessionRecord persists session metadata to the log store.
func (m *Manager) saveSessionRecord(s *Session) {
	if m.logStore == nil {
		return
	}
	r := sessionlog.Record{
		ID:          s.ID,
		WorkspaceID: s.Spec.WorkspaceID,
		Agent:       s.Spec.Agent,
		Image:       s.Spec.Image,
		RepoURL:     s.Spec.RepoURL,
		Branch:      s.Spec.Branch,
		Status:      string(s.Status),
		CreatedAt:   s.CreatedAt,
	}
	if s.StoppedAt != nil {
		r.StoppedAt = *s.StoppedAt
	} else {
		r.StoppedAt = time.Now()
	}
	if s.Error != "" {
		r.Error = s.Error
	}
	if err := m.logStore.Save(r); err != nil {
		log.Printf("warning: failed to save session record %s: %v", s.ID, err)
	}
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
