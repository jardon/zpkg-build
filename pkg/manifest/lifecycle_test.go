package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeHydratedRecipeHash(t *testing.T) {
	t.Run("deterministic hashing", func(t *testing.T) {
		m1 := map[string]interface{}{"name": "test", "version": "1.0"}
		m2 := map[string]interface{}{"name": "test", "version": "1.0"}

		h1, err := ComputeHydratedRecipeHash(m1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		h2, err := ComputeHydratedRecipeHash(m2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h1 != h2 {
			t.Errorf("same input produced different hashes: %q vs %q", h1, h2)
		}
		if len(h1) != 64 {
			t.Errorf("expected 64-char hex hash, got %d chars", len(h1))
		}
	})

	t.Run("different input produces different hash", func(t *testing.T) {
		m1 := map[string]interface{}{"name": "test", "version": "1.0"}
		m2 := map[string]interface{}{"name": "test", "version": "2.0"}

		h1, _ := ComputeHydratedRecipeHash(m1)
		h2, _ := ComputeHydratedRecipeHash(m2)
		if h1 == h2 {
			t.Error("different inputs produced same hash")
		}
	})
}

func TestLoadManifestRaw(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid yaml", func(t *testing.T) {
		path := filepath.Join(dir, "test.yaml")
		os.WriteFile(path, []byte("name: test\nversion: 1.0\n"), 0644)

		raw, err := LoadManifestRaw(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if raw["name"] != "test" {
			t.Errorf("expected name=test, got %v", raw["name"])
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadManifestRaw(filepath.Join(dir, "nonexistent.yaml"))
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func TestLoadAndHydrateManifest(t *testing.T) {
	dir := t.TempDir()

	writeManifest := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(dir, "manifest.yaml")
		os.WriteFile(path, []byte(content), 0644)
		return path
	}

	t.Run("valid minimal manifest", func(t *testing.T) {
		path := writeManifest(t, `
name: test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
`)
		manifest, hash, rawMap, err := LoadAndHydrateManifest(path, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if manifest.Name != "test" {
			t.Errorf("expected name=test, got %q", manifest.Name)
		}
		if len(hash) != 64 {
			t.Errorf("expected 64-char hash, got %d chars", len(hash))
		}
		if rawMap == nil {
			t.Error("expected non-nil rawMap")
		}
	})

	t.Run("rename with slash rejected", func(t *testing.T) {
		path := writeManifest(t, `
name: test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
build_deps:
  - name: gmp
    source: "https://example.com/gmp.tar.gz"
    sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
    extract-to: "/usr"
    rename: "foo/bar"
`)
		_, _, _, err := LoadAndHydrateManifest(path, nil)
		if err == nil {
			t.Error("expected error for rename with slash")
		}
		if err != nil && !strings.Contains(err.Error(), "path separators") {
			t.Errorf("expected error about path separators, got: %v", err)
		}
	})

	t.Run("rename valid", func(t *testing.T) {
		path := writeManifest(t, `
name: test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
build_deps:
  - name: gmp
    source: "https://example.com/gmp.tar.gz"
    sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
    extract-to: "/usr"
    rename: "gmp"
`)
		manifest, _, _, err := LoadAndHydrateManifest(path, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if manifest.BuildDeps[0].Rename != "gmp" {
			t.Errorf("expected rename=gmp, got %q", manifest.BuildDeps[0].Rename)
		}
	})

	t.Run("build dep with md5", func(t *testing.T) {
		path := writeManifest(t, `
name: test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
build_deps:
  - name: gmp
    source: "https://example.com/gmp.tar.gz"
    md5: "abcdef0123456789abcdef0123456789"
`)
		manifest, _, _, err := LoadAndHydrateManifest(path, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if manifest.BuildDeps[0].MD5 != "abcdef0123456789abcdef0123456789" {
			t.Errorf("expected md5 on build dep")
		}
	})

	t.Run("build dep missing checksum rejected", func(t *testing.T) {
		path := writeManifest(t, `
name: test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
build_deps:
  - name: gmp
    source: "https://example.com/gmp.tar.gz"
`)
		_, _, _, err := LoadAndHydrateManifest(path, nil)
		if err == nil {
			t.Error("expected error for build dep with no checksum")
		}
	})

	t.Run("relative extract-to rejected", func(t *testing.T) {
		path := writeManifest(t, `
name: test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
build_deps:
  - name: gmp
    source: "https://example.com/gmp.tar.gz"
    sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
    extract-to: "relative/path"
`)
		_, _, _, err := LoadAndHydrateManifest(path, nil)
		if err == nil {
			t.Error("expected error for relative extract-to")
		}
	})
}
