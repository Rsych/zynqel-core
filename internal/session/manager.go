package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Rsych/zynqel-core/internal/adapter"
	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

const defaultImage = "zynqel-base:latest"

const idleCheckInterval = 30 * time.Second

// ErrAtCapacity is returned when the maximum number of concurrent sessions is reached.
var ErrAtCapacity = errors.New("session capacity exceeded")

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	sandbox     sandbox.Sandbox
	policy      policy.ResourcePolicy
	idleTimeout time.Duration
	hardTimeout time.Duration
}

func NewManager(sb sandbox.Sandbox, p policy.ResourcePolicy) *Manager {
	return &Manager{
		sessions:    make(map[string]*Session),
		sandbox:     sb,
		policy:      p,
		idleTimeout: time.Duration(p.IdleTimeoutSec) * time.Second,
		hardTimeout: time.Duration(p.HardTimeoutSec) * time.Second,
	}
}

func (m *Manager) Create(ctx context.Context, spec SessionSpec) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	// Check for existing workspace session AND capacity under a single lock
	// to prevent races where two requests create duplicate sessions.
	m.mu.RLock()
	if spec.WorkspaceID != "" {
		for _, s := range m.sessions {
			if s.Spec.WorkspaceID == spec.WorkspaceID && s.Status == StatusRunning {
				m.mu.RUnlock()
				return s, nil
			}
		}
	}
	if m.policy.MaxSessions > 0 && len(m.sessions) >= m.policy.MaxSessions {
		m.mu.RUnlock()
		return nil, fmt.Errorf("at capacity (%d/%d): %w", len(m.sessions), m.policy.MaxSessions, ErrAtCapacity)
	}
	m.mu.RUnlock()

	// Validate agent and create adapter (nil for bare shell).
	agentAdapter, err := adapter.New(spec.Agent, m.sandbox)
	if err != nil {
		return nil, fmt.Errorf("create adapter: %w", err)
	}

	if spec.Env == nil {
		spec.Env = make(map[string]string)
	}

	// Image priority: committed workspace image > spec.Image > adapter.Image() > defaultImage
	img := defaultImage
	var cmd []string
	if agentAdapter != nil {
		img = agentAdapter.Image()
	} else {
		cmd = []string{"/bin/bash"}
	}
	if spec.Image != "" {
		img = spec.Image
	}

	// Check if a committed workspace image exists (from previous session).
	committedImage := "zynqel-ws-" + spec.WorkspaceID + ":latest"
	if m.sandbox.ImageExists(ctx, committedImage) {
		img = committedImage
		log.Printf("using committed workspace image %s", committedImage)
	}

	// All sessions are persistent — auto-generate workspace ID if not provided.
	if spec.WorkspaceID == "" {
		spec.WorkspaceID = id[:8]
	}
	volumeName := "zynqel-ws-" + spec.WorkspaceID

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
			return nil, fmt.Errorf("setup workspace: %w", err)
		}
	}

	// If an adapter is configured, start the agent inside the container.
	var adapterPTY sandbox.PTYConn
	if agentAdapter != nil {
		adapterPTY, err = agentAdapter.Start(ctx, containerID)
		if err != nil {
			log.Printf("adapter start failed, cleaning up container %s: %v", shortid.Format(containerID), err)
			m.cleanupSession(ctx, &Session{ID: id, ContainerID: containerID})
			return nil, fmt.Errorf("start agent adapter: %w", err)
		}
	}

	// Create the broadcaster — single PTY reader with fan-out to WS clients.
	var broadcastConn sandbox.PTYConn
	if adapterPTY != nil {
		broadcastConn = adapterPTY
	} else {
		// Shell sessions: attach once, share via broadcaster.
		broadcastConn, err = m.sandbox.Attach(ctx, containerID)
		if err != nil {
			m.cleanupSession(ctx, &Session{ID: id, ContainerID: containerID})
			return nil, fmt.Errorf("attach for broadcast: %w", err)
		}
	}

	s := &Session{
		ID:          id,
		Spec:        spec,
		Status:      StatusRunning,
		ContainerID: containerID,
		CreatedAt:   time.Now(),
		adapter:     agentAdapter,
		adapterPTY:  adapterPTY,
	}
	s.TouchActivity()
	s.broadcaster = NewBroadcaster(broadcastConn, DefaultBufferSize, s.TouchActivity)

	m.mu.Lock()
	// Recheck under write lock to handle concurrent creates.
	if m.policy.MaxSessions > 0 && len(m.sessions) >= m.policy.MaxSessions {
		m.mu.Unlock()
		log.Printf("warning: capacity race — cleaning up container %s", shortid.Format(s.ContainerID))
		m.cleanupSession(context.Background(), s)
		return nil, fmt.Errorf("at capacity (race): %w", ErrAtCapacity)
	}
	// Recheck workspace — another request may have created it while we were starting.
	if spec.WorkspaceID != "" {
		for _, existing := range m.sessions {
			if existing.Spec.WorkspaceID == spec.WorkspaceID && existing.Status == StatusRunning {
				m.mu.Unlock()
				log.Printf("warning: workspace race — cleaning up duplicate container %s", shortid.Format(s.ContainerID))
				m.cleanupSession(context.Background(), s)
				return existing, nil
			}
		}
	}
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

func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	m.cleanupSession(ctx, s)
	return nil
}

// Attach returns a PTY connection for the given session.
// If the session has an agent adapter, returns the adapter's PTY.
// Otherwise, attaches directly to the container's main process.
func (m *Manager) Attach(ctx context.Context, id string) (sandbox.PTYConn, error) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if s.Status != StatusRunning {
		return nil, fmt.Errorf("session %s is not running", id)
	}

	// If an adapter is active, return its exec PTY.
	if s.adapterPTY != nil {
		return s.adapterPTY, nil
	}

	return m.sandbox.Attach(ctx, s.ContainerID)
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
	vols, err := m.sandbox.ListVolumes(ctx, "zynqel-ws-")
	if err != nil {
		return nil, err
	}
	workspaces := make([]Workspace, 0, len(vols))
	for _, v := range vols {
		wsID := v.Name[len("zynqel-ws-"):]
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
	return m.sandbox.RemoveVolume(ctx, "zynqel-ws-"+wsID)
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
		m.cleanupSession(ctx, s)
		log.Printf("cleaned up session %s", s.ID)
	}
}

// cleanupSession stops the adapter, broadcaster, and container.
// Order matters: adapter sends SIGTERM/SIGKILL first, then broadcaster
// closes the PTY (they share the same connection).
func (m *Manager) cleanupSession(ctx context.Context, s *Session) {
	if s.adapter != nil {
		if err := s.adapter.Stop(); err != nil {
			log.Printf("warning: failed to stop adapter for session %s: %v", s.ID, err)
		}
	}
	if s.broadcaster != nil {
		s.broadcaster.Close()
	}
	// Commit container state as workspace image (preserves installed packages).
	if s.Spec.WorkspaceID != "" {
		commitImage := "zynqel-ws-" + s.Spec.WorkspaceID + ":latest"
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

	// Clone the repo into /workspace.
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

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
