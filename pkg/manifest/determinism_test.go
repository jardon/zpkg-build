package manifest

import (
	"strings"
	"testing"
)

func TestIsCommitSHA(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"valid lowercase", "abcdef0123456789abcdef0123456789abcdef01", true},
		{"valid uppercase", "ABCDEF0123456789ABCDEF0123456789ABCDEF01", true},
		{"valid mixed case", "aAbBcCdD0011223344556677889900aAbBcCdD00", true},
		{"too short", "abc123", false},
		{"too long", "abcdef0123456789abcdef0123456789abcdef0100", false},
		{"empty", "", false},
		{"non-hex chars", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", false},
		{"contains dash", "abcdef01-3456-789a-bcde-f01234567890abcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCommitSHA(tt.input); got != tt.expect {
				t.Errorf("IsCommitSHA(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestAnalyzeReproducibility(t *testing.T) {
	t.Run("fully deterministic", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base": "alpine:3.23@sha256:abc123",
			"plugin": map[string]interface{}{
				"name":   "golang",
				"source": "https://go.dev/dl/go1.20.tar.gz",
				"sha256": strings.Repeat("a", 64),
			},
			"source": map[string]interface{}{
				"url":    "https://example.com/src.tar.gz",
				"sha256": strings.Repeat("b", 64),
			},
		}
		result := AnalyzeReproducibility(recipe)
		if !result.Deterministic {
			t.Errorf("expected deterministic, got warnings: %v", result.Warnings)
		}
	})

	t.Run("unpinned base image", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"url": "x", "sha256": strings.Repeat("b", 64)},
		}
		result := AnalyzeReproducibility(recipe)
		if result.Deterministic {
			t.Error("expected non-deterministic for unpinned base")
		}
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "not pinned") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about unpinned base image")
		}
	})

	t.Run("missing plugin", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"source": map[string]interface{}{},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "No toolchain plugin") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about missing plugin")
		}
	})

	t.Run("local source path", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"path": "/home/user/src"},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "Local source path") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about local source path")
		}
	})

	t.Run("git without ref", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"git": "https://github.com/x/y.git"},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "no pinned ref") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about git without ref")
		}
	})

	t.Run("git with non-commit ref", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"git": "https://github.com/x/y.git", "ref": "main"},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "not a commit SHA") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about non-commit ref")
		}
	})

	t.Run("url source without checksum", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"url": "https://example.com/src.tar.gz"},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "no SHA-256 or MD5 verification") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about url without checksum")
		}
	})

	t.Run("url source with md5 is fine", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"url": "https://example.com/src.tar.gz", "md5": strings.Repeat("b", 32)},
		}
		result := AnalyzeReproducibility(recipe)
		for _, w := range result.Warnings {
			if strings.Contains(w, "no SHA-256 or MD5 verification") {
				t.Error("should not warn when md5 is present")
			}
		}
	})

	t.Run("patch without checksum", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{
				"url":    "https://example.com/src.tar.gz",
				"sha256": strings.Repeat("b", 64),
				"patches": []interface{}{
					map[string]interface{}{"url": "https://example.com/fix.patch"},
				},
			},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "Patch [0]") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about patch without checksum")
		}
	})

	t.Run("patch with md5 is fine", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{
				"url":    "https://example.com/src.tar.gz",
				"sha256": strings.Repeat("b", 64),
				"patches": []interface{}{
					map[string]interface{}{"url": "https://example.com/fix.patch", "md5": strings.Repeat("c", 32)},
				},
			},
		}
		result := AnalyzeReproducibility(recipe)
		for _, w := range result.Warnings {
			if strings.Contains(w, "Patch [0]") {
				t.Error("should not warn when patch has md5")
			}
		}
	})

	t.Run("local license file", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"url": "x", "sha256": strings.Repeat("b", 64)},
			"licenses": []interface{}{
				map[string]interface{}{"name": "Custom", "file": "custom.txt"},
			},
		}
		result := AnalyzeReproducibility(recipe)
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "License [0]") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about local license file")
		}
	})

	t.Run("override-steps present", func(t *testing.T) {
		recipe := map[string]interface{}{
			"base":   "alpine:3.23@sha256:abc",
			"plugin": map[string]interface{}{"name": "golang", "source": "x", "sha256": strings.Repeat("a", 64)},
			"source": map[string]interface{}{"url": "x", "sha256": strings.Repeat("b", 64)},
			"build": map[string]interface{}{
				"override-steps": "make\nmake install",
			},
		}
		result := AnalyzeReproducibility(recipe)
		if result.Deterministic {
			t.Error("expected non-deterministic for override-steps")
		}
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "override-steps") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning about override-steps")
		}
	})
}
