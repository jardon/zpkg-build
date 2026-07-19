package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

type LXDEngine struct {
	client        lxd.InstanceServer
	containerName string
}

func NewLXDEngine(socketPath string) *LXDEngine {
	return &LXDEngine{}
}

func (l *LXDEngine) Name() string {
	return "lxc"
}

func (l *LXDEngine) connect() error {
	if l.client != nil {
		return nil
	}

	socketPath := findLXDSocket()
	c, err := lxd.ConnectLXDUnix(socketPath, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to LXD: %w", err)
	}
	l.client = c
	return nil
}

func findLXDSocket() string {
	candidates := []string{
		os.Getenv("LXD_DIR") + "/unix.socket",
		"/var/snap/lxd/common/lxd/unix.socket",
		"/var/lib/lxd/unix.socket",
		filepath.Join(os.Getenv("HOME"), "snap", "lxd", "common", "lxd", "unix.socket"),
	}

	for _, path := range candidates {
		if path == "/unix.socket" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (l *LXDEngine) CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error {
	if err := l.connect(); err != nil {
		return err
	}

	l.containerName = fmt.Sprintf("zpkg-build-%d", os.Getpid())
	alias := l.aliasFromImage(baseImage)

	req := api.InstancesPost{
		Name: l.containerName,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: alias,
		},
		Type: api.InstanceTypeContainer,
	}

	op, err := l.client.CreateInstance(req)
	if err != nil {
		return fmt.Errorf("failed to create LXC instance: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed to create LXC instance: %w", err)
	}

	if len(mounts) > 0 {
		instance, etag, err := l.client.GetInstance(l.containerName)
		if err != nil {
			return fmt.Errorf("failed to get instance config: %w", err)
		}

		if instance.Devices == nil {
			instance.Devices = make(map[string]map[string]string)
		}

		for i, m := range mounts {
			deviceName := fmt.Sprintf("zpkg-mount-%d", i)
			instance.Devices[deviceName] = map[string]string{
				"type":   "disk",
				"path":   m.ContainerPath,
				"source": m.HostPath,
			}
			if m.ReadOnly {
				instance.Devices[deviceName]["readonly"] = "true"
			}
		}

		op, err = l.client.UpdateInstance(l.containerName, api.InstancePut{
			Config:  instance.Config,
			Devices: instance.Devices,
		}, etag)
		if err != nil {
			return fmt.Errorf("failed to configure mounts: %w", err)
		}

		if err := op.Wait(); err != nil {
			return fmt.Errorf("failed to configure mounts: %w", err)
		}
	}

	op, err = l.client.UpdateInstanceState(l.containerName, api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}, "")
	if err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	return nil
}

func (l *LXDEngine) Run(ctx context.Context, config RunConfig) error {
	if l.client == nil {
		return fmt.Errorf("environment not initialized")
	}

	for _, cmdStr := range config.Commands {
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}

		var fullCmd string
		if len(config.EnvVars) > 0 {
			var exports []string
			for k, v := range config.EnvVars {
				exports = append(exports, fmt.Sprintf("export %s=%q", k, v))
			}
			fullCmd = strings.Join(exports, "; ")
		}
		if config.WorkingDir != "" {
			fullCmd += fmt.Sprintf("; cd %s", config.WorkingDir)
		}
		if fullCmd != "" {
			fullCmd += "; "
		}
		fullCmd += cmdStr

		var stdout, stderr bytes.Buffer
		execReq := api.InstanceExecPost{
			Command:  []string{"sh", "-c", fullCmd},
			WaitForWS: true,
		}
		execArgs := lxd.InstanceExecArgs{
			Stdout: &stdout,
			Stderr: &stderr,
			DataDone: make(chan bool),
		}

		op, err := l.client.ExecInstance(l.containerName, execReq, &execArgs)
		if err != nil {
			return fmt.Errorf("failed to exec command '%s': %w", cmdStr, err)
		}

		<-execArgs.DataDone

		if err := op.Wait(); err != nil {
			return fmt.Errorf("command '%s' failed: %w", cmdStr, err)
		}
	}

	return nil
}

func (l *LXDEngine) CopyTo(ctx context.Context, hostSrc, guestDest string) error {
	if l.client == nil {
		return fmt.Errorf("environment not initialized")
	}

	tarReader, err := tarArchive(hostSrc)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	defer tarReader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, tarReader); err != nil {
		return fmt.Errorf("failed to read tar archive: %w", err)
	}

	args := lxd.InstanceFileArgs{
		Content:  bytes.NewReader(buf.Bytes()),
		Type:     "file",
		WriteMode: "overwrite",
	}

	if err := l.client.CreateInstanceFile(l.containerName, guestDest, args); err != nil {
		return fmt.Errorf("failed to copy to LXC instance: %w", err)
	}

	return nil
}

func (l *LXDEngine) CopyFrom(ctx context.Context, guestSrc, hostDest string) error {
	if l.client == nil {
		return fmt.Errorf("environment not initialized")
	}

	content, _, err := l.client.GetInstanceFile(l.containerName, guestSrc)
	if err != nil {
		return fmt.Errorf("failed to copy from LXC instance: %w", err)
	}
	defer content.Close()

	if err := extractTar(content, hostDest); err != nil {
		return fmt.Errorf("failed to extract tar: %w", err)
	}

	return nil
}

func (l *LXDEngine) Destroy(ctx context.Context) error {
	if l.client == nil || l.containerName == "" {
		return nil
	}

	op, err := l.client.UpdateInstanceState(l.containerName, api.InstanceStatePut{
		Action:  "stop",
		Timeout: 30,
	}, "")
	if err == nil {
		_ = op.Wait()
	}

	op, err = l.client.DeleteInstance(l.containerName, true)
	if err != nil {
		return fmt.Errorf("failed to delete LXC instance: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed to delete LXC instance: %w", err)
	}

	l.containerName = ""
	return nil
}

func (l *LXDEngine) aliasFromImage(image string) string {
	parts := strings.SplitN(image, ":", 2)
	name := parts[0]
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	release := "latest"
	if len(parts) > 1 {
		release = parts[1]
		if idx := strings.Index(release, "@"); idx != -1 {
			release = release[:idx]
		}
	}

	return name + "/" + release
}
