package manifest

import (
	"github.com/jardon/zpkg-build/pkg/plugin"
)

type Dependency struct {
	Name      string `yaml:"name"`
	Min       string `yaml:"min,omitempty"`
	Max       string `yaml:"max,omitempty"`
	Ver       string `yaml:"version,omitempty"`
	Source    string `yaml:"source,omitempty"`
	SHA256    string `yaml:"sha256,omitempty"`
	ExtractTo string `yaml:"extract-to,omitempty"`
}

type PatchSource struct {
	Path   string `yaml:"path,omitempty"`
	URL    string `yaml:"url,omitempty"`
	SHA256 string `yaml:"sha256"`
}

type License struct {
	Name   string `yaml:"name"`
	File   string `yaml:"file,omitempty"`
	URL    string `yaml:"url,omitempty"`
	SHA256 string `yaml:"sha256,omitempty"`
}

type SourceBlock struct {
	Path    string        `yaml:"path,omitempty"`
	Git     string        `yaml:"git,omitempty"`
	URL     string        `yaml:"url,omitempty"`
	SHA256  string        `yaml:"sha256,omitempty"`
	MD5     string        `yaml:"md5,omitempty" json:",omitempty"`
	Ref     string        `yaml:"ref,omitempty"`
	Patches []PatchSource `yaml:"patches,omitempty"`
}

type BuildBlock struct {
	Env map[string]string `yaml:"env,omitempty"`
}

type RecipeManifest struct {
	Name         string              `yaml:"name"`
	Version      string              `yaml:"version"`
	Arch         string              `yaml:"arch"`
	License      string              `yaml:"license,omitempty"`
	Licenses     []License           `yaml:"licenses,omitempty"`
	Source       SourceBlock         `yaml:"source"`
	Engine       string              `yaml:"engine"`
	Base         string              `yaml:"base"`
	Plugin       plugin.PluginSource `yaml:"plugin"`
	Build        BuildBlock          `yaml:"build"`
	BuildDeps    []Dependency        `yaml:"build_deps,omitempty"`
	RuntimeDeps  []Dependency        `yaml:"runtime_deps,omitempty"`
	Package      map[string][]string `yaml:"package"`
}

func (m *RecipeManifest) EffectiveLicenses() []License {
	if len(m.Licenses) > 0 {
		return m.Licenses
	}
	if m.License != "" {
		return []License{{Name: m.License}}
	}
	return nil
}
