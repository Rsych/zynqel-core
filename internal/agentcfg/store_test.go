package agentcfg

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCRUDAndSort(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agents.json")
	s := NewStore(path)

	if err := s.Put(AgentConfig{Name: "zeta", Command: []string{"z"}}); err != nil {
		t.Fatalf("put zeta: %v", err)
	}
	if err := s.Put(AgentConfig{Name: "alpha", Command: []string{"a"}}); err != nil {
		t.Fatalf("put alpha: %v", err)
	}

	if !s.IsCustom("zeta") {
		t.Fatalf("expected zeta to be custom")
	}

	got, ok := s.Get("alpha")
	if !ok || got.Name != "alpha" {
		t.Fatalf("get alpha failed: ok=%v got=%+v", ok, got)
	}

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "zeta" {
		t.Fatalf("list order = [%s, %s], want [alpha, zeta]", list[0].Name, list[1].Name)
	}

	if err := s.Delete("alpha"); err != nil {
		t.Fatalf("delete alpha: %v", err)
	}
	if _, ok := s.Get("alpha"); ok {
		t.Fatalf("alpha should be deleted")
	}
}

func TestStoreValidation(t *testing.T) {
	t.Parallel()

	s := NewStore(filepath.Join(t.TempDir(), "agents.json"))

	tests := []AgentConfig{
		{Name: "BadName", Command: []string{"run"}},
		{Name: "claude", Command: []string{"run"}},
		{Name: "shell", Command: []string{"run"}},
		{Name: "opencode", Command: []string{"run"}},
		{Name: "codex", Command: []string{"run"}},
		{Name: "custom", Command: nil},
	}
	for _, tc := range tests {
		if err := s.Put(tc); err == nil {
			t.Fatalf("expected validation error for %+v", tc)
		}
	}
}

func TestStoreLoadEmptyAndCorrupted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "agents.json")

	// Empty/non-existent file should load successfully.
	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("load empty store: %v", err)
	}

	// Corrupted file should fail.
	if err := os.WriteFile(path, []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if err := s.Load(); err == nil {
		t.Fatalf("expected load error for corrupted file")
	}
}

func TestStoreFlushAndTimestamps(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agents.json")
	s := NewStore(path)

	if err := s.Put(AgentConfig{
		Name:    "custom",
		Command: []string{"run"},
	}); err != nil {
		t.Fatalf("put create: %v", err)
	}
	first, ok := s.Get("custom")
	if !ok {
		t.Fatalf("custom missing after create")
	}

	time.Sleep(10 * time.Millisecond)
	if err := s.Put(AgentConfig{
		Name:    "custom",
		Command: []string{"run", "--json"},
	}); err != nil {
		t.Fatalf("put update: %v", err)
	}
	second, ok := s.Get("custom")
	if !ok {
		t.Fatalf("custom missing after update")
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("created_at changed: first=%v second=%v", first.CreatedAt, second.CreatedAt)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Fatalf("updated_at not refreshed: first=%v second=%v", first.UpdatedAt, second.UpdatedAt)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store file: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("expected persisted file content")
	}
}
