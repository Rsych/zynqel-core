package server

import (
	"encoding/json"
	"log"
	"net/http"
)

type Server struct {
	router *http.ServeMux
}

func New() *Server {
	s := &Server{
		router: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.HandleFunc("GET /health", s.handleHealth)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]string{
		"status": "healthy",
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("error encoding health response: %v", err)
	}
}
