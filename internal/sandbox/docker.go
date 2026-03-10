package sandbox

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type DockerSandbox struct {
	cli *client.Client
}

func NewDockerSandbox() (*DockerSandbox, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &DockerSandbox{cli: cli}, nil
}

func (d *DockerSandbox) ensureImage(ctx context.Context, img string) error {
	_, _, err := d.cli.ImageInspectWithRaw(ctx, img)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return fmt.Errorf("inspect image %s: %w", img, err)
	}

	log.Printf("pulling image %s ...", img)
	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", img, err)
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	log.Printf("pulled image %s", img)
	return nil
}

func (d *DockerSandbox) Create(ctx context.Context, spec Spec) (string, error) {
	if err := d.ensureImage(ctx, spec.Image); err != nil {
		return "", err
	}

	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}

	config := &container.Config{
		Image:     spec.Image,
		Env:       env,
		Labels:    spec.Labels,
		Tty:       true,
		OpenStdin: true,
		Cmd:       []string{"/bin/sh"},
	}

	hostConfig := &container.HostConfig{}

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	return resp.ID, nil
}

func (d *DockerSandbox) Start(ctx context.Context, id string) error {
	if err := d.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", id[:12], err)
	}
	return nil
}

func (d *DockerSandbox) Stop(ctx context.Context, id string) error {
	timeout := 10
	stopOpts := container.StopOptions{Timeout: &timeout}
	if err := d.cli.ContainerStop(ctx, id, stopOpts); err != nil {
		return fmt.Errorf("stop container %s: %w", id[:12], err)
	}
	return nil
}

func (d *DockerSandbox) Remove(ctx context.Context, id string) error {
	opts := container.RemoveOptions{Force: true}
	if err := d.cli.ContainerRemove(ctx, id, opts); err != nil {
		return fmt.Errorf("remove container %s: %w", id[:12], err)
	}
	return nil
}

func (d *DockerSandbox) Close() error {
	return d.cli.Close()
}
