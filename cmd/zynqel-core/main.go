package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Rsych/zynqel-core/internal/server"
	"github.com/Rsych/zynqel-core/internal/session"
)

func main() {
	port := os.Getenv("ZYNQEL_PORT")
	if port == "" {
		port = "8080"
	}

	sm := session.NewManager()
	srv := server.New(sm)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("zynqel-core starting on %s", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
