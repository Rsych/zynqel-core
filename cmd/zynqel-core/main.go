package main

import (
	"context"
	"fmt"
	log "github.com/Rsych/zynqel-core/internal/logger"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/server"
	"github.com/Rsych/zynqel-core/internal/session"
	"github.com/Rsych/zynqel-core/internal/sessionlog"
)

func main() {
	log.Init(os.Getenv("ZYNQEL_LOG_FORMAT"), os.Getenv("ZYNQEL_LOG_LEVEL"))

	port := os.Getenv("ZYNQEL_PORT")
	if port == "" {
		port = "8080"
	}

	// Load resource policy from environment.
	rp, err := policy.PolicyFromEnv()
	if err != nil {
		log.Fatalf("invalid resource policy: %v", err)
	}
	log.Printf("resource policy: memory=%dMB cpu=%d%% idle_timeout=%ds hard_timeout=%ds max_sessions=%d",
		rp.MemoryMB, rp.CPUQuota, rp.IdleTimeoutSec, rp.HardTimeoutSec, rp.MaxSessions)

	// Connect to Docker daemon.
	sb, err := sandbox.NewDockerSandbox()
	if err != nil {
		log.Fatalf("failed to connect to docker: %v", err)
	}
	defer func() { _ = sb.Close() }()

	// Sweep orphan containers from previous runs.
	sweepCtx, sweepCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sweepCancel()
	n, err := sb.Sweep(sweepCtx)
	if err != nil {
		log.Printf("warning: orphan sweep failed: %v", err)
	} else if n > 0 {
		log.Printf("orphan sweep: removed %d stale container(s)", n)
	}

	// Load custom agent configs.
	dataDir := os.Getenv("ZYNQEL_DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("cannot determine home directory: %v (set ZYNQEL_DATA_DIR to override)", err)
		}
		dataDir = filepath.Join(home, ".zynqel")
	}
	agentStore := agentcfg.NewStore(filepath.Join(dataDir, "agents.json"))
	if err := agentStore.Load(); err != nil {
		log.Printf("warning: failed to load agent configs: %v", err)
	}

	// Create session log store for persisting session history.
	logPTY := strings.EqualFold(os.Getenv("ZYNQEL_LOG_PTY"), "true")
	retentionDays := 30
	if v := os.Getenv("ZYNQEL_LOG_RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			retentionDays = d
		}
	}
	logStore, err := sessionlog.NewStore(dataDir, logPTY, retentionDays)
	if err != nil {
		log.Fatalf("failed to create session log store: %v", err)
	}
	if removed, err := logStore.Cleanup(); err != nil {
		log.Printf("warning: session log cleanup: %v", err)
	} else if removed > 0 {
		log.Printf("session log cleanup: removed %d old record(s)", removed)
	}

	sm := session.NewManager(sb, rp, agentStore, logStore)

	// Serve dashboard from web/out/ (Next.js static export).
	var webFS fs.FS
	if info, err := os.Stat("web/out"); err == nil && info.IsDir() {
		webFS = os.DirFS("web/out")
		log.Println("serving dashboard from web/out/")
	}
	srv := server.New(sm, agentStore, sb, logStore, webFS)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           srv,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Start idle session checker.
	idleCtx, idleCancel := context.WithCancel(context.Background())
	defer idleCancel()
	sm.StartTimeoutChecker(idleCtx)

	// Start HTTP server in a goroutine.
	go func() {
		log.Printf("zynqel-core starting on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("received %v, shutting down...", sig)

	// Phase 1: Stop accepting new HTTP connections, finish in-flight requests.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("warning: http shutdown: %v", err)
	}

	// Phase 2: Clean up all sessions (stop + remove containers).
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	sm.Shutdown(cleanupCtx)

	log.Println("shutdown complete")
}
