package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
)

// agentInfo is the API response for a single agent.
type agentInfo struct {
	Name       string            `json:"name"`
	Builtin    bool              `json:"builtin"`
	Command    []string          `json:"command,omitempty"`
	Image      string            `json:"image,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

// handleListAgents returns all agents (built-in + custom).
func (s *Server) handleListAgents(w http.ResponseWriter, _ *http.Request) {
	agents := []agentInfo{
		{Name: "shell", Builtin: true},
		{Name: "claude", Builtin: true, Image: "zynqel-claude:latest"},
	}

	for _, cfg := range s.agents.List() {
		agents = append(agents, agentInfo{
			Name:       cfg.Name,
			Builtin:    false,
			Command:    cfg.Command,
			Image:      cfg.Image,
			Dockerfile: cfg.Dockerfile,
			Env:        cfg.Env,
		})
	}

	writeJSON(w, http.StatusOK, agents)
}

// handleCreateAgent creates a new custom agent config.
// If a Dockerfile is provided, builds the image automatically.
func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var cfg agentcfg.AgentConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Build custom image from Dockerfile if provided.
	if cfg.Dockerfile != "" {
		imageName := fmt.Sprintf("zynqel-agent-%s:latest", cfg.Name)
		if err := s.sandbox.BuildImage(r.Context(), cfg.Dockerfile, imageName); err != nil {
			log.Printf("error building image for agent %q: %v", cfg.Name, err)
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to build image: %v", err))
			return
		}
		cfg.Image = imageName
	}

	if err := s.agents.Put(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, _ := s.agents.Get(cfg.Name)
	log.Printf("created custom agent %q", cfg.Name)
	writeJSON(w, http.StatusCreated, saved)
}

// handleUpdateAgent updates an existing custom agent config.
func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var cfg agentcfg.AgentConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg.Name = name

	// Rebuild image if Dockerfile changed.
	if cfg.Dockerfile != "" {
		imageName := fmt.Sprintf("zynqel-agent-%s:latest", name)
		if err := s.sandbox.BuildImage(r.Context(), cfg.Dockerfile, imageName); err != nil {
			log.Printf("error building image for agent %q: %v", name, err)
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to build image: %v", err))
			return
		}
		cfg.Image = imageName
	}

	if err := s.agents.Put(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, _ := s.agents.Get(name)
	log.Printf("updated custom agent %q", name)
	writeJSON(w, http.StatusOK, saved)
}

// handleDeleteAgent removes a custom agent config.
func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if err := s.agents.Delete(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("deleted custom agent %q", name)
	w.WriteHeader(http.StatusNoContent)
}
