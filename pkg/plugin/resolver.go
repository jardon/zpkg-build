package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func ResolveAndStage(spec PluginSource, cacheDir string) (string, error) {
	expandedSource := os.ExpandEnv(spec.Source)

	if strings.TrimSpace(expandedSource) == "" {
		return "", nil
	}

	isRemote := strings.HasPrefix(expandedSource, "http://") || strings.HasPrefix(expandedSource, "https://")

	var sourcePath string

	if isRemote {
		targetDir := filepath.Join(cacheDir, "plugins", spec.Name, spec.Version)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create cache directory: %w", err)
		}

		archiveName := filepath.Base(expandedSource)
		sourcePath = filepath.Join(targetDir, archiveName)

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			if err := downloadFile(expandedSource, sourcePath); err != nil {
				return "", fmt.Errorf("failed to download remote plugin: %w", err)
			}
		}
	} else {
		localPath := strings.TrimPrefix(expandedSource, "file://")
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			return "", fmt.Errorf("invalid local path: %w", err)
		}
		sourcePath = absPath
	}

	if err := verifyChecksum(sourcePath, spec.SHA256); err != nil {
		return "", fmt.Errorf("integrity check failed for %s: %w", sourcePath, err)
	}

	return sourcePath, nil
}

func verifyChecksum(filePath, expectedSHA string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualSHA := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualSHA, expectedSHA) {
		return fmt.Errorf("mismatch (got %s, expected %s)", actualSHA, expectedSHA)
	}
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
