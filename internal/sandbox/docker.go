package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/Rsych/zynqel-core/internal/shortid"
	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
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
	_, err := d.cli.ImageInspect(ctx, img)
	if err == nil {
		return nil
	}
	if !errdefs.IsNotFound(err) {
		return fmt.Errorf("inspect image %s: %w", img, err)
	}

	log.Printf("pulling image %s ...", img)
	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", img, err)
	}
	defer func() { _ = reader.Close() }()
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("pull image %s: %w", img, err)
	}
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

	labels := make(map[string]string, len(spec.Labels)+1)
	for k, v := range spec.Labels {
		labels[k] = v
	}
	labels[LabelManaged] = "true"

	config := &container.Config{
		Image:     spec.Image,
		Env:       env,
		Labels:    labels,
		Tty:       true,
		OpenStdin: true,
		Cmd:       spec.Cmd, // nil = use image CMD
	}

	hostConfig := &container.HostConfig{}
	if spec.MemoryBytes > 0 || spec.NanoCPUs > 0 {
		hostConfig.Resources = container.Resources{
			Memory:   spec.MemoryBytes,
			NanoCPUs: spec.NanoCPUs,
		}
	}
	if spec.VolumeName != "" {
		// Ensure volume exists with labels (no-op if already exists).
		if _, err := d.cli.VolumeCreate(ctx, volume.CreateOptions{
			Name:   spec.VolumeName,
			Labels: spec.VolumeLabels,
		}); err != nil {
			return "", fmt.Errorf("create volume %s: %w", spec.VolumeName, err)
		}
		hostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: spec.VolumeName,
				Target: "/workspace",
			},
		}
	}

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container (image=%s): %w", spec.Image, err)
	}

	return resp.ID, nil
}

func (d *DockerSandbox) Start(ctx context.Context, id string) error {
	if err := d.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", shortid.Format(id), err)
	}
	return nil
}

func (d *DockerSandbox) Stop(ctx context.Context, id string) error {
	timeout := 10
	stopOpts := container.StopOptions{Timeout: &timeout}
	if err := d.cli.ContainerStop(ctx, id, stopOpts); err != nil {
		return fmt.Errorf("stop container %s: %w", shortid.Format(id), err)
	}
	return nil
}

func (d *DockerSandbox) Remove(ctx context.Context, id string) error {
	opts := container.RemoveOptions{Force: true}
	if err := d.cli.ContainerRemove(ctx, id, opts); err != nil {
		return fmt.Errorf("remove container %s: %w", shortid.Format(id), err)
	}
	return nil
}

// Sweep finds and removes all containers labeled zynqel.managed=true.
// Intended to run on boot to clean up orphans from previous runs.
func (d *DockerSandbox) Sweep(ctx context.Context) (int, error) {
	args := filters.NewArgs(filters.Arg("label", LabelManaged+"=true"))
	containers, err := d.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return 0, fmt.Errorf("list orphan containers: %w", err)
	}

	removed := 0
	for _, c := range containers {
		if err := d.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("warning: failed to remove orphan container %s: %v", shortid.Format(c.ID), err)
			continue
		}
		removed++
	}
	return removed, nil
}

// Attach connects to a running container's PTY (stdin + stdout/stderr).
// The container must have been created with Tty: true and OpenStdin: true.
func (d *DockerSandbox) Attach(ctx context.Context, id string) (PTYConn, error) {
	resp, err := d.cli.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("attach container %s: %w", shortid.Format(id), err)
	}
	return &dockerPTYConn{resp: resp}, nil
}

// dockerPTYConn wraps Docker's HijackedResponse as a PTYConn.
// With Tty: true, stdout and stderr are multiplexed into a single stream.
type dockerPTYConn struct {
	resp types.HijackedResponse
}

func (c *dockerPTYConn) Read(p []byte) (int, error) {
	return c.resp.Reader.Read(p)
}

func (c *dockerPTYConn) Write(p []byte) (int, error) {
	return c.resp.Conn.Write(p)
}

func (c *dockerPTYConn) Close() error {
	c.resp.Close()
	return nil
}

// Exec runs a command inside a running container with a PTY.
// Returns a PTYConn connected to the exec process's stdin/stdout.
func (d *DockerSandbox) Exec(ctx context.Context, id string, cmd []string) (PTYConn, error) {
	execCfg := container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	execResp, err := d.cli.ContainerExecCreate(ctx, id, execCfg)
	if err != nil {
		return nil, fmt.Errorf("exec create in container %s: %w", shortid.Format(id), err)
	}

	attachResp, err := d.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec attach in container %s: %w", shortid.Format(id), err)
	}

	return &dockerPTYConn{resp: attachResp}, nil
}

