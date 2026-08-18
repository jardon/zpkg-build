package builder

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/jardon/zpkg-build/pkg/manifest"
)

func TestDepChecksum(t *testing.T) {
	tests := []struct {
		name   string
		dep    manifest.Dependency
		expect string
	}{
		{
			name:   "SHA256 preferred",
			dep:    manifest.Dependency{SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
			expect: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:   "falls back to MD5",
			dep:    manifest.Dependency{MD5: "abcdef0123456789abcdef0123456789"},
			expect: "abcdef0123456789abcdef0123456789",
		},
		{
			name:   "empty when neither set",
			dep:    manifest.Dependency{},
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := depChecksum(tt.dep); got != tt.expect {
				t.Errorf("depChecksum() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestSanitizeLicenseName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"simple", "MIT", "MIT"},
		{"with spaces", "Apache 2.0", "Apache_2.0"},
		{"special chars", "BSD/3-Clause!", "BSD_3-Clause_"},
		{"preserved chars", "GPL-3.0-or-later", "GPL-3.0-or-later"},
		{"empty returns default", "", "LICENSE"},
		{"only special chars becomes underscores", "!@#", "___"},
		{"dots preserved", "MIT.with.dots", "MIT.with.dots"},
		{"underscores preserved", "MIT_with_underscores", "MIT_with_underscores"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeLicenseName(tt.input); got != tt.expect {
				t.Errorf("sanitizeLicenseName(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestNormalizePatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"strips leading slash", []string{"/usr/bin", "/etc"}, []string{"usr/bin", "etc"}},
		{"no leading slash unchanged", []string{"usr/bin"}, []string{"usr/bin"}},
		{"mixed", []string{"/a", "b", "/c/d"}, []string{"a", "b", "c/d"}},
		{"empty", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePatterns(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("got %d items, want %d", len(got), len(tt.expect))
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("normalizePatterns()[%d] = %q, want %q", i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestMatchesAnyInclude(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		patterns []string
		expect   bool
	}{
		{"exact match", "usr/bin/app", []string{"usr/bin/app"}, true},
		{"glob match", "usr/bin/app", []string{"usr/bin/*"}, true},
		{"double star", "usr/bin/app", []string{"usr/**"}, true},
		{"no match", "etc/passwd", []string{"usr/bin/*"}, false},
		{"empty patterns", "usr/bin/app", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesAnyInclude(tt.relPath, tt.patterns); got != tt.expect {
				t.Errorf("matchesAnyInclude(%q, %v) = %v, want %v", tt.relPath, tt.patterns, got, tt.expect)
			}
		})
	}
}

func TestMatchesAnyExclude(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		patterns []string
		expect   bool
	}{
		{"match", "usr/bin/debug", []string{"usr/bin/debug"}, true},
		{"glob match", "usr/bin/debug", []string{"usr/bin/*"}, true},
		{"no match", "usr/bin/app", []string{"usr/bin/debug"}, false},
		{"empty patterns", "usr/bin/app", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesAnyExclude(tt.relPath, tt.patterns); got != tt.expect {
				t.Errorf("matchesAnyExclude(%q, %v) = %v, want %v", tt.relPath, tt.patterns, got, tt.expect)
			}
		})
	}
}

func TestIsIncludePrefix(t *testing.T) {
	tests := []struct {
		name     string
		dirPath  string
		patterns []string
		expect   bool
	}{
		{"has child", "usr/bin", []string{"usr/bin/app"}, true},
		{"no child", "usr/bin", []string{"usr/lib/app"}, false},
		{"exact match not prefix", "usr/bin", []string{"usr/bin"}, false},
		{"empty patterns", "usr/bin", nil, false},
		{"deeper child", "usr", []string{"usr/bin/app"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIncludePrefix(tt.dirPath, tt.patterns); got != tt.expect {
				t.Errorf("isIncludePrefix(%q, %v) = %v, want %v", tt.dirPath, tt.patterns, got, tt.expect)
			}
		})
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		pattern string
		expect  bool
	}{
		{"exact match", "usr/bin/app", "usr/bin/app", true},
		{"single star", "usr/bin/app", "usr/bin/*", true},
		{"double star", "usr/bin/debug/app", "usr/**", true},
		{"double star nested", "a/b/c/d", "a/**/d", true},
		{"no match", "usr/bin/app", "etc/*", false},
		{"partial name", "usr/bin/app", "usr/bin/ap", false},
		{"star matches segment", "usr/bin/app", "usr/*/app", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchPath(tt.relPath, tt.pattern); got != tt.expect {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.relPath, tt.pattern, got, tt.expect)
			}
		})
	}
}

func TestMatchParts(t *testing.T) {
	t.Run("both empty", func(t *testing.T) {
		if !matchParts(nil, nil) {
			t.Error("expected true for both empty")
		}
	})

	t.Run("pattern empty path not", func(t *testing.T) {
		if matchParts([]string{"a"}, nil) {
			t.Error("expected false")
		}
	})

	t.Run("path empty pattern not", func(t *testing.T) {
		if matchParts(nil, []string{"a"}) {
			t.Error("expected false")
		}
	})
}

func TestDebugExecHint(t *testing.T) {
	tests := []struct {
		engine string
		id     string
		expect string
	}{
		{"podman", "abc123", "podman exec -it abc123 /bin/sh"},
		{"docker", "def456", "docker exec -it def456 /bin/sh"},
		{"lxc", "mycontainer", "lxc-attach -n mycontainer"},
		{"chroot", "/mnt/rootfs", "sudo chroot /mnt/rootfs /bin/sh"},
		{"unknown", "abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			if got := debugExecHint(tt.engine, tt.id); got != tt.expect {
				t.Errorf("debugExecHint(%q, %q) = %q, want %q", tt.engine, tt.id, got, tt.expect)
			}
		})
	}
}

func TestVerifyTarballHash(t *testing.T) {
	content := []byte("test tarball content")

	t.Run("SHA256 match", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.tar.gz")
		os.WriteFile(path, content, 0644)

		h := sha256.Sum256(content)
		expected := hex.EncodeToString(h[:])

		if err := verifyTarballHash(path, expected); err != nil {
			t.Errorf("verifyTarballHash() with SHA256 returned error: %v", err)
		}
	})

	t.Run("MD5 match", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.tar.gz")
		os.WriteFile(path, content, 0644)

		h := md5.Sum(content)
		expected := hex.EncodeToString(h[:])

		if err := verifyTarballHash(path, expected); err != nil {
			t.Errorf("verifyTarballHash() with MD5 returned error: %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.tar.gz")
		os.WriteFile(path, content, 0644)

		err := verifyTarballHash(path, "0000000000000000000000000000000000000000000000000000000000000000")
		if err == nil {
			t.Error("expected error on hash mismatch")
		}
	})

	t.Run("unsupported length", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.tar.gz")
		os.WriteFile(path, content, 0644)

		err := verifyTarballHash(path, "abc123")
		if err == nil {
			t.Error("expected error for unsupported hash length")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		err := verifyTarballHash("/nonexistent/file", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}
