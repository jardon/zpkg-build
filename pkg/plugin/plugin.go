package plugin

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type PluginSource struct {
	Name    string   `yaml:"name"`
	Version string   `yaml:"version"`
	Source  string   `yaml:"source"`
	SHA256  string   `yaml:"sha256"`
	Args    []string `json:"args,omitempty"`
}

func (s *PluginSource) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Name    string                 `yaml:"name"`
		Version string                 `yaml:"version"`
		Source  string                 `yaml:"source"`
		SHA256  string                 `yaml:"sha256"`
		Rest    map[string]interface{} `yaml:",inline"`
	}

	if err := node.Decode(&raw); err != nil {
		return err
	}

	s.Name = raw.Name
	s.Version = raw.Version
	s.Source = raw.Source
	s.SHA256 = raw.SHA256

	key := ArgsKeyForPlugin(raw.Name)
	if key == "" {
		return nil
	}

	rawVal, ok := raw.Rest[key]
	if !ok || rawVal == nil {
		return nil
	}

	items, ok := rawVal.([]interface{})
	if !ok {
		return fmt.Errorf("plugin %q %s must be a list of strings", raw.Name, key)
	}

	for _, item := range items {
		str, ok := item.(string)
		if !ok {
			return fmt.Errorf("plugin %q %s must be a list of strings", raw.Name, key)
		}
		s.Args = append(s.Args, str)
	}

	return nil
}

func ArgsKeyForPlugin(name string) string {
	switch name {
	case "golang":
		return "go-build-args"
	case "rust":
		return "cargo-build-args"
	case "cmake":
		return "cmake-config-args"
	case "make":
		return "make-args"
	case "autotools":
		return "configure-args"
	case "meson":
		return "meson-args"
	case "maven":
		return "maven-args"
	case "poetry":
		return "poetry-args"
	default:
		return ""
	}
}

var shellMetacharacters = []string{
	"`", "$(", "|", ";", ">", "<", "&&", "||", "&", "\n",
}

var networkCommands = []string{
	"curl ", "wget ", "git clone ", "npm install ", "pip install ", "cargo install ",
}

func ValidateArgs(pluginName string, args []string) error {
	if len(args) == 0 {
		return nil
	}

	if ArgsKeyForPlugin(pluginName) == "" {
		return fmt.Errorf("plugin %q does not support build argument overrides", pluginName)
	}

	joined := strings.Join(args, " ")
	joinedLower := strings.ToLower(joined)

	for _, mc := range shellMetacharacters {
		if strings.Contains(joinedLower, mc) {
			return fmt.Errorf("build args for plugin %q contain shell metacharacter %q", pluginName, mc)
		}
	}

	for _, nc := range networkCommands {
		if strings.Contains(joinedLower, nc) {
			return fmt.Errorf("build args for plugin %q contain network command %q", pluginName, nc)
		}
	}

	return nil
}

func commandWithArgs(prefix string, args []string, defaultArgs string) string {
	argsLine := defaultArgs
	if len(args) > 0 {
		argsLine = strings.Join(args, " ")
	}
	if argsLine == "" {
		return prefix
	}
	return prefix + " " + argsLine
}

type PackageCache struct {
	HostSubdir string
	GuestPath  string
}

type Plugin interface {
	Name() string
	Version() string
	GetExtractPath() string
	GetPostExtractSteps() []string
	GetEnvVars() map[string]string
	GetCacheDirectories() []PackageCache
	GetBuildCommands() []string
}

func GetPlugin(source PluginSource) Plugin {
	switch source.Name {
	case "golang":
		return &GoPlugin{source: source}
	case "rust":
		return &RustPlugin{source: source}
	case "cmake":
		return &CMakePlugin{source: source}
	case "make":
		return &MakePlugin{source: source}
	case "autotools":
		return &AutotoolsPlugin{source: source}
	case "poetry":
		return &PoetryPlugin{source: source}
	case "maven":
		return &MavenPlugin{source: source}
	case "meson":
		return &MesonPlugin{source: source}
	case "none", "":
		return &NoOpPlugin{source: source}
	default:
		return &NoOpPlugin{source: source}
	}
}
