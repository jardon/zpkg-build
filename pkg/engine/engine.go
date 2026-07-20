package engine

import (
	"context"
	"io"
)

type Mount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
	SELinuxLabel  bool
}

type RunConfig struct {
	EnvVars    map[string]string
	WorkingDir string
	Commands   []string
}

type Engine interface {
	Name() string
	CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error
	Run(ctx context.Context, config RunConfig) error
	RunOutput(ctx context.Context, config RunConfig) (string, error)
	CopyTo(ctx context.Context, hostSrc, guestDest string) error
	CopyFrom(ctx context.Context, guestSrc, hostDest string) error
	CopyTarStream(ctx context.Context, tarReader io.Reader, guestDest string) error
	Destroy(ctx context.Context) error
}

func New(name, socketPath string) (Engine, error) {
	switch name {
	case "podman":
		return NewPodmanEngine(socketPath), nil
	case "docker":
		return NewDockerEngine(socketPath), nil
	case "lxc":
		return NewLXCEngine(socketPath), nil
	case "chroot":
		return NewChrootEngine(socketPath), nil
	default:
		return nil, &UnknownEngineError{Name: name}
	}
}

type UnknownEngineError struct {
	Name string
}

func (e *UnknownEngineError) Error() string {
	return "unsupported engine: " + e.Name
}
