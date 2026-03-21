package session

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func mustDockerClient(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("skipping: docker not available: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		t.Skipf("skipping: docker daemon not responding: %v", err)
	}
	return cli
}

func mustManager(t *testing.T, sb *sandbox.DockerSandbox) *Manager {
	t.Helper()
	return NewManager(sb, policy.DefaultPolicy())
}

func zynqelContainerCount(t *testing.T, cli *client.Client) int {
	t.Helper()
	args := filters.NewArgs(filters.Arg("label", sandbox.LabelManaged+"=true"))
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	return len(containers)
}

func mustSandbox(t *testing.T) *sandbox.DockerSandbox {
	t.Helper()
	sb, err := sandbox.NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Close() })
	return sb
}

// TestManager_KillDuringActiveTask creates a session with active processes,
// then kills it and verifies clean teardown.
func TestManager_KillDuringActiveTask(t *testing.T) {
	cli := mustDockerClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	sess, err := m.Create(ctx, SessionSpec{Agent: "shell"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Attach and start long-running processes.
	ptyConn, err := m.Attach(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}

	_, err = ptyConn.Write([]byte("sleep 3600 &\nyes > /dev/null &\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Kill the session while processes are running.
	if err := m.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify container is gone.
	if count := zynqelContainerCount(t, cli); count != 0 {
		t.Errorf("expected 0 zynqel containers after delete, got %d", count)
	}

	// Verify session is removed from manager.
	if _, err := m.Get(sess.ID); err == nil {
		t.Error("expected Get to fail after Delete")
	}
}

// TestManager_RapidCreateKill runs 20 rapid create/delete cycles and verifies
// no containers leak and goroutine count stays stable.
func TestManager_RapidCreateKill(t *testing.T) {
	cli := mustDockerClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	// Record baseline goroutine count.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseGoroutines := runtime.NumGoroutine()

	const cycles = 20

	for i := 0; i < cycles; i++ {
		sess, err := m.Create(ctx, SessionSpec{Agent: "shell"})
		if err != nil {
			t.Fatalf("cycle %d: Create: %v", i, err)
		}
		if err := m.Delete(ctx, sess.ID); err != nil {
			t.Fatalf("cycle %d: Delete: %v", i, err)
		}
	}

	// Verify no containers remain.
	if count := zynqelContainerCount(t, cli); count != 0 {
		t.Errorf("expected 0 zynqel containers after %d cycles, got %d", cycles, count)
	}

	// Verify no significant goroutine leak.
	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - baseGoroutines
	if leaked > 5 {
		t.Errorf("goroutine leak: base=%d final=%d leaked=%d", baseGoroutines, finalGoroutines, leaked)
	}

	t.Logf("completed %d create/kill cycles, goroutines: base=%d final=%d", cycles, baseGoroutines, finalGoroutines)
}

// TestManager_ConcurrentCreateKill runs concurrent create/delete operations
// to verify thread safety.
func TestManager_ConcurrentCreateKill(t *testing.T) {
	cli := mustDockerClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	const workers = 5
	const cyclesPerWorker = 3
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < cyclesPerWorker; j++ {
				sess, err := m.Create(ctx, SessionSpec{Agent: "shell"})
				if err != nil {
					t.Errorf("worker %d cycle %d: Create: %v", worker, j, err)
					return
				}
				if err := m.Delete(ctx, sess.ID); err != nil {
					t.Errorf("worker %d cycle %d: Delete: %v", worker, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no containers remain.
	if count := zynqelContainerCount(t, cli); count != 0 {
		t.Errorf("expected 0 zynqel containers after concurrent test, got %d", count)
	}

	t.Logf("completed %d workers x %d cycles", workers, cyclesPerWorker)
}

// TestManager_ShutdownCleansAll creates multiple sessions, then calls Shutdown
// and verifies all containers are cleaned up.
func TestManager_ShutdownCleansAll(t *testing.T) {
	cli := mustDockerClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	// Create 3 sessions.
	for i := 0; i < 3; i++ {
		if _, err := m.Create(ctx, SessionSpec{Agent: "shell"}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	// Verify 3 containers exist.
	if count := zynqelContainerCount(t, cli); count != 3 {
		t.Fatalf("expected 3 zynqel containers, got %d", count)
	}

	// Shutdown should clean all.
	m.Shutdown(ctx)

	// Verify all gone.
	if count := zynqelContainerCount(t, cli); count != 0 {
		t.Errorf("expected 0 zynqel containers after Shutdown, got %d", count)
	}

	// Verify session list is empty.
	if sessions := m.List(); len(sessions) != 0 {
		t.Errorf("expected 0 sessions after Shutdown, got %d", len(sessions))
	}
}
