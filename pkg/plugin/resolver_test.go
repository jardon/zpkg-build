package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	content := []byte("test plugin content for checksum verification")

	t.Run("match", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "plugin.tar.gz")
		os.WriteFile(path, content, 0644)

		h := sha256.Sum256(content)
		expected := hex.EncodeToString(h[:])

		if err := verifyChecksum(path, expected); err != nil {
			t.Errorf("verifyChecksum() returned error: %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "plugin.tar.gz")
		os.WriteFile(path, content, 0644)

		err := verifyChecksum(path, "0000000000000000000000000000000000000000000000000000000000000000")
		if err == nil {
			t.Error("expected error on checksum mismatch")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		err := verifyChecksum("/nonexistent/file.tar.gz", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}
