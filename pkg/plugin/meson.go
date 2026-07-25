package plugin

type MesonPlugin struct {
	source PluginSource
}

func (p *MesonPlugin) Name() string    { return "meson" }
func (p *MesonPlugin) Version() string { return p.source.Version }

func (p *MesonPlugin) GetExtractPath() string {
	return "/usr/local"
}

func (p *MesonPlugin) GetPostExtractSteps() []string {
	return nil
}

func (p *MesonPlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH": "/usr/local/bin:/usr/local/meson/bin:$PATH",
	}
}

func (p *MesonPlugin) GetCacheDirectories() []PackageCache {
	return nil
}

func (p *MesonPlugin) GetBuildCommands() map[string]BuildCommand {
	return map[string]BuildCommand{
		"configure": {Command: "meson", DefaultArgs: "setup build"},
		"build":     {Command: "ninja", DefaultArgs: "-C build"},
		"install":   {Command: "DESTDIR=$ZPKG_DEST ninja", DefaultArgs: "-C build install"},
	}
}
