package engine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectCompression(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		expect CompressionType
	}{
		{"tar.gz", "archive.tar.gz", CompGzip},
		{"tgz", "archive.tgz", CompGzip},
		{"tar.xz", "archive.tar.xz", CompXz},
		{"txz", "archive.txz", CompXz},
		{"bare gz", "archive.gz", CompGzip},
		{"bare xz", "archive.xz", CompXz},
		{"no extension", "archive", CompNone},
		{"txt", "archive.txt", CompNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectCompression(tt.path); got != tt.expect {
				t.Errorf("DetectCompression(%q) = %d, want %d", tt.path, got, tt.expect)
			}
		})
	}
}

func TestStripPathComponents(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		strip  int
		expect string
	}{
		{"strip 0", "a/b/c", 0, "a/b/c"},
		{"strip 1", "a/b/c", 1, "b/c"},
		{"strip 2", "a/b/c", 2, "c"},
		{"strip all", "a/b/c", 3, ""},
		{"strip more than path", "a/b", 5, ""},
		{"strip root only", "/a/b", 1, "a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPathComponents(tt.input, tt.strip)
			if got != tt.expect {
				t.Errorf("stripPathComponents(%q, %d) = %q, want %q", tt.input, tt.strip, got, tt.expect)
			}
		})
	}
}

func createTestTarGz(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestTopLevelDir(t *testing.T) {
	t.Run("tar with top-level dir", func(t *testing.T) {
		dir := t.TempDir()
		path := createTestTarGz(t, dir, map[string]string{
			"gmp-6.3.0/configure":  "#!/bin/sh",
			"gmp-6.3.0/Makefile":   "all:",
			"gmp-6.3.0/README":     "readme",
		})

		got, err := TopLevelDir(path)
		if err != nil {
			t.Fatalf("TopLevelDir() error: %v", err)
		}
		if got != "gmp-6.3.0" {
			t.Errorf("TopLevelDir() = %q, want %q", got, "gmp-6.3.0")
		}
	})

	t.Run("tar with flat files", func(t *testing.T) {
		dir := t.TempDir()
		path := createTestTarGz(t, dir, map[string]string{
			"file.txt": "hello",
		})

		got, err := TopLevelDir(path)
		if err != nil {
			t.Fatalf("TopLevelDir() error: %v", err)
		}
		if got != "file.txt" {
			t.Errorf("TopLevelDir() = %q, want %q", got, "file.txt")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := TopLevelDir("/nonexistent/path.tar.gz")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestExtractArchive(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir, map[string]string{
		"gmp-6.3.0/configure":  "#!/bin/sh",
		"gmp-6.3.0/README":     "readme",
	})

	dest := filepath.Join(dir, "extracted")
	if err := ExtractArchive(path, dest); err != nil {
		t.Fatalf("ExtractArchive() error: %v", err)
	}

	// Should preserve top-level dir
	configurePath := filepath.Join(dest, "gmp-6.3.0", "configure")
	data, err := os.ReadFile(configurePath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(data) != "#!/bin/sh" {
		t.Errorf("file content = %q, want %q", string(data), "#!/bin/sh")
	}
}

func TestExtractArchiveStrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir, map[string]string{
		"gmp-6.3.0/configure":  "#!/bin/sh",
		"gmp-6.3.0/README":     "readme",
	})

	dest := filepath.Join(dir, "extracted")
	if err := ExtractArchiveStrip(path, dest, 1); err != nil {
		t.Fatalf("ExtractArchiveStrip() error: %v", err)
	}

	// strip=1 removes "gmp-6.3.0/" prefix
	configurePath := filepath.Join(dest, "configure")
	data, err := os.ReadFile(configurePath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(data) != "#!/bin/sh" {
		t.Errorf("file content = %q, want %q", string(data), "#!/bin/sh")
	}
}

func TestDecompressArchive(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir, map[string]string{
		"test.txt": "hello world",
	})

	reader, err := DecompressArchive(path)
	if err != nil {
		t.Fatalf("DecompressArchive() error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("failed to read decompressed data: %v", err)
	}

	// The decompressed data should be a valid tar
	tr := tar.NewReader(&buf)
	header, err := tr.Next()
	if err != nil {
		t.Fatalf("failed to read tar header: %v", err)
	}
	if header.Name != "test.txt" {
		t.Errorf("expected entry name test.txt, got %q", header.Name)
	}
}

func TestIsTarArchive(t *testing.T) {
	t.Run("valid tar.gz", func(t *testing.T) {
		dir := t.TempDir()
		path := createTestTarGz(t, dir, map[string]string{
			"test.txt": "hello",
		})
		if !isTarArchive(path) {
			t.Error("expected isTarArchive to return true for valid tar.gz")
		}
	})

	t.Run("nonexistent file with non-tar extension", func(t *testing.T) {
		if isTarArchive("/nonexistent/file.txt") {
			t.Error("expected false for nonexistent file with non-tar extension")
		}
	})

	t.Run("plain text file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "not-a-tar.txt")
		os.WriteFile(path, []byte("just text"), 0644)
		if isTarArchive(path) {
			t.Error("expected false for plain text file")
		}
	})
}

func TestExtractArchiveNonexistent(t *testing.T) {
	dir := t.TempDir()
	err := ExtractArchive("/nonexistent/file.tar.gz", filepath.Join(dir, "dest"))
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

func TestExtractArchiveStripNonexistent(t *testing.T) {
	dir := t.TempDir()
	err := ExtractArchiveStrip("/nonexistent/file.tar.gz", filepath.Join(dir, "dest"), 1)
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
}
