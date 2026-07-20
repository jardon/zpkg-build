package plugin

type PluginSource struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Source  string `yaml:"source"`
	SHA256  string `yaml:"sha256"`
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
	GetDefaultBuildSteps() []string
	GetDefaultInstallSteps() []string
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
