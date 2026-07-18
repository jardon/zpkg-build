//go:build cgo

package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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
	if err := l.setupUnprivileged(); err != nil {
		return fmt.Errorf("failed to setup unprivileged containers: %w", err)
	}

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

	subuid, subuidCount, err := parseSubordinate("subuid")
	if err != nil {
		return fmt.Errorf("failed to parse /etc/subuid: %w", err)
	}

	subgid, subgidCount, err := parseSubordinate("subgid")
	if err != nil {
		return fmt.Errorf("failed to parse /etc/subgid: %w", err)
	}

	l.lxcContainer.SetConfigItem("lxc.idmap", fmt.Sprintf("u 0 %d %d", subuid, subuidCount))
	l.lxcContainer.SetConfigItem("lxc.idmap", fmt.Sprintf("g 0 %d %d", subgid, subgidCount))

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

func parseSubordinate(name string) (int, int, error) {
	file, err := os.Open("/etc/" + name)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	currentUser, err := user.Current()
	if err != nil {
		return 0, 0, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		if parts[0] == currentUser.Username {
			start, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid subordinate start %q: %w", parts[1], err)
			}
			count, err := strconv.Atoi(parts[2])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid subordinate count %q: %w", parts[2], err)
			}
			return start, count, nil
		}
	}

	return 0, 0, fmt.Errorf("no entry for %s in /etc/%s", currentUser.Username, name)
}

func (l *LXCEngine) setupUnprivileged() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Ensure parent directories of configDir have no execute permission
	// This is a security requirement for unprivileged LXC containers
	dirs := []string{
		home,
		filepath.Join(home, ".local"),
		filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".local", "share", "lxc"),
	}
	for _, dir := range dirs {
		os.Chmod(dir, 0755&^0111)
	}

	// Ensure user LXC config exists
	configDir := filepath.Join(home, ".config", "lxc")
	configFile := filepath.Join(configDir, "default.conf")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create ~/.config/lxc: %w", err)
		}

		subuid, subuidCount, err := parseSubordinate("subuid")
		if err != nil {
			return fmt.Errorf("failed to parse /etc/subuid: %w", err)
		}

		subgid, subgidCount, err := parseSubordinate("subgid")
		if err != nil {
			return fmt.Errorf("failed to parse /etc/subgid: %w", err)
		}

		content := fmt.Sprintf(
			"lxc.idmap = u 0 %d %d\n"+
				"lxc.idmap = g 0 %d %d\n",
			subuid, subuidCount,
			subgid, subgidCount,
		)

		if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", configFile, err)
		}
	}

	return nil
}
