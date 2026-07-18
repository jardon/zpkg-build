package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerEngine struct {
	client      *client.Client
	containerID string
	baseEnv     map[string]string
}

func NewDockerEngine(socketPath string) *DockerEngine {
	return &DockerEngine{}
}

func (d *DockerEngine) Name() string {
	return "docker"
}

func (d *DockerEngine) ensureClient() error {
	if d.client != nil {
		return nil
	}

	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	if sock := os.Getenv("DOCKER_HOST"); sock != "" {
		opts = append(opts, client.WithHost(sock))
	}

	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	d.client = c
	return nil
}

func (d *DockerEngine) CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	reader, err := d.client.ImagePull(ctx, baseImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", baseImage, err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	var bindMounts []mount.Mount
	for _, m := range mounts {
		bindMode := "rw"
		if m.ReadOnly {
			bindMode = "ro"
		}
		if m.SELinuxLabel {
			bindMode += ",Z"
		}

		bindMounts = append(bindMounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.HostPath,
			Target:   m.ContainerPath,
			ReadOnly: m.ReadOnly,
			BindOptions: &mount.BindOptions{
				Propagation: mount.PropagationPrivate,
			},
		})
	}

	capDrop := []string{"ALL"}
	containerCfg := &container.Config{
		Image: baseImage,
		Cmd:   []string{"sleep", "infinity"},
	}

	hostCfg := &container.HostConfig{
		Mounts:      bindMounts,
		SecurityOpt: []string{"no-new-privileges"},
		CapDrop:     capDrop,
		NetworkMode: container.NetworkMode("none"),
	}

	networkCfg := &network.NetworkingConfig{}

	resp, err := d.client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	d.containerID = resp.ID

	if err := d.client.ContainerStart(ctx, d.containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

func (d *DockerEngine) Run(ctx context.Context, config RunConfig) error {
	if d.containerID == "" {
		return fmt.Errorf("environment not initialized")
	}

	if d.baseEnv == nil {
		d.baseEnv = d.getContainerEnv(ctx)
	}

	for _, cmdStr := range config.Commands {
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}

		merged := make(map[string]string)
		for k, v := range d.baseEnv {
			merged[k] = v
		}
		for k, v := range config.EnvVars {
			merged[k] = v
		}

		var exports []string
		for k, v := range merged {
			exports = append(exports, fmt.Sprintf("export %s=%q", k, v))
		}
		fullCmd := strings.Join(exports, "; ")
		if config.WorkingDir != "" {
			fullCmd += fmt.Sprintf("; cd %s", config.WorkingDir)
		}
		fullCmd += "; " + cmdStr

		execCfg := container.ExecOptions{
			Cmd:          []string{"sh", "-c", fullCmd},
			AttachStdout: true,
			AttachStderr: true,
		}

		execID, err := d.client.ContainerExecCreate(ctx, d.containerID, execCfg)
		if err != nil {
			return fmt.Errorf("failed to create exec for '%s': %w", cmdStr, err)
		}

		resp, err := d.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
		if err != nil {
			return fmt.Errorf("failed to attach exec for '%s': %w", cmdStr, err)
		}
		defer resp.Close()

		stdcopy.StdCopy(os.Stdout, os.Stderr, resp.Reader)

		inspect, err := d.client.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect exec for '%s': %w", cmdStr, err)
		}
		if inspect.ExitCode != 0 {
			return fmt.Errorf("command '%s' exited with code %d", cmdStr, inspect.ExitCode)
		}
	}

	return nil
}

func (d *DockerEngine) getContainerEnv(ctx context.Context) map[string]string {
	envMap := make(map[string]string)

	inspect, err := d.client.ContainerInspect(ctx, d.containerID)
	if err != nil {
		return envMap
	}

	for _, e := range inspect.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	return envMap
}

func (d *DockerEngine) CopyTo(ctx context.Context, hostSrc, guestDest string) error {
	if d.containerID == "" {
		return fmt.Errorf("environment not initialized")
	}

	tarReader, err := tarArchive(hostSrc)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	defer tarReader.Close()

	if err := d.client.CopyToContainer(ctx, d.containerID, filepath.Dir(guestDest), tarReader, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	return nil
}

func (d *DockerEngine) CopyFrom(ctx context.Context, guestSrc, hostDest string) error {
	if d.containerID == "" {
		return fmt.Errorf("environment not initialized")
	}

	reader, _, err := d.client.CopyFromContainer(ctx, d.containerID, guestSrc)
	if err != nil {
		return fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	if err := extractTar(reader, hostDest); err != nil {
		return fmt.Errorf("failed to extract tar: %w", err)
	}

	return nil
}

func (d *DockerEngine) Destroy(ctx context.Context) error {
	if d.containerID == "" {
		return nil
	}

	if d.client == nil {
		return nil
	}

	d.client.ContainerRemove(ctx, d.containerID, container.RemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})

	d.containerID = ""
	return nil
}
