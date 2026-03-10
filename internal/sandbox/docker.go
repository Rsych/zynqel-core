package sandbox

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// DockerSandbox implements Sandbox using the Docker Engine API.
//
// Why client.NewClientWithOpts? The Docker SDK talks to the
// Docker daemon via its REST API (usually over a Unix socket
// at /var/run/docker.sock). client.FromEnv() picks up config
// from DOCKER_HOST, DOCKER_TLS_VERIFY, etc.
type DockerSandbox struct {
	cli *client.Client
}

// NewDockerSandbox creates a DockerSandbox connected to the local Docker daemon.
func NewDockerSandbox() (*DockerSandbox, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &DockerSandbox{cli: cli}, nil
}

// ensureImage pulls the image if it's not available locally.
// Docker won't auto-pull on ContainerCreate — you have to
// pull explicitly. We check first to avoid pulling every time.
func (d *DockerSandbox) ensureImage(ctx context.Context, img string) error {
	_, _, err := d.cli.ImageInspectWithRaw(ctx, img)
	if err == nil {
		return nil // already have it
	}
	if !client.IsErrNotFound(err) {
		return fmt.Errorf("inspect image %s: %w", img, err)
	}

	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", img, err)
	}
	defer reader.Close()

	// Must drain the reader or the pull won't complete.
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// Create pulls the image if needed, then creates a container.
func (d *DockerSandbox) Create(spec Spec) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := d.ensureImage(ctx, spec.Image); err != nil {
		return "", err
	}

	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}

	config := &container.Config{
		Image:      spec.Image,
		Env:        env,
		Labels:     spec.Labels,
		Tty:       true,              // Required for interactive CLI agents (PTY streaming)
		OpenStdin: true,              // Keep stdin open for agent input
		Cmd:       []string{"/bin/sh"}, // Keep-alive default; AgentAdapter overrides this (ZYNQ-08)
	}

	// HostConfig is where resource limits go (ZYNQ-04).
	hostConfig := &container.HostConfig{}

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	return resp.ID, nil
}

// Start starts a previously created container.
func (d *DockerSandbox) Start(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := d.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", id[:12], err)
	}
	return nil
}

// Stop sends SIGTERM to the container, waits for graceful
// shutdown, then SIGKILL after timeout. This is Docker's
// default behavior — the timeout parameter controls how long
// to wait before force-killing.
func (d *DockerSandbox) Stop(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	timeout := 10 // seconds to wait before SIGKILL
	stopOpts := container.StopOptions{Timeout: &timeout}
	if err := d.cli.ContainerStop(ctx, id, stopOpts); err != nil {
		return fmt.Errorf("stop container %s: %w", id[:12], err)
	}
	return nil
}

// Remove deletes a container. Force=true removes it even if
// it's still running (stop + remove in one call).
func (d *DockerSandbox) Remove(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := container.RemoveOptions{Force: true}
	if err := d.cli.ContainerRemove(ctx, id, opts); err != nil {
		return fmt.Errorf("remove container %s: %w", id[:12], err)
	}
	return nil
}

// Close closes the Docker client connection.
func (d *DockerSandbox) Close() error {
	return d.cli.Close()
}
