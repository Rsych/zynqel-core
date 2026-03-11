package sandbox

import (
	"context"
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
			"zynqel.managed":    "true",
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
	if info.Config.Labels["zynqel.managed"] != "true" {
		t.Error("expected zynqel.managed=true label")
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
			"zynqel.managed":    "true",
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
