package plugin

type CMakePlugin struct {
	source PluginSource
}

func (p *CMakePlugin) Name() string    { return "cmake" }
func (p *CMakePlugin) Version() string { return p.source.Version }

func (p *CMakePlugin) GetExtractPath() string {
	return "/usr/local"
}

func (p *CMakePlugin) GetPostExtractSteps() []string {
	return nil
}

func (p *CMakePlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH": "/usr/local/cmake/bin:$PATH",
	}
}

func (p *CMakePlugin) GetCacheDirectories() []PackageCache {
	return nil
}

func (p *CMakePlugin) GetBuildCommands() map[string]BuildCommand {
	return map[string]BuildCommand{
		"configure": {Command: "cmake", DefaultArgs: "-B build -S ."},
		"build":     {Command: "cmake --build", DefaultArgs: "build"},
		"install":   {Command: "DESTDIR=$ZPKG_DEST cmake --install", DefaultArgs: "build"},
	}
}
