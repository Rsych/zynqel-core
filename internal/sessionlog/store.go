package sessionlog

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Record represents a completed session's metadata persisted to disk.
type Record struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	Agent       string    `json:"agent"`
	Image       string    `json:"image,omitempty"`
	RepoURL     string    `json:"repo_url,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	StoppedAt   time.Time `json:"stopped_at,omitempty"`
	Duration    string    `json:"duration,omitempty"`
	Error       string    `json:"error,omitempty"`
	HasLog      bool      `json:"has_log"`
}

// Store persists session records and optional PTY logs to disk.
type Store struct {
	dir           string // $ZYNQEL_DATA_DIR/sessions
	logDir        string // $ZYNQEL_DATA_DIR/logs
	logPTY        bool
	retentionDays int
}

// NewStore creates a Store. Directories are created if they don't exist.
func NewStore(dataDir string, logPTY bool, retentionDays int) (*Store, error) {
	dir := filepath.Join(dataDir, "sessions")
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return &Store{
		dir:           dir,
		logDir:        logDir,
		logPTY:        logPTY,
		retentionDays: retentionDays,
	}, nil
}

// LogPTY returns whether PTY logging is enabled.
func (s *Store) LogPTY() bool { return s.logPTY }

// Save writes a session record to disk.
func (s *Store) Save(r Record) error {
	// Compute duration if we have both timestamps.
	if !r.StoppedAt.IsZero() && !r.CreatedAt.IsZero() {
		r.Duration = r.StoppedAt.Sub(r.CreatedAt).Truncate(time.Second).String()
	}

	// Check if a PTY log exists for this session.
	if _, err := os.Stat(filepath.Join(s.logDir, r.ID+".log")); err == nil {
		r.HasLog = true
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session record: %w", err)
	}
	path := filepath.Join(s.dir, r.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write session record: %w", err)
	}
	return nil
}

// List returns all session records sorted by created_at descending.
func (s *Store) List() ([]Record, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}
	var records []Record
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := s.readRecord(filepath.Join(s.dir, e.Name()))
		if err != nil {
			log.Printf("warning: skip bad session record %s: %v", e.Name(), err)
			continue
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

// Get returns a single session record by ID.
func (s *Store) Get(id string) (*Record, error) {
	path := filepath.Join(s.dir, id+".json")
	r, err := s.readRecord(path)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Delete removes a session record and its log file.
func (s *Store) Delete(id string) error {
	_ = os.Remove(filepath.Join(s.dir, id+".json"))
	_ = os.Remove(filepath.Join(s.logDir, id+".log"))
	return nil
}

// OpenLogWriter opens a log file for writing PTY output.
// The caller is responsible for closing the writer.
func (s *Store) OpenLogWriter(id string) (io.WriteCloser, error) {
	path := filepath.Join(s.logDir, id+".log")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	return f, nil
}

// ReadLog opens a session's PTY log file for reading.
// Returns os.ErrNotExist if no log exists.
func (s *Store) ReadLog(id string) (io.ReadCloser, error) {
	path := filepath.Join(s.logDir, id+".log")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Cleanup removes records older than the retention period. Returns count removed.
func (s *Store) Cleanup() (int, error) {
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("read sessions dir: %w", err)
	}
	removed := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := s.readRecord(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		if r.CreatedAt.Before(cutoff) {
			id := strings.TrimSuffix(e.Name(), ".json")
			_ = s.Delete(id)
			removed++
		}
	}
	return removed, nil
}

func (s *Store) readRecord(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return Record{}, err
	}
	return r, nil
}
