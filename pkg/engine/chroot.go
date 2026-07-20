package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type ChrootEngine struct {
	rootfsPath string
	mounts     []Mount
	cacheDir   string
}

func NewChrootEngine(socketPath string) *ChrootEngine {
	return &ChrootEngine{}
}

func (c *ChrootEngine) Name() string {
	return "chroot"
}

func (c *ChrootEngine) CreateEnvironment(ctx context.Context, baseImage string, mounts []Mount) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	c.cacheDir = filepath.Join(homeDir, ".local", "share", "zpkg-build", "cache", "base-images")

	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	slug := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(baseImage)
	c.rootfsPath = filepath.Join(c.cacheDir, slug)

	if _, err := os.Stat(filepath.Join(c.rootfsPath, "bin", "sh")); err == nil {
		c.mounts = mounts
		if err := c.setupMounts(); err != nil {
			return fmt.Errorf("failed to setup mounts: %w", err)
		}
		return nil
	}

	archivePath, err := c.downloadOrCache(ctx, baseImage)
	if err != nil {
		return fmt.Errorf("failed to obtain rootfs archive: %w", err)
	}

	if err := os.MkdirAll(c.rootfsPath, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs directory: %w", err)
	}

	if err := c.extractArchive(archivePath, c.rootfsPath); err != nil {
		return fmt.Errorf("failed to extract rootfs: %w", err)
	}

	c.mounts = mounts

	if err := c.setupMounts(); err != nil {
		return fmt.Errorf("failed to setup mounts: %w", err)
	}

	return nil
}

func (c *ChrootEngine) downloadOrCache(ctx context.Context, url string) (string, error) {
	slug := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(url)
	archivePath := filepath.Join(c.cacheDir, slug+".tar")

	if _, err := os.Stat(archivePath); err == nil {
		return archivePath, nil
	}

	tmpPath := archivePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	out.Close()

	if err := os.Rename(tmpPath, archivePath); err != nil {
		return "", err
	}

	_ = hex.EncodeToString(hasher.Sum(nil))
	return archivePath, nil
}

func (c *ChrootEngine) extractArchive(archivePath, destDir string) error {
	cmd := exec.Command("tar", "-xf", archivePath, "-C", destDir, "--strip-components=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar extraction failed: %w\noutput: %s", err, string(output))
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

		env := []string{"HOME=/root", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
		for k, v := range config.EnvVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}

		var workDir string
		if config.WorkingDir != "" {
			workDir = config.WorkingDir
		} else {
			workDir = "/"
		}

		args := []string{
			"unshare",
			"--pid",
			"--fork",
			"--mount",
			"--mount-proc=" + filepath.Join(c.rootfsPath, "proc"),
			"chroot",
			c.rootfsPath,
			"sh", "-c",
			fmt.Sprintf("cd %s && %s", workDir, cmdStr),
		}

		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = env
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: 0,
				Gid: 0,
			},
		}

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

func (c *ChrootEngine) CopyTarStream(ctx context.Context, tarReader io.Reader, guestDest string) error {
	tmpDir, err := os.MkdirTemp("", "zpkg-chroot-tar-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTar(tarReader, tmpDir); err != nil {
		return fmt.Errorf("failed to extract tar stream: %w", err)
	}

	target := filepath.Join(c.rootfsPath, guestDest)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	cmd := exec.Command("cp", "-a", tmpDir+"/.", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy extracted tar to chroot: %w\noutput: %s", err, string(output))
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
