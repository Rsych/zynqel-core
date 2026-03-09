package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/server"
	"github.com/Rsych/zynqel-core/internal/session"
)

func main() {
	port := os.Getenv("ZYNQEL_PORT")
	if port == "" {
		port = "8080"
	}

	// Connect to Docker daemon.
	sb, err := sandbox.NewDockerSandbox()
	if err != nil {
		log.Fatalf("failed to connect to docker: %v", err)
	}
	defer sb.Close()

	sm := session.NewManager(sb)
	srv := server.New(sm)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("zynqel-core starting on %s", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
