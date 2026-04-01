package sessionlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, false, 30)
	if err != nil {
		t.Fatal(err)
	}

	r := Record{
		ID:          "abc123",
		WorkspaceID: "my-ws",
		Agent:       "claude",
		Status:      "stopped",
		CreatedAt:   time.Now().Add(-10 * time.Minute),
		StoppedAt:   time.Now(),
	}
	if err := store.Save(r); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkspaceID != "my-ws" {
		t.Errorf("got workspace_id=%q, want my-ws", got.WorkspaceID)
	}
	if got.Duration == "" {
		t.Error("expected duration to be computed")
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, false, 30)
	if err != nil {
		t.Fatal(err)
	}

	// Save two records with different timestamps.
	r1 := Record{ID: "older", Agent: "shell", Status: "stopped", CreatedAt: time.Now().Add(-2 * time.Hour)}
	r2 := Record{ID: "newer", Agent: "claude", Status: "stopped", CreatedAt: time.Now().Add(-1 * time.Hour)}
	_ = store.Save(r1)
	_ = store.Save(r2)

	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	// Should be sorted newest first.
	if records[0].ID != "newer" {
		t.Errorf("first record should be newer, got %s", records[0].ID)
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, true, 30)
	if err != nil {
		t.Fatal(err)
	}

	r := Record{ID: "del-me", Agent: "shell", Status: "stopped", CreatedAt: time.Now()}
	_ = store.Save(r)

	// Create a log file too.
	w, err := store.OpenLogWriter("del-me")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("hello"))
	_ = w.Close()

	_ = store.Delete("del-me")

	if _, err := store.Get("del-me"); err == nil {
		t.Error("expected record to be deleted")
	}
	if _, err := store.ReadLog("del-me"); err == nil {
		t.Error("expected log to be deleted")
	}
}

func TestStoreCleanup(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, false, 7)
	if err != nil {
		t.Fatal(err)
	}

	old := Record{ID: "old", Agent: "shell", Status: "stopped", CreatedAt: time.Now().AddDate(0, 0, -10)}
	recent := Record{ID: "recent", Agent: "shell", Status: "stopped", CreatedAt: time.Now()}
	_ = store.Save(old)
	_ = store.Save(recent)

	removed, err := store.Cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	records, _ := store.List()
	if len(records) != 1 || records[0].ID != "recent" {
		t.Error("expected only recent record to remain")
	}
}

func TestStoreLogWriter(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, true, 30)
	if err != nil {
		t.Fatal(err)
	}

	w, err := store.OpenLogWriter("log-test")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("terminal output"))
	_ = w.Close()

	// Save record — should detect the log file.
	r := Record{ID: "log-test", Agent: "claude", Status: "stopped", CreatedAt: time.Now()}
	_ = store.Save(r)

	got, _ := store.Get("log-test")
	if !got.HasLog {
		t.Error("expected HasLog=true")
	}

	rc, err := store.ReadLog("log-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	data := make([]byte, 100)
	n, _ := rc.Read(data)
	if string(data[:n]) != "terminal output" {
		t.Errorf("got log %q, want 'terminal output'", string(data[:n]))
	}
}

func TestNewStoreCreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")
	_, err := NewStore(dir, false, 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions")); err != nil {
		t.Error("sessions dir not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "logs")); err != nil {
		t.Error("logs dir not created")
	}
}
