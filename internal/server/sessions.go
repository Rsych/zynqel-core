package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/Rsych/zynqel-core/internal/session"
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

	sess, err := s.sessions.Create(r.Context(), spec)
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

	sess, err := s.sessions.Restart(r.Context(), id)
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
