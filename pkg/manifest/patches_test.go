package manifest

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchChecksum(t *testing.T) {
	tests := []struct {
		name   string
		patch  PatchSource
		expect string
	}{
		{
			name:   "SHA256 wins when 64 hex chars",
			patch:  PatchSource{SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
			expect: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:   "falls back to MD5 when SHA256 absent",
			patch:  PatchSource{MD5: "abcdef0123456789abcdef0123456789"},
			expect: "abcdef0123456789abcdef0123456789",
		},
		{
			name:   "falls back to MD5 when SHA256 too short",
			patch:  PatchSource{SHA256: "abc", MD5: "abcdef0123456789abcdef0123456789"},
			expect: "abcdef0123456789abcdef0123456789",
		},
		{
			name:   "returns empty when neither set",
			patch:  PatchSource{},
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchChecksum(tt.patch)
			if got != tt.expect {
				t.Errorf("PatchChecksum() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestVerifyPatchHash(t *testing.T) {
	content := []byte("test patch content for hashing")

	t.Run("SHA256 match", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "patch.patch")
		os.WriteFile(path, content, 0644)

		h := sha256.Sum256(content)
		expected := hex.EncodeToString(h[:])

		if err := verifyPatchHash(path, expected); err != nil {
			t.Errorf("verifyPatchHash() with SHA256 returned error: %v", err)
		}
	})

	t.Run("MD5 match", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "patch.patch")
		os.WriteFile(path, content, 0644)

		h := md5.Sum(content)
		expected := hex.EncodeToString(h[:])

		if err := verifyPatchHash(path, expected); err != nil {
			t.Errorf("verifyPatchHash() with MD5 returned error: %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "patch.patch")
		os.WriteFile(path, content, 0644)

		err := verifyPatchHash(path, "0000000000000000000000000000000000000000000000000000000000000000")
		if err == nil {
			t.Error("verifyPatchHash() should have returned error on mismatch")
		}
	})
}

func TestResolveAndVerifyPatches_LocalSHA256(t *testing.T) {
	dir := t.TempDir()
	content := []byte("local patch content")
	patchPath := filepath.Join(dir, "fix.patch")
	os.WriteFile(patchPath, content, 0644)

	h := sha256.Sum256(content)
	sha := hex.EncodeToString(h[:])

	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte(""), 0644)

	patches := []PatchSource{
		{Path: "fix.patch", SHA256: sha},
	}

	result, err := ResolveAndVerifyPatches(manifestPath, patches, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatalf("ResolveAndVerifyPatches() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0] != patchPath {
		t.Errorf("result path = %q, want %q", result[0], patchPath)
	}
}

func TestResolveAndVerifyPatches_LocalMD5(t *testing.T) {
	dir := t.TempDir()
	content := []byte("local patch with md5")
	patchPath := filepath.Join(dir, "fix.patch")
	os.WriteFile(patchPath, content, 0644)

	h := md5.Sum(content)
	md5sum := hex.EncodeToString(h[:])

	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte(""), 0644)

	patches := []PatchSource{
		{Path: "fix.patch", MD5: md5sum},
	}

	result, err := ResolveAndVerifyPatches(manifestPath, patches, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatalf("ResolveAndVerifyPatches() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0] != patchPath {
		t.Errorf("result path = %q, want %q", result[0], patchPath)
	}
}

func TestResolveAndVerifyPatches_NoChecksum(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte(""), 0644)

	patches := []PatchSource{
		{Path: "fix.patch"},
	}

	_, err := ResolveAndVerifyPatches(manifestPath, patches, filepath.Join(dir, "cache"))
	if err == nil {
		t.Error("expected error for patch with no checksum")
	}
}

func TestResolveAndVerifyPatches_MissingFile(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte(""), 0644)

	patches := []PatchSource{
		{Path: "nonexistent.patch", SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
	}

	_, err := ResolveAndVerifyPatches(manifestPath, patches, filepath.Join(dir, "cache"))
	if err == nil {
		t.Error("expected error for missing local patch file")
	}
}

func TestResolveAndVerifyPatches_NoSource(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte(""), 0644)

	patches := []PatchSource{
		{SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
	}

	_, err := ResolveAndVerifyPatches(manifestPath, patches, filepath.Join(dir, "cache"))
	if err == nil {
		t.Error("expected error for patch with neither path nor url")
	}
}
