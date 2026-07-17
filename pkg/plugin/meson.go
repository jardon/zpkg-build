package plugin

type MesonPlugin struct {
	source PluginSource
}

func (p *MesonPlugin) Name() string    { return "meson" }
func (p *MesonPlugin) Version() string { return p.source.Version }

func (p *MesonPlugin) GetInstallScripts(baseOS string) []string {
	return []string{
		"pip install meson ninja",
	}
}

func (p *MesonPlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH": "/usr/local/bin:$PATH",
	}
}

func (p *MesonPlugin) GetCacheDirectories() []PackageCache {
	return nil
}

func (p *MesonPlugin) GetDefaultBuildSteps() []string {
	return []string{
		"meson setup build",
		"ninja -C build",
	}
}

func (p *MesonPlugin) GetDefaultInstallSteps() []string {
	return []string{
		"DESTDIR=$ZPKG_DEST ninja -C build install",
	}
}
