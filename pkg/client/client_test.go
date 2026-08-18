package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jardon/zpkg-build/pkg/manifest"
	"github.com/jardon/zpkg-build/pkg/plugin"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "package.yaml")
	os.WriteFile(manifestPath, []byte(`
name: test-pkg
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
licenses:
  - name: "MIT"
`), 0644)

	c, err := New(manifestPath, Options{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if c == nil {
		t.Fatal("expected non-nil client")
	}

	m := c.Manifest()
	if m == nil {
		t.Fatal("expected non-nil manifest from Manifest()")
	}
	if m.Name != "test-pkg" {
		t.Errorf("manifest.Name = %q, want %q", m.Name, "test-pkg")
	}
}

func TestNew_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "bad.yaml")
	os.WriteFile(manifestPath, []byte(`{{{invalid yaml`), 0644)

	_, err := New(manifestPath, Options{})
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

func TestNewWithManifest(t *testing.T) {
	m := &manifest.RecipeManifest{
		Name:    "programmatic",
		Version: "2.0",
		Arch:    "amd64",
		Source: manifest.SourceBlock{
			URL:    "https://example.com/src.tar.gz",
			SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		Engine: "podman",
		Base:   "alpine:3.23@sha256:abc123",
		Plugin: plugin.PluginSource{Name: "none"},
		Licenses: []manifest.License{{Name: "MIT"}},
	}

	rawRecipe := map[string]interface{}{
		"name":    "programmatic",
		"version": "2.0",
		"base":    "alpine:3.23@sha256:abc123",
		"plugin":  map[string]interface{}{"name": "none"},
		"source":  map[string]interface{}{"url": "https://example.com/src.tar.gz", "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
	}
	hash := "abc123def456"

	c, err := NewWithManifest(t.TempDir(), m, rawRecipe, hash, Options{
		ExportFormat: "tar.gz",
		NoArchive:    true,
	})
	if err != nil {
		t.Fatalf("NewWithManifest() error: %v", err)
	}

	if c.Manifest().Name != "programmatic" {
		t.Errorf("manifest.Name = %q, want %q", c.Manifest().Name, "programmatic")
	}

	if c.RecipeHash() != hash {
		t.Errorf("RecipeHash() = %q, want %q", c.RecipeHash(), hash)
	}

	reprod := c.Reproducibility()
	if !reprod.Deterministic {
		t.Errorf("expected deterministic manifest, got warnings: %v", reprod.Warnings)
	}
}

func TestNewWithManifest_EmptyBaseDir(t *testing.T) {
	m := &manifest.RecipeManifest{
		Name:    "test",
		Version: "1.0",
		Arch:    "amd64",
		Engine:  "podman",
		Base:    "alpine:3.23@sha256:abc",
		Plugin:  plugin.PluginSource{Name: "none"},
	}

	c, err := NewWithManifest("", m, nil, "hash", Options{})
	if err != nil {
		t.Fatalf("NewWithManifest() with empty baseDir error: %v", err)
	}
	if c.Manifest().Name != "test" {
		t.Errorf("manifest.Name = %q, want %q", c.Manifest().Name, "test")
	}
}

func TestOptions(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "package.yaml")
	os.WriteFile(manifestPath, []byte(`
name: opts-test
version: "1.0"
arch: amd64
source:
  url: "https://example.com/src.tar.gz"
  sha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
engine: podman
base: "alpine:3.23@sha256:abc123"
plugin:
  name: none
`), 0644)

	c, err := New(manifestPath, Options{
		OutputDir:    "/tmp/output",
		ExportFormat: "tar.xz",
		NoArchive:    true,
		KeepContainer: true,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewWithManifest_NonDeterministic(t *testing.T) {
	m := &manifest.RecipeManifest{
		Name:    "nondet",
		Version: "1.0",
		Arch:    "amd64",
		Engine:  "podman",
		Base:    "alpine:3.23",
		Plugin:  plugin.PluginSource{Name: "none"},
	}

	rawRecipe := map[string]interface{}{
		"base": "alpine:3.23",
	}

	c, err := NewWithManifest(t.TempDir(), m, rawRecipe, "hash", Options{})
	if err != nil {
		t.Fatalf("NewWithManifest() error: %v", err)
	}

	reprod := c.Reproducibility()
	if reprod.Deterministic {
		t.Error("expected non-deterministic manifest (unpinned base)")
	}
	if len(reprod.Warnings) == 0 {
		t.Error("expected warnings for non-deterministic manifest")
	}
}
