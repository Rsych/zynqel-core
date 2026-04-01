package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Rsych/zynqel-core/internal/session"
	"github.com/Rsych/zynqel-core/internal/sessionlog"
)

// handleCreateSession decodes a SessionSpec from the request body
// and creates a new session. Returns 201 Created with the session.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var spec session.SessionSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if spec.Agent == "" {
		writeError(w, http.StatusBadRequest, "agent is required")
		return
	}

	// Use a long timeout — git clone + image pull can take minutes.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	sess, err := s.sessions.Create(ctx, spec)
	if err != nil {
		if errors.Is(err, session.ErrAtCapacity) {
			writeError(w, http.StatusTooManyRequests, "session capacity exceeded")
			return
		}
		log.Printf("error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sess)
}

// handleListSessions returns all sessions.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.sessions.List()
	writeJSON(w, http.StatusOK, sessions)
}

// handleGetSession returns a single session by ID.
// r.PathValue("id") extracts the {id} from the URL pattern —
// this is Go 1.22+'s built-in path parameter support.
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.sessions.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	writeJSON(w, http.StatusOK, sess)
}

// handleDeleteSession kills and removes a session.
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.sessions.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 — success, no body
}

// handleStopSession gracefully stops a running session without removing it.
func (s *Server) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.sessions.Stop(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, _ := s.sessions.Get(id)
	writeJSON(w, http.StatusOK, sess)
}

// handleRestartSession creates a new session from a stopped session's spec.
func (s *Server) handleRestartSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	sess, err := s.sessions.Restart(ctx, id)
	if err != nil {
		log.Printf("error restarting session %s: %v", id, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sess)
}

// handleListWorkspaces returns all saved workspace volumes.
func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.sessions.ListWorkspaces(r.Context())
	if err != nil {
		log.Printf("error listing workspaces: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}
	writeJSON(w, http.StatusOK, workspaces)
}

// handleDeleteWorkspace removes a saved workspace volume.
func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.sessions.DeleteWorkspace(r.Context(), id); err != nil {
		log.Printf("error deleting workspace %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to delete workspace")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRenameWorkspace renames a workspace volume and committed image.
func (s *Server) handleRenameWorkspace(w http.ResponseWriter, r *http.Request) {
	oldID := r.PathValue("id")
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := s.sessions.RenameWorkspace(r.Context(), oldID, body.ID); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "active session"), strings.Contains(msg, "already exists"):
			writeError(w, http.StatusConflict, msg)
		case strings.Contains(msg, "invalid workspace_id"):
			writeError(w, http.StatusBadRequest, msg)
		default:
			log.Printf("error renaming workspace %s → %s: %v", oldID, body.ID, err)
			writeError(w, http.StatusInternalServerError, "failed to rename workspace")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": body.ID})
}

// handleSessionStats returns container CPU/memory stats.
func (s *Server) handleSessionStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	stats, err := s.sessions.Stats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found or not running")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleSystemInfo returns system-level information.
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	p := s.sessions.Policy()
	writeJSON(w, http.StatusOK, map[string]any{
		"max_sessions": p.MaxSessions,
		"active_count": s.sessions.ActiveCount(),
		"memory_mb":    p.MemoryMB,
		"cpu_quota":    p.CPUQuota,
		"idle_timeout": p.IdleTimeoutSec,
		"hard_timeout": p.HardTimeoutSec,
	})
}

// handleListSessionHistory returns past session records.
func (s *Server) handleListSessionHistory(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	records, err := s.logStore.List()
	if err != nil {
		log.Printf("error listing session history: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list session history")
		return
	}

	// Filter by workspace_id if provided.
	if wsID := r.URL.Query().Get("workspace_id"); wsID != "" {
		var filtered []sessionlog.Record
		for _, rec := range records {
			if rec.WorkspaceID == wsID {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	writeJSON(w, http.StatusOK, records)
}

// handleGetSessionHistory returns a single session record.
func (s *Server) handleGetSessionHistory(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeError(w, http.StatusNotFound, "session history not found")
		return
	}
	id := r.PathValue("id")
	rec, err := s.logStore.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session history not found")
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// handleGetSessionLog streams a session's PTY log as plain text.
func (s *Server) handleGetSessionLog(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeError(w, http.StatusNotFound, "no log available")
		return
	}
	id := r.PathValue("id")
	rc, err := s.logStore.ReadLog(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "no log available")
		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := io.Copy(w, rc); err != nil {
		log.Printf("error streaming session log %s: %v", id, err)
	}
}

// handleDeleteSessionHistory removes a session record and its log.
func (s *Server) handleDeleteSessionHistory(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeError(w, http.StatusNotFound, "session history not found")
		return
	}
	id := r.PathValue("id")
	if err := s.logStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete session history")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON is a helper to send JSON responses.
// Having this in one place means consistent Content-Type headers
// and error handling everywhere.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

// writeError sends a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
