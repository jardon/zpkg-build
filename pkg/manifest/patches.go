package manifest

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

func ResolveAndVerifyPatches(manifestPath string, patches []PatchSource, cacheDir string) ([]string, error) {
	manifestDir := filepath.Dir(manifestPath)
	var verifiedPatchPaths []string

	for idx, patch := range patches {
		if patch.SHA256 == "" || len(patch.SHA256) != 64 {
			return nil, fmt.Errorf("patch [%d] is missing a valid 64-character SHA-256 hash", idx)
		}

		var targetPath string

		if patch.URL != "" {
			targetPath = filepath.Join(cacheDir, "patches", patch.SHA256+".patch")
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
					return nil, err
				}
				if err := downloadPatch(patch.URL, targetPath); err != nil {
					return nil, fmt.Errorf("failed to download remote patch %s: %w", patch.URL, err)
				}
			}
		} else if patch.Path != "" {
			targetPath = filepath.Join(manifestDir, patch.Path)
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				return nil, fmt.Errorf("local patch file not found: %s", targetPath)
			}
		} else {
			return nil, fmt.Errorf("patch [%d] must declare either a 'path' or a 'url'", idx)
		}

		if err := verifyPatchHash(targetPath, patch.SHA256); err != nil {
			return nil, fmt.Errorf("integrity check failed for patch [%d] (%s): %w", idx, targetPath, err)
		}

		verifiedPatchPaths = append(verifiedPatchPaths, targetPath)
	}

	return verifiedPatchPaths, nil
}

func verifyPatchHash(filePath, expectedSHA string) error {
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

func downloadPatch(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code pulling patch: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
