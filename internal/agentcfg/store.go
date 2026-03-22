package agentcfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

// validName matches lowercase alphanumeric, hyphens, underscores.
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// builtinNames are reserved and cannot be used for custom agents.
var builtinNames = map[string]struct{}{
	"claude": {},
	"shell":  {},
}

// AgentConfig defines a custom agent that can be run in a container.
type AgentConfig struct {
	Name       string            `json:"name"`
	Command    []string          `json:"command"`
	Image      string            `json:"image,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty"` // custom Dockerfile content (extends zynqel-base)
	Env        map[string]string `json:"env,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type storeData struct {
	Agents map[string]AgentConfig `json:"agents"`
}

// Store provides thread-safe CRUD for custom agent configs backed by a JSON file.
type Store struct {
	mu   sync.RWMutex
	path string
	data storeData
}

// NewStore creates a Store that reads from and writes to the given file path.
// Call Load() to read existing configs from disk.
func NewStore(path string) *Store {
	return &Store{
		path: path,
		data: storeData{Agents: make(map[string]AgentConfig)},
	}
}

// Load reads the config file from disk. Creates the directory if needed.
// If the file doesn't exist, starts with an empty config.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil // empty store
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var data storeData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if data.Agents == nil {
		data.Agents = make(map[string]AgentConfig)
	}
	s.data = data
	return nil
}

// List returns all custom agent configs sorted by name.
func (s *Store) List() []AgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AgentConfig, 0, len(s.data.Agents))
	for _, cfg := range s.data.Agents {
		result = append(result, cfg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Get returns a custom agent config by name.
func (s *Store) Get(name string) (AgentConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.data.Agents[name]
	return cfg, ok
}

// IsCustom returns true if the name is a registered custom agent.
func (s *Store) IsCustom(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data.Agents[name]
	return ok
}

// Put creates or updates a custom agent config.
func (s *Store) Put(cfg AgentConfig) error {
	if err := validate(cfg); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if existing, ok := s.data.Agents[cfg.Name]; ok {
		cfg.CreatedAt = existing.CreatedAt
	} else {
		cfg.CreatedAt = now
	}
	cfg.UpdatedAt = now
	s.data.Agents[cfg.Name] = cfg

	return s.flush()
}

// Delete removes a custom agent config by name.
func (s *Store) Delete(name string) error {
	if _, ok := builtinNames[name]; ok {
		return fmt.Errorf("cannot delete built-in agent %q", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Agents[name]; !ok {
		return fmt.Errorf("agent %q not found", name)
	}
	delete(s.data.Agents, name)
	return s.flush()
}

// flush writes the current state to disk atomically.
// Caller must hold s.mu write lock.
func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

func validate(cfg AgentConfig) error {
	if !validName.MatchString(cfg.Name) {
		return fmt.Errorf("invalid agent name %q: must be lowercase alphanumeric, hyphens, underscores", cfg.Name)
	}
	if _, ok := builtinNames[cfg.Name]; ok {
		return fmt.Errorf("cannot use reserved name %q", cfg.Name)
	}
	if len(cfg.Command) == 0 {
		return fmt.Errorf("command is required")
	}
	return nil
}
