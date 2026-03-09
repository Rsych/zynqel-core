package server

import (
	"encoding/json"
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

	sess, err := s.sessions.Create(spec)
	if err != nil {
		log.Printf("error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
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

	if err := s.sessions.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 — success, no body
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
