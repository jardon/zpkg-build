package manifest

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func PatchChecksum(patch PatchSource) string {
	if len(patch.SHA256) == 64 {
		return patch.SHA256
	}
	return patch.MD5
}

func ResolveAndVerifyPatches(manifestPath string, patches []PatchSource, cacheDir string) ([]string, error) {
	manifestDir := filepath.Dir(manifestPath)
	var verifiedPatchPaths []string

	for idx, patch := range patches {
		hasSHA := len(patch.SHA256) == 64
		hasMD5 := len(patch.MD5) == 32
		if !hasSHA && !hasMD5 {
			return nil, fmt.Errorf("patch [%d] requires a valid SHA-256 (64 hex) or MD5 (32 hex) checksum", idx)
		}

		var targetPath string

		if patch.URL != "" {
			targetPath = filepath.Join(cacheDir, "patches", PatchChecksum(patch)+".patch")
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

		expected := PatchChecksum(patch)
		if err := verifyPatchHash(targetPath, expected); err != nil {
			return nil, fmt.Errorf("integrity check failed for patch [%d] (%s): %w", idx, targetPath, err)
		}

		verifiedPatchPaths = append(verifiedPatchPaths, targetPath)
	}

	return verifiedPatchPaths, nil
}

func verifyPatchHash(filePath, expected string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if len(expected) == 64 {
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		actual := hex.EncodeToString(hash.Sum(nil))
		if !strings.EqualFold(actual, expected) {
			return fmt.Errorf("SHA-256 mismatch (got %s, expected %s)", actual, expected)
		}
	} else {
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		actual := hex.EncodeToString(hash.Sum(nil))
		if !strings.EqualFold(actual, expected) {
			return fmt.Errorf("MD5 mismatch (got %s, expected %s)", actual, expected)
		}
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
