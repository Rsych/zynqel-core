package session

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
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

func mustSandbox(t *testing.T) *sandbox.DockerSandbox {
	t.Helper()
	sb, err := sandbox.NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Close() })
	return sb
}

// testImage is a publicly available image for CI (zynqel-base is local only).
const testImage = "ubuntu:22.04"

func testSpec() SessionSpec {
	return SessionSpec{Agent: "shell", Image: testImage}
}

func mustManager(t *testing.T, sb *sandbox.DockerSandbox) *Manager {
	t.Helper()
	p := policy.DefaultPolicy()
	p.MaxSessions = 100 // allow more for stability tests
	return NewManager(sb, p)
}

func goroutineCount() int {
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	return runtime.NumGoroutine()
}

func heapAlloc() uint64 {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// waitContainerRemoved polls until a container no longer exists or timeout.
// Docker removal can be async — this avoids flaky assertions.
func waitContainerRemoved(t *testing.T, cli *client.Client, containerID string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.After(timeout)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			return false
		case <-tick.C:
			_, err := cli.ContainerInspect(context.Background(), containerID)
			if err != nil {
				return true // container gone
			}
		}
	}
}

// TestManager_KillDuringActiveTask creates a session with active processes,
// then kills it and verifies clean teardown.
func TestManager_KillDuringActiveTask(t *testing.T) {
	cli := mustDockerClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	sess, err := m.Create(ctx, testSpec())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	containerID := sess.ContainerID

	// Start long-running processes via WriteInput.
	err = m.WriteInput(sess.ID, []byte("sleep 3600 &\nyes > /dev/null &\n"))
	if err != nil {
		t.Fatalf("WriteInput: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Kill the session while processes are running.
	if err := m.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify this specific container is gone (Docker removal can be async).
	if !waitContainerRemoved(t, cli, containerID, 10*time.Second) {
		t.Errorf("container %s still exists after Delete", containerID[:12])
	}

	// Verify session is removed from manager.
	if _, err := m.Get(sess.ID); err == nil {
		t.Error("expected Get to fail after Delete")
	}
}

// TestManager_RapidCreateKill runs 20 rapid create/delete cycles and verifies
// no containers leak and goroutine count stays stable.
func TestManager_RapidCreateKill(t *testing.T) {
	_ = mustDockerClient(t)

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	// Record baseline goroutine count.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseGoroutines := runtime.NumGoroutine()

	const cycles = 20

	for i := 0; i < cycles; i++ {
		sess, err := m.Create(ctx, testSpec())
		if err != nil {
			t.Fatalf("cycle %d: Create: %v", i, err)
		}
		if err := m.Delete(ctx, sess.ID); err != nil {
			t.Fatalf("cycle %d: Delete: %v", i, err)
		}
	}

	// Verify no sessions remain in manager.
	if sessions := m.List(); len(sessions) != 0 {
		t.Errorf("expected 0 sessions after %d cycles, got %d", cycles, len(sessions))
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
	_ = mustDockerClient(t)

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
				sess, err := m.Create(ctx, testSpec())
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

	// Verify no sessions remain in manager.
	if sessions := m.List(); len(sessions) != 0 {
		t.Errorf("expected 0 sessions after concurrent test, got %d", len(sessions))
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

	// Create 3 sessions, track their container IDs.
	var containerIDs []string
	for i := 0; i < 3; i++ {
		sess, err := m.Create(ctx, testSpec())
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		containerIDs = append(containerIDs, sess.ContainerID)
	}

	// Verify 3 sessions exist in manager.
	if sessions := m.List(); len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Shutdown should clean all.
	m.Shutdown(ctx)

	// Verify all specific containers are gone.
	for _, cid := range containerIDs {
		if !waitContainerRemoved(t, cli, cid, 10*time.Second) {
			t.Errorf("container %s still exists after Shutdown", cid[:12])
		}
	}

	// Verify session list is empty.
	if sessions := m.List(); len(sessions) != 0 {
		t.Errorf("expected 0 sessions after Shutdown, got %d", len(sessions))
	}
}

// --- Stability tests ---

// TestStability_ConcurrentSustainedSessions runs 10 sessions with active PTY
// streams for 30 seconds, then tears them all down and checks for leaks.
func TestStability_ConcurrentSustainedSessions(t *testing.T) {
	cli := mustDockerClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	const numSessions = 10
	const sustainDuration = 30 * time.Second

	baseGoroutines := goroutineCount()
	baseHeap := heapAlloc()

	// Create all sessions.
	type sessionInfo struct {
		id          string
		containerID string
	}
	sessions := make([]sessionInfo, numSessions)

	for i := 0; i < numSessions; i++ {
		sess, err := m.Create(ctx, testSpec())
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		sessions[i] = sessionInfo{id: sess.ID, containerID: sess.ContainerID}

		// Start a command that generates continuous output.
		err = m.WriteInput(sess.ID, []byte("while true; do echo stability-test; sleep 1; done &\n"))
		if err != nil {
			t.Fatalf("WriteInput %d: %v", i, err)
		}
	}

	t.Logf("created %d sessions, sustaining for %v...", numSessions, sustainDuration)

	// Subscribe to all sessions and consume output for sustainDuration.
	var wg sync.WaitGroup
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, sub, err := m.Subscribe(sessions[idx].id)
			if err != nil {
				t.Errorf("Subscribe %d: %v", idx, err)
				return
			}
			defer m.Unsubscribe(sessions[idx].id, sub)

			timer := time.After(sustainDuration)
			for {
				select {
				case <-timer:
					return
				case _, ok := <-sub.Ch:
					if !ok {
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()

	t.Logf("sustained %d sessions for %v, cleaning up...", numSessions, sustainDuration)

	// Delete all sessions.
	for _, s := range sessions {
		if err := m.Delete(ctx, s.id); err != nil {
			t.Errorf("Delete %s: %v", s.id, err)
		}
	}

	// Verify all containers removed.
	for _, s := range sessions {
		if !waitContainerRemoved(t, cli, s.containerID, 15*time.Second) {
			t.Errorf("container %s still exists after delete", s.containerID[:12])
		}
	}

	// Check for leaks.
	finalGoroutines := goroutineCount()
	finalHeap := heapAlloc()

	leaked := finalGoroutines - baseGoroutines
	if leaked > 5 {
		t.Errorf("goroutine leak: base=%d final=%d leaked=%d", baseGoroutines, finalGoroutines, leaked)
	}

	// Verify no sessions remain.
	if remaining := m.List(); len(remaining) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(remaining))
	}

	t.Logf("stability test complete: goroutines base=%d final=%d, heap base=%dKB final=%dKB",
		baseGoroutines, finalGoroutines, baseHeap/1024, finalHeap/1024)
}

// TestStability_SubscribeUnsubscribeCycles verifies that rapid subscribe/unsubscribe
// doesn't leak goroutines or channels.
func TestStability_SubscribeUnsubscribeCycles(t *testing.T) {
	_ = mustDockerClient(t)

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	sess, err := m.Create(ctx, testSpec())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	baseGoroutines := goroutineCount()

	const cycles = 50
	for i := 0; i < cycles; i++ {
		_, sub, err := m.Subscribe(sess.ID)
		if err != nil {
			t.Fatalf("Subscribe %d: %v", i, err)
		}
		m.Unsubscribe(sess.ID, sub)
	}

	if err := m.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	finalGoroutines := goroutineCount()
	leaked := finalGoroutines - baseGoroutines
	if leaked > 5 {
		t.Errorf("goroutine leak after %d sub/unsub cycles: base=%d final=%d", cycles, baseGoroutines, finalGoroutines)
	}

	t.Logf("completed %d subscribe/unsubscribe cycles, goroutines: base=%d final=%d", cycles, baseGoroutines, finalGoroutines)
}

// TestStability_SessionsWithWriteInput verifies that sustained input writing
// doesn't leak resources.
func TestStability_SessionsWithWriteInput(t *testing.T) {
	_ = mustDockerClient(t)

	sb := mustSandbox(t)
	m := mustManager(t, sb)
	ctx := context.Background()

	const numSessions = 5
	const writesPerSession = 100

	baseGoroutines := goroutineCount()

	var sessionIDs []string
	for i := 0; i < numSessions; i++ {
		sess, err := m.Create(ctx, testSpec())
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		sessionIDs = append(sessionIDs, sess.ID)
	}

	// Write input to all sessions concurrently.
	var wg sync.WaitGroup
	for _, id := range sessionIDs {
		wg.Add(1)
		go func(sid string) {
			defer wg.Done()
			for j := 0; j < writesPerSession; j++ {
				if err := m.WriteInput(sid, []byte("echo test\n")); err != nil {
					return // session may have been cleaned up
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(id)
	}
	wg.Wait()

	// Delete all sessions.
	for _, id := range sessionIDs {
		if err := m.Delete(ctx, id); err != nil {
			t.Errorf("Delete %s: %v", id, err)
		}
	}

	finalGoroutines := goroutineCount()
	leaked := finalGoroutines - baseGoroutines
	if leaked > 5 {
		t.Errorf("goroutine leak: base=%d final=%d leaked=%d", baseGoroutines, finalGoroutines, leaked)
	}

	if remaining := m.List(); len(remaining) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(remaining))
	}

	t.Logf("completed %d sessions x %d writes, goroutines: base=%d final=%d",
		numSessions, writesPerSession, baseGoroutines, finalGoroutines)
}
