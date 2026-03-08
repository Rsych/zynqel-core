package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Rsych/zynqel-core/internal/server"
)

func main() {
	port := os.Getenv("ZYNQEL_PORT")
	if port == "" {
		port = "8080"
	}

	srv := server.New()

	addr := fmt.Sprintf(":%s", port)
	log.Printf("zynqel-core starting on %s", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
