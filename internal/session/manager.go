package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/shortid"
)

const defaultImage = "ubuntu:22.04"

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	sandbox  sandbox.Sandbox
	policy   policy.ResourcePolicy
}

func NewManager(sb sandbox.Sandbox, p policy.ResourcePolicy) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		sandbox:  sb,
		policy:   p,
	}
}

func (m *Manager) Create(ctx context.Context, spec SessionSpec) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	if spec.Env == nil {
		spec.Env = make(map[string]string)
	}

	sbSpec := sandbox.Spec{
		Image: defaultImage,
		Env:   spec.Env,
		Labels: map[string]string{
			"zynqel.session-id": id,
		},
		MemoryBytes: m.policy.MemoryBytes(),
		NanoCPUs:    m.policy.NanoCPUs(),
	}

	containerID, err := m.sandbox.Create(ctx, sbSpec)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}

	if err := m.sandbox.Start(ctx, containerID); err != nil {
		if rmErr := m.sandbox.Remove(ctx, containerID); rmErr != nil {
			log.Printf("failed to remove container %s after start failure: %v", shortid.Format(containerID), rmErr)
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

	if err := m.sandbox.Stop(ctx, s.ContainerID); err != nil {
		log.Printf("warning: failed to stop container %s: %v", shortid.Format(s.ContainerID), err)
	}
	if err := m.sandbox.Remove(ctx, s.ContainerID); err != nil {
		log.Printf("warning: failed to remove container %s: %v", shortid.Format(s.ContainerID), err)
	}

	return nil
}

// Attach returns a PTY connection to the container for the given session.
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

	return m.sandbox.Attach(ctx, s.ContainerID)
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
		if err := m.sandbox.Stop(ctx, s.ContainerID); err != nil {
			log.Printf("warning: failed to stop container %s: %v", shortid.Format(s.ContainerID), err)
		}
		if err := m.sandbox.Remove(ctx, s.ContainerID); err != nil {
			log.Printf("warning: failed to remove container %s: %v", shortid.Format(s.ContainerID), err)
		}
		log.Printf("cleaned up session %s", s.ID)
	}
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
