package server

import (
	"io/fs"
	"net/http"

	"github.com/Rsych/zynqel-core/internal/session"
)

type Server struct {
	router   *http.ServeMux
	sessions *session.Manager
}

// New takes a session.Manager and a filesystem for static web assets.
// Pass nil for webFS to disable the dev console.
func New(sm *session.Manager, webFS fs.FS) *Server {
	s := &Server{
		router:   http.NewServeMux(),
		sessions: sm,
	}
	s.routes(webFS)
	return s
}

func (s *Server) routes(webFS fs.FS) {
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("POST /sessions", s.handleCreateSession)
	s.router.HandleFunc("GET /sessions", s.handleListSessions)
	s.router.HandleFunc("GET /sessions/{id}", s.handleGetSession)
	s.router.HandleFunc("DELETE /sessions/{id}", s.handleDeleteSession)
	s.router.HandleFunc("GET /sessions/{id}/stream", s.handleSessionStream)

	if webFS != nil {
		s.router.Handle("GET /console/", http.StripPrefix("/console/", http.FileServer(http.FS(webFS))))
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
