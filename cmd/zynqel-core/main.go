package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/server"
	"github.com/Rsych/zynqel-core/internal/session"
)

func main() {
	port := os.Getenv("ZYNQEL_PORT")
	if port == "" {
		port = "8080"
	}

	// Load resource policy from environment.
	rp, err := policy.PolicyFromEnv()
	if err != nil {
		log.Fatalf("invalid resource policy: %v", err)
	}
	log.Printf("resource policy: memory=%dMB cpu=%d%%", rp.MemoryMB, rp.CPUQuota)

	// Connect to Docker daemon.
	sb, err := sandbox.NewDockerSandbox()
	if err != nil {
		log.Fatalf("failed to connect to docker: %v", err)
	}
	defer func() { _ = sb.Close() }()

	// Sweep orphan containers from previous runs.
	sweepCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n, err := sb.Sweep(sweepCtx)
	if err != nil {
		log.Printf("warning: orphan sweep failed: %v", err)
	} else if n > 0 {
		log.Printf("orphan sweep: removed %d stale container(s)", n)
	}

	sm := session.NewManager(sb, rp)

	// Serve web dev console from ./web directory if it exists.
	var webFS fs.FS
	if info, err := os.Stat("web"); err == nil && info.IsDir() {
		webFS = os.DirFS("web")
		log.Println("serving dev console from ./web")
	}
	srv := server.New(sm, webFS)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("zynqel-core starting on %s", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
