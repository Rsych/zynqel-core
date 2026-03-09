package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

// These are integration tests — they require a running Docker daemon.
// Skip gracefully if Docker isn't available.

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

	// 1. Create
	spec := Spec{
		Image: "ubuntu:22.04",
		Env:   map[string]string{"TEST_VAR": "hello"},
		Labels: map[string]string{
			"zynqel.managed":    "true",
			"zynqel.session-id": "test-session-001",
		},
	}

	containerID, err := sb.Create(spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Logf("created container: %s", containerID[:12])

	// Ensure cleanup even if test fails midway.
	defer func() {
		_ = sb.Remove(containerID)
	}()

	// 2. Verify container exists (created but not started)
	ctx := context.Background()
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("container should exist after Create: %v", err)
	}
	if info.State.Running {
		t.Fatal("container should not be running before Start")
	}

	// Verify labels
	if info.Config.Labels["zynqel.managed"] != "true" {
		t.Error("expected zynqel.managed=true label")
	}
	if info.Config.Labels["zynqel.session-id"] != "test-session-001" {
		t.Error("expected zynqel.session-id label")
	}

	// Verify env
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

	// 3. Start
	if err := sb.Start(containerID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Log("started container")

	info, err = cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("inspect after start: %v", err)
	}
	if !info.State.Running {
		t.Fatal("container should be running after Start")
	}

	// 4. Stop
	if err := sb.Stop(containerID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	t.Log("stopped container")

	info, err = cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("inspect after stop: %v", err)
	}
	if info.State.Running {
		t.Fatal("container should not be running after Stop")
	}

	// 5. Remove
	if err := sb.Remove(containerID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	t.Log("removed container")

	// Verify container is gone
	_, err = cli.ContainerInspect(ctx, containerID)
	if err == nil {
		t.Fatal("container should not exist after Remove")
	}
}

func TestDockerSandbox_RemoveForceKillsRunning(t *testing.T) {
	_ = mustDockerClient(t)

	sb, err := NewDockerSandbox()
	if err != nil {
		t.Fatalf("NewDockerSandbox: %v", err)
	}
	defer sb.Close()

	spec := Spec{
		Image: "ubuntu:22.04",
		Labels: map[string]string{
			"zynqel.managed":    "true",
			"zynqel.session-id": "test-force-kill",
		},
	}

	containerID, err := sb.Create(spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := sb.Start(containerID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Remove while running — Force=true should handle it
	if err := sb.Remove(containerID); err != nil {
		t.Fatalf("Remove (force): %v", err)
	}
	t.Log("force-removed running container")

	// Verify gone
	ctx := context.Background()
	cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	defer cli.Close()
	_, err = cli.ContainerInspect(ctx, containerID)
	if err == nil {
		t.Fatal("container should not exist after force Remove")
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

	_, err = sb.Create(spec)
	if err == nil {
		t.Fatal("Create should fail with nonexistent image")
	}
	t.Logf("expected error: %v", err)
}
