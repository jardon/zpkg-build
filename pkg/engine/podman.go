package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
)

type PodmanEngine struct {
	socketConn  context.Context
	containerID string
}

func NewPodmanEngine(socketPath string) *PodmanEngine {
	return &PodmanEngine{
		socketPath: socketPath,
	}
}

func (p *PodmanEngine) Name() string {
	return "podman"
}

func (p *PodmanEngine) resolveSocketPath() string {
	if p.socketPath != "" {
		return p.socketPath
	}

	if uid := os.Getuid(); uid != 0 {
		return fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", uid)
	}
	return "unix:///run/podman/podman.sock"
}

func (p *PodmanEngine) CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error {
	conn, err := bindings.NewConnection(ctx, p.resolveSocketPath())
	if err != nil {
		return fmt.Errorf("failed to connect to podman socket: %w", err)
	}
	p.socketConn = conn

	_, err = images.Pull(p.socketConn, baseImage, nil)
	if err != nil {
		return fmt.Errorf("failed to pull image %s via podman: %w", baseImage, err)
	}

	s := specgen.NewSpecGenerator(baseImage, false)
	s.Terminal = false
	s.ContainerSecurityConfig.CapDrop = []string{"ALL"}
	s.ContainerSecurityConfig.NoNewPrivileges = true

	for _, m := range mounts {
		mode := "rw"
		if m.ReadOnly {
			mode = "ro"
		}
		if m.SELinuxLabel {
			mode += ",Z"
		}
		s.Mounts = append(s.Mounts, specgen.Mount{
			Type:        "bind",
			Source:      m.HostPath,
			Destination: m.ContainerPath,
			Options:     strings.Split(mode, ","),
		})
	}

	s.Command = []string{"sleep", "infinity"}

	createResponse, err := containers.CreateWithSpec(p.socketConn, s, nil)
	if err != nil {
		return fmt.Errorf("failed to create podman container: %w", err)
	}
	p.containerID = createResponse.ID

	if err := containers.Start(p.socketConn, p.containerID, nil); err != nil {
		return fmt.Errorf("failed to start podman container %s: %w", p.containerID, err)
	}

	return nil
}

func (p *PodmanEngine) Run(ctx context.Context, config RunConfig) error {
	if p.containerID == "" || p.socketConn == nil {
		return fmt.Errorf("environment not initialized")
	}

	for _, cmd := range config.Commands {
		if strings.TrimSpace(cmd) == "" {
			continue
		}

		args := strings.Fields(cmd)

		execConfig := new(containers.ExecStartAndAttachOptions)
		execConfig.WithAttachStdout(true).WithAttachStderr(true)

		createConfig := new(containers.ExecCreateConfig)
		createConfig.WithCmd(args).WithWorkingDir(config.WorkingDir)

		var envList []string
		for k, v := range config.EnvVars {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}
		createConfig.WithEnv(envList)

		execID, err := containers.ExecCreate(p.socketConn, p.containerID, createConfig)
		if err != nil {
			return fmt.Errorf("failed to create exec session for '%s': %w", cmd, err)
		}

		err = containers.ExecStartAndAttach(p.socketConn, execID, execConfig)
		if err != nil {
			return fmt.Errorf("execution error on command '%s': %w", cmd, err)
		}

		inspect, err := containers.ExecInspect(p.socketConn, execID)
		if err != nil {
			return fmt.Errorf("failed to inspect session status: %w", err)
		}
		if inspect.ExitCode != 0 {
			return fmt.Errorf("command '%s' exited with code %d", cmd, inspect.ExitCode)
		}
	}

	return nil
}

func (p *PodmanEngine) CopyTo(ctx context.Context, hostSrc, guestDest string) error {
	if p.containerID == "" || p.socketConn == nil {
		return fmt.Errorf("environment not initialized")
	}

	copyOpts := new(containers.CopyOptions)
	err := containers.CopyToContainer(p.socketConn, p.containerID, hostSrc, guestDest, copyOpts)
	if err != nil {
		return fmt.Errorf("failed to copy files into podman environment: %w", err)
	}
	return nil
}

func (p *PodmanEngine) CopyFrom(ctx context.Context, guestSrc, hostDest string) error {
	if p.containerID == "" || p.socketConn == nil {
		return fmt.Errorf("environment not initialized")
	}

	copyOpts := new(containers.CopyOptions)
	err := containers.CopyFromContainer(p.socketConn, p.containerID, guestSrc, hostDest, copyOpts)
	if err != nil {
		return fmt.Errorf("failed to extract outputs from podman environment: %w", err)
	}
	return nil
}

func (p *PodmanEngine) Destroy(ctx context.Context) error {
	if p.containerID == "" || p.socketConn == nil {
		return nil
	}

	force := true
	stopOpts := new(containers.StopOptions)
	_ = containers.Stop(p.socketConn, p.containerID, stopOpts)

	removeOpts := new(containers.RemoveOptions)
	removeOpts.WithForce(force).WithVolumes(true)
	_, err := containers.Remove(p.socketConn, p.containerID, removeOpts)

	p.containerID = ""
	return err
}
