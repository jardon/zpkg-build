package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSPDX(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"MIT", "MIT", true},
		{"Apache-2.0", "Apache-2.0", true},
		{"GPL-3.0-or-later", "GPL-3.0-or-later", true},
		{"ISC", "ISC", true},
		{"BSD-3-Clause", "BSD-3-Clause", true},
		{"unknown", "MyCustomLicense", false},
		{"empty", "", false},
		{"case sensitive", "mit", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSPDX(tt.input); got != tt.expect {
				t.Errorf("IsSPDX(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestEffectiveLicenses(t *testing.T) {
	t.Run("licenses list takes precedence", func(t *testing.T) {
		m := &RecipeManifest{
			License:  "MIT",
			Licenses: []License{{Name: "Apache-2.0"}, {Name: "BSD-3-Clause"}},
		}
		got := m.EffectiveLicenses()
		if len(got) != 2 || got[0].Name != "Apache-2.0" || got[1].Name != "BSD-3-Clause" {
			t.Errorf("expected [Apache-2.0, BSD-3-Clause], got %v", got)
		}
	})

	t.Run("legacy license string", func(t *testing.T) {
		m := &RecipeManifest{License: "MIT"}
		got := m.EffectiveLicenses()
		if len(got) != 1 || got[0].Name != "MIT" {
			t.Errorf("expected [MIT], got %v", got)
		}
	})

	t.Run("both empty returns nil", func(t *testing.T) {
		m := &RecipeManifest{}
		if got := m.EffectiveLicenses(); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestLicenseWarnings(t *testing.T) {
	t.Run("non-SPDX without file or url warns", func(t *testing.T) {
		m := &RecipeManifest{
			Licenses: []License{{Name: "CustomProprietary"}},
		}
		warnings := m.LicenseWarnings()
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
	})

	t.Run("SPDX name no warning", func(t *testing.T) {
		m := &RecipeManifest{
			Licenses: []License{{Name: "MIT"}},
		}
		if got := m.LicenseWarnings(); len(got) != 0 {
			t.Errorf("expected 0 warnings, got %d", len(got))
		}
	})

	t.Run("non-SPDX with file no warning", func(t *testing.T) {
		m := &RecipeManifest{
			Licenses: []License{{Name: "Custom", File: "custom.txt"}},
		}
		if got := m.LicenseWarnings(); len(got) != 0 {
			t.Errorf("expected 0 warnings, got %d", len(got))
		}
	})

	t.Run("non-SPDX with url no warning", func(t *testing.T) {
		m := &RecipeManifest{
			Licenses: []License{{Name: "Custom", URL: "https://example.com/license"}},
		}
		if got := m.LicenseWarnings(); len(got) != 0 {
			t.Errorf("expected 0 warnings, got %d", len(got))
		}
	})
}

func TestValidateLicenses(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid identifier only", func(t *testing.T) {
		m := &RecipeManifest{Licenses: []License{{Name: "MIT"}}}
		if err := m.ValidateLicenses(dir); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid with local file", func(t *testing.T) {
		licFile := filepath.Join(dir, "custom.txt")
		os.WriteFile(licFile, []byte("license text"), 0644)
		m := &RecipeManifest{Licenses: []License{{Name: "Custom", File: "custom.txt"}}}
		if err := m.ValidateLicenses(dir); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		m := &RecipeManifest{Licenses: []License{{}}}
		if err := m.ValidateLicenses(dir); err == nil {
			t.Error("expected error for missing license name")
		}
	})

	t.Run("both file and url", func(t *testing.T) {
		m := &RecipeManifest{Licenses: []License{{Name: "X", File: "f.txt", URL: "http://x"}}}
		if err := m.ValidateLicenses(dir); err == nil {
			t.Error("expected error when both file and url set")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		m := &RecipeManifest{Licenses: []License{{Name: "X", File: "nonexistent.txt"}}}
		if err := m.ValidateLicenses(dir); err == nil {
			t.Error("expected error for missing license file")
		}
	})

	t.Run("bad sha256 length", func(t *testing.T) {
		m := &RecipeManifest{Licenses: []License{{Name: "X", SHA256: "tooshort"}}}
		if err := m.ValidateLicenses(dir); err == nil {
			t.Error("expected error for bad sha256 length")
		}
	})

	t.Run("valid sha256", func(t *testing.T) {
		m := &RecipeManifest{Licenses: []License{{Name: "X", SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}}}
		if err := m.ValidateLicenses(dir); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
