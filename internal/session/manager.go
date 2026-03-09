package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Rsych/zynqel-core/internal/sandbox"
)

const defaultImage = "ubuntu:22.04"

// Manager is the in-memory session registry.
// It owns the sandbox — all container operations go through here.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	sandbox  sandbox.Sandbox
}

// NewManager creates a session registry wired to a sandbox backend.
func NewManager(sb sandbox.Sandbox) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		sandbox:  sb,
	}
}

// Create builds a new Session, provisions a container, and starts it.
// The flow: generate ID → create container → start container → store session.
// If any step fails, we clean up what we created.
func (m *Manager) Create(spec SessionSpec) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	if spec.Env == nil {
		spec.Env = make(map[string]string)
	}

	// Build sandbox spec from session spec.
	// The sandbox doesn't know about agents or repos —
	// it just needs an image, env vars, and labels.
	sbSpec := sandbox.Spec{
		Image: defaultImage,
		Env:   spec.Env,
		Labels: map[string]string{
			"zynqel.managed":    "true",
			"zynqel.session-id": id,
		},
	}

	containerID, err := m.sandbox.Create(sbSpec)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}

	if err := m.sandbox.Start(containerID); err != nil {
		// Clean up the created-but-not-started container.
		if rmErr := m.sandbox.Remove(containerID); rmErr != nil {
			log.Printf("failed to remove container %s after start failure: %v", containerID[:12], rmErr)
		}
		return nil, fmt.Errorf("start sandbox: %w", err)
	}

	s := &Session{
		ID:          id,
		Spec:        spec,
		Status:      StatusRunning,
		ContainerID: containerID,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	return s, nil
}

// Get returns a session by ID, or an error if not found.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return s, nil
}

// List returns all sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Delete stops and removes the container, then removes the session.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	// Stop and remove outside the lock — these are slow I/O operations.
	// We already removed from the map, so no other request can reference it.
	if err := m.sandbox.Stop(s.ContainerID); err != nil {
		log.Printf("warning: failed to stop container %s: %v", s.ContainerID[:12], err)
	}
	if err := m.sandbox.Remove(s.ContainerID); err != nil {
		log.Printf("warning: failed to remove container %s: %v", s.ContainerID[:12], err)
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
