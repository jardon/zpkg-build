package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	dockerContainer "github.com/docker/docker/api/types/container"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type PodmanEngine struct {
	socketConn  context.Context
	containerID string
}

func NewPodmanEngine(socketPath string) *PodmanEngine {
	return &PodmanEngine{}
}

func (p *PodmanEngine) Name() string {
	return "podman"
}

func (p *PodmanEngine) resolveSocketPath() string {
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
	f := false
	t := false
	s.Terminal = &f
	s.NoNewPrivileges = &t
	s.CapDrop = []string{"ALL"}

	for _, m := range mounts {
		mode := "rw"
		if m.ReadOnly {
			mode = "ro"
		}
		if m.SELinuxLabel {
			mode += ",Z"
		}
		s.Mounts = append(s.Mounts, spec.Mount{
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

		args := []string{"sh", "-c", cmd}

		var envList []string
		for k, v := range config.EnvVars {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}

		createConfig := &handlers.ExecCreateConfig{
			ExecOptions: dockerContainer.ExecOptions{
				AttachStdout: true,
				AttachStderr: true,
				Cmd:          args,
				WorkingDir:   config.WorkingDir,
				Env:          envList,
			},
		}

		execID, err := containers.ExecCreate(p.socketConn, p.containerID, createConfig)
		if err != nil {
			return fmt.Errorf("failed to create exec session for '%s': %w", cmd, err)
		}

		execOpts := new(containers.ExecStartAndAttachOptions)
		t := true
		execOpts.AttachOutput = &t
		execOpts.AttachError = &t
		var stdout io.Writer = os.Stdout
		var stderr io.Writer = os.Stderr
		execOpts.OutputStream = &stdout
		execOpts.ErrorStream = &stderr
		err = containers.ExecStartAndAttach(p.socketConn, execID, execOpts)
		if err != nil {
			return fmt.Errorf("execution error on command '%s': %w", cmd, err)
		}

		inspect, err := containers.ExecInspect(p.socketConn, execID, nil)
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

	tarReader, err := tarArchive(hostSrc)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	defer tarReader.Close()

	copyFunc, err := containers.CopyFromArchive(p.socketConn, p.containerID, guestDest, tarReader)
	if err != nil {
		return fmt.Errorf("failed to copy files into podman environment: %w", err)
	}
	if err := copyFunc(); err != nil {
		return fmt.Errorf("failed to apply copy: %w", err)
	}
	return nil
}

func (p *PodmanEngine) CopyFrom(ctx context.Context, guestSrc, hostDest string) error {
	if p.containerID == "" || p.socketConn == nil {
		return fmt.Errorf("environment not initialized")
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	copyFunc, err := containers.CopyToArchive(p.socketConn, p.containerID, guestSrc, pw)
	if err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("failed to extract outputs from podman environment: %w", err)
	}

	go func() {
		copyFunc()
		pw.Close()
	}()

	if err := extractTar(pr, hostDest); err != nil {
		pr.Close()
		return fmt.Errorf("failed to extract tar: %w", err)
	}
	pr.Close()
	return nil
}

func (p *PodmanEngine) Destroy(ctx context.Context) error {
	if p.containerID == "" || p.socketConn == nil {
		return nil
	}

	stopOpts := new(containers.StopOptions)
	_ = containers.Stop(p.socketConn, p.containerID, stopOpts)

	removeOpts := new(containers.RemoveOptions)
	removeOpts.WithForce(true).WithVolumes(true)
	_, err := containers.Remove(p.socketConn, p.containerID, removeOpts)

	p.containerID = ""
	return err
}
