//go:build cgo

package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	lxc "github.com/lxc/go-lxc"
)

type LXCEngine struct {
	containerName string
	configDir     string
	lxcContainer  *lxc.Container
}

func NewLXCEngine(socketPath string) *LXCEngine {
	configDir := os.Getenv("LXC_DIR")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".local", "share", "lxc")
	}
	return &LXCEngine{
		configDir: configDir,
	}
}

func (l *LXCEngine) Name() string {
	return "lxc"
}

func (l *LXCEngine) CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error {
	l.containerName = fmt.Sprintf("zpkg-build-%d", os.Getpid())
	containerPath := filepath.Join(l.configDir, l.containerName)

	if err := os.MkdirAll(containerPath, 0755); err != nil {
		return fmt.Errorf("failed to create container directory: %w", err)
	}

	lxcContainer, err := lxc.NewContainer(l.containerName, l.configDir)
	if err != nil {
		return fmt.Errorf("failed to create LXC container handle: %w", err)
	}
	l.lxcContainer = lxcContainer

	l.lxcContainer.SetVerbosity(lxc.Verbose)

	distro := l.distroFromImage(baseImage)
	release := l.releaseFromImage(baseImage)

	if err := l.lxcContainer.Create(lxc.TemplateOptions{
		Template: "download",
		Distro:   distro,
		Release:  release,
		Arch:     "amd64",
	}); err != nil {
		return fmt.Errorf("failed to create LXC template: %w", err)
	}

	for _, m := range mounts {
		options := "bind"
		if m.ReadOnly {
			options = "bind,ro"
		}
		mountEntry := fmt.Sprintf("%s %s none %s 0 0", m.HostPath, m.ContainerPath, options)
		l.lxcContainer.SetConfigItem("lxc.mount.entry", mountEntry)
	}

	l.lxcContainer.SetConfigItem("lxc.cap.drop", "ALL")
	l.lxcContainer.SetConfigItem("lxc.security.nesting", "false")

	if err := l.lxcContainer.SaveConfigFile(filepath.Join(containerPath, "config")); err != nil {
		return fmt.Errorf("failed to save LXC config: %w", err)
	}

	if err := l.lxcContainer.Start(); err != nil {
		return fmt.Errorf("failed to start LXC container: %w", err)
	}

	return nil
}

func (l *LXCEngine) Run(ctx context.Context, config RunConfig) error {
	if l.lxcContainer == nil {
		return fmt.Errorf("environment not initialized")
	}

	for _, cmdStr := range config.Commands {
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}

		args := []string{"sh", "-c", cmdStr}

		options := lxc.DefaultAttachOptions
		options.ClearEnv = false

		for k, v := range config.EnvVars {
			options.Env = append(options.Env, fmt.Sprintf("%s=%s", k, v))
		}

		if config.WorkingDir != "" {
			options.Cwd = config.WorkingDir
		}

		ok, err := l.lxcContainer.RunCommand(args, options)
		if err != nil {
			return fmt.Errorf("command '%s' failed: %w", cmdStr, err)
		}
		if !ok {
			return fmt.Errorf("command '%s' exited with non-zero status", cmdStr)
		}
	}

	return nil
}

func (l *LXCEngine) CopyTo(ctx context.Context, hostSrc, guestDest string) error {
	if l.lxcContainer == nil {
		return fmt.Errorf("environment not initialized")
	}

	rootfsPath := filepath.Join(l.configDir, l.containerName, "rootfs")
	targetPath := filepath.Join(rootfsPath, guestDest)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	cmd := exec.Command("cp", "-a", hostSrc, targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy to LXC rootfs: %w\noutput: %s", err, string(output))
	}

	return nil
}

func (l *LXCEngine) CopyFrom(ctx context.Context, guestSrc, hostDest string) error {
	if l.lxcContainer == nil {
		return fmt.Errorf("environment not initialized")
	}

	rootfsPath := filepath.Join(l.configDir, l.containerName, "rootfs")
	sourcePath := filepath.Join(rootfsPath, guestSrc)

	cmd := exec.Command("cp", "-a", sourcePath, hostDest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy from LXC rootfs: %w\noutput: %s", err, string(output))
	}

	return nil
}

func (l *LXCEngine) Destroy(ctx context.Context) error {
	if l.lxcContainer == nil {
		return nil
	}

	if l.lxcContainer.Running() {
		if err := l.lxcContainer.Stop(); err != nil {
			_ = l.lxcContainer.Shutdown(0)
		}
	}

	if err := l.lxcContainer.Destroy(); err != nil {
		return fmt.Errorf("failed to destroy LXC container: %w", err)
	}

	l.lxcContainer = nil
	l.containerName = ""
	return nil
}

func (l *LXCEngine) distroFromImage(image string) string {
	parts := strings.SplitN(image, ":", 2)
	name := parts[0]
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	return name
}

func (l *LXCEngine) releaseFromImage(image string) string {
	parts := strings.SplitN(image, ":", 2)
	if len(parts) < 2 {
		return "latest"
	}
	release := parts[1]
	if idx := strings.Index(release, "@"); idx != -1 {
		release = release[:idx]
	}
	return release
}
