package packager

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jardon/zpkg-build/pkg/manifest"
	"github.com/jardon/zpkg-build/pkg/plugin"
)

func TestCalculateFileHash(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("test file content for hashing")
		path := filepath.Join(dir, "test.txt")
		os.WriteFile(path, content, 0644)

		got, err := calculateFileHash(path)
		if err != nil {
			t.Fatalf("calculateFileHash() error: %v", err)
		}

		h := sha256.Sum256(content)
		expected := hex.EncodeToString(h[:])

		if got != expected {
			t.Errorf("calculateFileHash() = %q, want %q", got, expected)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := calculateFileHash("/nonexistent/file.txt")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.txt")
		os.WriteFile(path, []byte{}, 0644)

		got, err := calculateFileHash(path)
		if err != nil {
			t.Fatalf("calculateFileHash() error: %v", err)
		}

		h := sha256.Sum256([]byte{})
		expected := hex.EncodeToString(h[:])

		if got != expected {
			t.Errorf("calculateFileHash() for empty file = %q, want %q", got, expected)
		}
	})
}

func TestGenerateMetadata(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "pkg")
	os.MkdirAll(pkgDir, 0755)

	// Create some files in the package directory
	os.WriteFile(filepath.Join(pkgDir, "app"), []byte("#!/bin/sh\necho hello"), 0755)
	os.MkdirAll(filepath.Join(pkgDir, "usr", "bin"), 0755)
	os.WriteFile(filepath.Join(pkgDir, "usr", "bin", "helper"), []byte("helper script"), 0755)

	rawRecipe := map[string]interface{}{
		"base":   "alpine:3.23@sha256:abc123",
		"plugin": map[string]interface{}{"name": "make", "source": "", "sha256": ""},
		"source": map[string]interface{}{"url": "https://example.com/src.tar.gz", "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
	}

	licenses := []manifest.License{{Name: "MIT"}}
	buildDeps := []manifest.Dependency{{Name: "make", Min: "4.0"}}
	runtimeDeps := []manifest.Dependency{{Name: "libc", Min: "2.31"}}
	toolchain := plugin.PluginSource{Name: "make", Version: "4.3"}

	err := GenerateMetadata(
		"test-pkg",
		"1.0.0",
		pkgDir,
		"recipehash123",
		rawRecipe,
		toolchain,
		licenses,
		buildDeps,
		runtimeDeps,
	)
	if err != nil {
		t.Fatalf("GenerateMetadata() error: %v", err)
	}

	// Verify metadata.json was created
	metaPath := filepath.Join(pkgDir, "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}

	var meta PackageMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}

	if meta.Name != "test-pkg" {
		t.Errorf("name = %q, want %q", meta.Name, "test-pkg")
	}
	if meta.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", meta.Version, "1.0.0")
	}
	if meta.RecipeHash != "recipehash123" {
		t.Errorf("recipe_hash = %q, want %q", meta.RecipeHash, "recipehash123")
	}
	if len(meta.Licenses) != 1 || meta.Licenses[0].Name != "MIT" {
		t.Errorf("licenses = %v, want [MIT]", meta.Licenses)
	}
	if len(meta.BuildDeps) != 1 || meta.BuildDeps[0].Name != "make" {
		t.Errorf("build_deps = %v, want [make]", meta.BuildDeps)
	}
	if len(meta.RuntimeDeps) != 1 || meta.RuntimeDeps[0].Name != "libc" {
		t.Errorf("runtime_deps = %v, want [libc]", meta.RuntimeDeps)
	}
	if meta.Plugin.Name != "make" {
		t.Errorf("plugin.name = %q, want %q", meta.Plugin.Name, "make")
	}

	// Verify file contents are listed
	if len(meta.Contents) == 0 {
		t.Fatal("expected non-empty contents list")
	}

	// Check that files have paths and hashes
	for _, c := range meta.Contents {
		if c.Path == "" {
			t.Error("file entry has empty path")
		}
		if c.Type == "file" && c.SHA256 == "" {
			t.Errorf("file %q has empty sha256", c.Path)
		}
	}

	// Verify metadata.json itself is excluded from contents
	for _, c := range meta.Contents {
		if c.Path == "/metadata.json" {
			t.Error("metadata.json should be excluded from contents")
		}
	}
}

func TestGenerateMetadata_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "empty-pkg")
	os.MkdirAll(pkgDir, 0755)

	rawRecipe := map[string]interface{}{
		"base":   "alpine:3.23@sha256:abc",
		"plugin": map[string]interface{}{"name": "none"},
	}

	err := GenerateMetadata(
		"empty",
		"0.1",
		pkgDir,
		"hash",
		rawRecipe,
		plugin.PluginSource{Name: "none"},
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("GenerateMetadata() error: %v", err)
	}

	metaPath := filepath.Join(pkgDir, "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}

	var meta PackageMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}

	if meta.Name != "empty" {
		t.Errorf("name = %q, want %q", meta.Name, "empty")
	}
}
