package server

import (
	"net/http"

	"github.com/Rsych/zynqel-core/internal/session"
)

type Server struct {
	router   *http.ServeMux
	sessions *session.Manager
}

// New takes a session.Manager as a dependency.
// This is dependency injection — the server doesn't create its
// own manager, it receives one. Makes testing easy: pass a
// real manager in prod, or a fresh one in each test.
func New(sm *session.Manager) *Server {
	s := &Server{
		router:   http.NewServeMux(),
		sessions: sm,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("POST /sessions", s.handleCreateSession)
	s.router.HandleFunc("GET /sessions", s.handleListSessions)
	s.router.HandleFunc("GET /sessions/{id}", s.handleGetSession)
	s.router.HandleFunc("DELETE /sessions/{id}", s.handleDeleteSession)
	s.router.HandleFunc("GET /sessions/{id}/stream", s.handleSessionStream)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
