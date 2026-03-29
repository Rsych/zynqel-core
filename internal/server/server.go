package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/session"
)

type Server struct {
	router   *http.ServeMux
	sessions *session.Manager
	agents   *agentcfg.Store
	sandbox  sandbox.Sandbox
}

// New takes a session.Manager, agent config store, sandbox, and a filesystem for static web assets.
// Pass nil for webFS to disable the dashboard.
func New(sm *session.Manager, agents *agentcfg.Store, sb sandbox.Sandbox, webFS fs.FS) *Server {
	s := &Server{
		router:   http.NewServeMux(),
		sessions: sm,
		agents:   agents,
		sandbox:  sb,
	}
	s.routes(webFS)
	return s
}

func (s *Server) routes(webFS fs.FS) {
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("POST /sessions", s.handleCreateSession)
	s.router.HandleFunc("GET /sessions", s.handleListSessions)
	s.router.HandleFunc("GET /sessions/{id}", s.handleGetSession)
	s.router.HandleFunc("POST /sessions/{id}/stop", s.handleStopSession)
	s.router.HandleFunc("POST /sessions/{id}/restart", s.handleRestartSession)
	s.router.HandleFunc("DELETE /sessions/{id}", s.handleDeleteSession)
	s.router.HandleFunc("GET /sessions/{id}/stats", s.handleSessionStats)
	s.router.HandleFunc("GET /sessions/{id}/stream", s.handleSessionStream)
	s.router.HandleFunc("GET /agents", s.handleListAgents)
	s.router.HandleFunc("POST /agents", s.handleCreateAgent)
	s.router.HandleFunc("PUT /agents/{name}", s.handleUpdateAgent)
	s.router.HandleFunc("DELETE /agents/{name}", s.handleDeleteAgent)
	s.router.HandleFunc("GET /system/info", s.handleSystemInfo)
	s.router.HandleFunc("GET /workspaces", s.handleListWorkspaces)
	s.router.HandleFunc("DELETE /workspaces/{id}", s.handleDeleteWorkspace)

	if webFS != nil {
		fileServer := http.FileServer(http.FS(webFS))
		s.router.HandleFunc("GET /console/", func(w http.ResponseWriter, r *http.Request) {
			// Strip /console/ prefix for file lookup.
			path := strings.TrimPrefix(r.URL.Path, "/console/")

			// Try exact file, then path.html (Next.js static export).
			if path != "" && !strings.Contains(path, ".") {
				if _, err := fs.Stat(webFS, path+".html"); err == nil {
					r.URL.Path = "/console/" + path + ".html"
				}
			}
			http.StripPrefix("/console/", fileServer).ServeHTTP(w, r)
		})
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS: allow cross-origin requests from localhost dev servers (e.g. Next.js on :3000).
	// In production, Go serves the static export so all requests are same-origin.
	origin := r.Header.Get("Origin")
	if origin != "" && strings.HasPrefix(origin, "http://localhost") {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
