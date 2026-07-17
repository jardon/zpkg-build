package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type ChrootEngine struct {
	rootfsPath string
	mounts     []Mount
	pid        int
}

func NewChrootEngine(socketPath string) *ChrootEngine {
	return &ChrootEngine{}
}

func (c *ChrootEngine) Name() string {
	return "chroot"
}

func (c *ChrootEngine) CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error {
	cacheDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	c.rootfsPath = filepath.Join(cacheDir, ".local", "share", "zpkg-build", "cache", "base-images", baseImage)

	if err := os.MkdirAll(c.rootfsPath, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "debootstrap", "--variant=minbase", c.releaseFromImage(baseImage), c.rootfsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to bootstrap rootfs: %w\noutput: %s", err, string(output))
	}

	c.mounts = mounts

	if err := c.setupMounts(); err != nil {
		return fmt.Errorf("failed to setup mounts: %w", err)
	}

	return nil
}

func (c *ChrootEngine) setupMounts() error {
	bindMounts := []struct {
		source string
		target string
		flags  uintptr
	}{
		{"/proc", filepath.Join(c.rootfsPath, "proc"), syscall.MS_BIND | syscall.MS_REC},
		{"/sys", filepath.Join(c.rootfsPath, "sys"), syscall.MS_BIND | syscall.MS_REC},
		{"/dev", filepath.Join(c.rootfsPath, "dev"), syscall.MS_BIND | syscall.MS_REC},
		{"/dev/pts", filepath.Join(c.rootfsPath, "dev", "pts"), syscall.MS_BIND | syscall.MS_REC},
	}

	for _, bm := range bindMounts {
		if err := os.MkdirAll(bm.target, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", bm.target, err)
		}

		if err := syscall.Mount(bm.source, bm.target, "", bm.flags, ""); err != nil {
			return fmt.Errorf("failed to mount %s to %s: %w", bm.source, bm.target, err)
		}
	}

	for _, m := range c.mounts {
		target := filepath.Join(c.rootfsPath, m.ContainerPath)
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", target, err)
		}

		flags := uintptr(syscall.MS_BIND | syscall.MS_REC)
		if m.ReadOnly {
			flags |= syscall.MS_RDONLY
		}

		if err := syscall.Mount(m.HostPath, target, "", flags, ""); err != nil {
			return fmt.Errorf("failed to bind mount %s to %s: %w", m.HostPath, target, err)
		}
	}

	return nil
}

func (c *ChrootEngine) Run(ctx context.Context, config RunConfig) error {
	if c.rootfsPath == "" {
		return fmt.Errorf("environment not initialized")
	}

	for _, cmdStr := range config.Commands {
		if cmdStr == "" {
			continue
		}

		args := []string{"chroot", c.rootfsPath}

		if config.WorkingDir != "" {
			args = append(args, "sh", "-c", fmt.Sprintf("cd %s && %s", config.WorkingDir, cmdStr))
		} else {
			args = append(args, "sh", "-c", cmdStr)
		}

		cmd := exec.CommandContext(ctx, args[0], args[1:]...)

		env := []string{}
		for k, v := range config.EnvVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command '%s' failed: %w\nstdout: %s\nstderr: %s", cmdStr, err, stdout.String(), stderr.String())
		}
	}

	return nil
}

func (c *ChrootEngine) CopyTo(ctx context.Context, hostSrc, guestDest string) error {
	target := filepath.Join(c.rootfsPath, guestDest)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	cmd := exec.Command("cp", "-a", hostSrc, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy to chroot: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (c *ChrootEngine) CopyFrom(ctx context.Context, guestSrc, hostDest string) error {
	source := filepath.Join(c.rootfsPath, guestSrc)

	cmd := exec.Command("cp", "-a", source, hostDest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy from chroot: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (c *ChrootEngine) Destroy(ctx context.Context) error {
	if c.rootfsPath == "" {
		return nil
	}

	for _, m := range c.mounts {
		target := filepath.Join(c.rootfsPath, m.ContainerPath)
		syscall.Unmount(target, 0)
	}

	bindMounts := []string{
		filepath.Join(c.rootfsPath, "dev", "pts"),
		filepath.Join(c.rootfsPath, "dev"),
		filepath.Join(c.rootfsPath, "sys"),
		filepath.Join(c.rootfsPath, "proc"),
	}

	for i := len(bindMounts) - 1; i >= 0; i-- {
		syscall.Unmount(bindMounts[i], 0)
	}

	_ = os.RemoveAll(c.rootfsPath)
	c.rootfsPath = ""
	return nil
}

func (c *ChrootEngine) releaseFromImage(image string) string {
	parts := []byte(image)
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == ':' {
			release := string(parts[i+1:])
			if idx := -1; idx < len(release) {
				for j, ch := range release {
					if ch == '@' {
						return release[:j]
					}
				}
			}
			return release
		}
	}
	return "stable"
}