// ExecRun runs a command inside a container and waits for it to finish.
// Returns combined stdout/stderr output. Returns an error if the command
// exits with a non-zero status.
func (d *DockerSandbox) ExecRun(ctx context.Context, id string, cmd []string) ([]byte, error) {
	execCfg := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := d.cli.ContainerExecCreate(ctx, id, execCfg)
	if err != nil {
		return nil, fmt.Errorf("exec create in container %s: %w", shortid.Format(id), err)
	}

	attachResp, err := d.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("exec attach in container %s: %w", shortid.Format(id), err)
	}
	defer attachResp.Close()

	output, err := io.ReadAll(attachResp.Reader)
	if err != nil {
		return output, fmt.Errorf("exec read in container %s: %w", shortid.Format(id), err)
	}

	inspect, err := d.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return output, fmt.Errorf("exec inspect in container %s: %w", shortid.Format(id), err)
	}
	if inspect.ExitCode != 0 {
		return output, fmt.Errorf("exec in container %s exited with code %d: %s", shortid.Format(id), inspect.ExitCode, string(output))
	}

	return output, nil
}

// Stats returns point-in-time CPU and memory usage for a container.
func (d *DockerSandbox) Stats(ctx context.Context, id string) (*ContainerStats, error) {
	resp, err := d.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("stats container %s: %w", shortid.Format(id), err)
	}
	defer func() { _ = resp.Body.Close() }()

	var v container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode stats for %s: %w", shortid.Format(id), err)
	}

	// Calculate CPU percentage.
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	var cpuPercent float64
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(v.CPUStats.OnlineCPUs) * 100.0
	}

	memoryMB := float64(v.MemoryStats.Usage) / (1024 * 1024)
	memoryMaxMB := float64(v.MemoryStats.Limit) / (1024 * 1024)

	return &ContainerStats{
		CPUPercent: cpuPercent,
		MemoryMB:   memoryMB,
		MemoryMax:  memoryMaxMB,
	}, nil
}

// Commit saves the current container state as a new image.
// The image can be used to resume the workspace with all installed packages.
func (d *DockerSandbox) Commit(ctx context.Context, containerID, imageName string) error {
	resp, err := d.cli.ContainerCommit(ctx, containerID, container.CommitOptions{
		Reference: imageName,
		Comment:   "zynqel workspace snapshot",
		Pause:     true,
	})
	if err != nil {
		return fmt.Errorf("commit container %s: %w", shortid.Format(containerID), err)
	}
	log.Printf("committed container %s as %s (sha: %s)", shortid.Format(containerID), imageName, shortid.Format(resp.ID))
	return nil
}

// ImageExists checks if a Docker image exists locally.
func (d *DockerSandbox) ImageExists(ctx context.Context, imageName string) bool {
	_, err := d.cli.ImageInspect(ctx, imageName)
	return err == nil
}

// BuildImage builds a Docker image from a Dockerfile string.
func (d *DockerSandbox) BuildImage(ctx context.Context, dockerfile, imageName string) error {
	// Create a tar archive with just the Dockerfile.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: "Dockerfile",
		Mode: 0o644,
		Size: int64(len(dockerfile)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}
	if _, err := tw.Write([]byte(dockerfile)); err != nil {
		return fmt.Errorf("tar write: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}

	log.Printf("building image %s ...", imageName)
	resp, err := d.cli.ImageBuild(ctx, &buf, types.ImageBuildOptions{
		Tags:       []string{imageName},
		Dockerfile: "Dockerfile",
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("build image %s: %w", imageName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Drain build output to complete the build.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("build image %s: %w", imageName, err)
	}
	log.Printf("built image %s", imageName)
	return nil
}

// ListVolumes returns all Docker volumes matching the given name prefix.
func (d *DockerSandbox) ListVolumes(ctx context.Context, prefix string) ([]VolumeInfo, error) {
	resp, err := d.cli.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", prefix)),
	})
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	var vols []VolumeInfo
	for _, v := range resp.Volumes {
		vols = append(vols, VolumeInfo{
			Name:      v.Name,
			CreatedAt: v.CreatedAt,
			Image:     v.Labels["zynqel.image"],
			Agent:     v.Labels["zynqel.agent"],
		})
	}
	return vols, nil
}

// RemoveVolume removes a Docker volume by name.
func (d *DockerSandbox) RemoveVolume(ctx context.Context, name string) error {
	return d.cli.VolumeRemove(ctx, name, true)
}

// Resize changes the TTY dimensions of a running container.
func (d *DockerSandbox) Resize(ctx context.Context, id string, cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return d.cli.ContainerResize(ctx, id, container.ResizeOptions{
		Width:  uint(cols),
		Height: uint(rows),
	})
}

func (d *DockerSandbox) Close() error {
	return d.cli.Close()
}
