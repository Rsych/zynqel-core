package sandbox

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestDockerSandbox_FullLifecycle(t *testing.T) {
	cli := mustDockerClient(t)
	defer cli.Close()

	sb, err := NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	defer sb.Close()

	ctx := context.Background()

	spec := Spec{
		Image: "ubuntu:22.04",
		Env:   map[string]string{"TEST_VAR": "hello"},
		Labels: map[string]string{
			"zynqel.session-id": "test-session-001",
		},
	}

	containerID, err := sb.Create(ctx, spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Logf("created container: %s", containerID[:12])
	defer func() { _ = sb.Remove(ctx, containerID) }()

	// Verify container exists but not started
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("container should exist after Create: %v", err)
	}
	if info.State.Running {
		t.Fatal("container should not be running before Start")
	}
	if info.Config.Labels[LabelManaged] != "true" {
		t.Error("expected zynqel.managed=true label (auto-injected by Create)")
	}
	if info.Config.Labels["zynqel.session-id"] != "test-session-001" {
		t.Error("expected zynqel.session-id label")
	}

	found := false
	for _, e := range info.Config.Env {
		if e == "TEST_VAR=hello" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected TEST_VAR=hello in container env")
	}

	// Start
	if err := sb.Start(ctx, containerID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	info, err = cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("inspect after start: %v", err)
	}
	if !info.State.Running {
		t.Fatal("container should be running after Start")
	}

	// Stop
	if err := sb.Stop(ctx, containerID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	info, err = cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("inspect after stop: %v", err)
	}
	if info.State.Running {
		t.Fatal("container should not be running after Stop")
	}

	// Remove
	if err := sb.Remove(ctx, containerID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err = cli.ContainerInspect(ctx, containerID)
	if err == nil {
		t.Fatal("container should not exist after Remove")
	}
}

func TestDockerSandbox_RemoveForceKillsRunning(t *testing.T) {
	cli := mustDockerClient(t)
	defer cli.Close()

	sb, err := NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	defer sb.Close()

	ctx := context.Background()

	spec := Spec{
		Image: "ubuntu:22.04",
		Labels: map[string]string{
			"zynqel.session-id": "test-force-kill",
		},
	}

	containerID, err := sb.Create(ctx, spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = sb.Remove(ctx, containerID) }()

	if err := sb.Start(ctx, containerID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := sb.Remove(ctx, containerID); err != nil {
		t.Fatalf("Remove (force): %v", err)
	}

	_, err = cli.ContainerInspect(ctx, containerID)
	if err == nil {
		t.Fatal("container should not exist after force Remove")
	}
}

func TestDockerSandbox_ResourceLimits(t *testing.T) {
	cli := mustDockerClient(t)
	defer cli.Close()

	sb, err := NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	defer sb.Close()

	ctx := context.Background()

	spec := Spec{
		Image: "ubuntu:22.04",
		Labels: map[string]string{
			"zynqel.session-id": "test-limits",
		},
		MemoryBytes: 256 * 1024 * 1024, // 256 MB
		NanoCPUs:    5e8,               // 0.5 cores
	}

	containerID, err := sb.Create(ctx, spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = sb.Remove(ctx, containerID) }()

	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if info.HostConfig.Memory != 256*1024*1024 {
		t.Errorf("Memory = %d, want %d", info.HostConfig.Memory, 256*1024*1024)
	}
	if info.HostConfig.NanoCPUs != 5e8 {
		t.Errorf("NanoCPUs = %d, want %d", info.HostConfig.NanoCPUs, int64(5e8))
	}
}

func TestDockerSandbox_Sweep(t *testing.T) {
	cli := mustDockerClient(t)
	defer cli.Close()

	sb, err := NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	defer sb.Close()

	ctx := context.Background()

	// Create two orphan containers.
	var ids []string
	for i := 0; i < 2; i++ {
		spec := Spec{
			Image: "ubuntu:22.04",
			Labels: map[string]string{
				"zynqel.managed":    "true",
				"zynqel.session-id": fmt.Sprintf("orphan-%d", i),
			},
		}
		id, err := sb.Create(ctx, spec)
		if err != nil {
			t.Fatalf("Create orphan %d: %v", i, err)
		}
		ids = append(ids, id)
	}

	// Start one so sweep handles both running and stopped containers.
	if err := sb.Start(ctx, ids[0]); err != nil {
		t.Fatalf("Start orphan: %v", err)
	}

	// Sweep should remove both.
	n, err := sb.Sweep(ctx)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n != 2 {
		t.Errorf("Sweep removed %d containers, want 2", n)
	}

	// Verify they're gone.
	for _, id := range ids {
		_, err := cli.ContainerInspect(ctx, id)
		if err == nil {
			t.Errorf("container %s should not exist after Sweep", id[:12])
		}
	}
}

func TestDockerSandbox_CreateBadImage(t *testing.T) {
	_ = mustDockerClient(t)

	sb, err := NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	defer sb.Close()

	spec := Spec{
		Image: "this-image-does-not-exist:never",
	}

	_, err = sb.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("Create should fail with nonexistent image")
	}
	t.Logf("expected error: %v", err)
}
