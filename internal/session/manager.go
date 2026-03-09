package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Manager is the in-memory session registry.
// It's the single source of truth for all sessions.
//
// Why sync.RWMutex? HTTP handlers run concurrently (each request
// is a goroutine). Without a mutex, two requests creating sessions
// at the same time would corrupt the map. RWMutex lets multiple
// readers run in parallel but blocks writers — good for read-heavy
// workloads like listing sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates an empty session registry.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Create builds a new Session from a spec and stores it.
// Returns the created session.
func (m *Manager) Create(spec SessionSpec) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	if spec.Env == nil {
		spec.Env = make(map[string]string)
	}

	s := &Session{
		ID:        id,
		Spec:      spec,
		Status:    StatusPending,
		CreatedAt: time.Now(),
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

// List returns all sessions. Order is not guaranteed
// (map iteration in Go is random by design).
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Delete removes a session from the registry.
// Later this will also stop the container before removing.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	delete(m.sessions, id)
	return nil
}

// generateID creates a random 8-byte hex string (16 chars).
// Using crypto/rand, not math/rand — crypto/rand is safe for
// IDs that might appear in URLs or logs. No need to seed it.
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
