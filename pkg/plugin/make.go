package plugin

type MakePlugin struct {
	source PluginSource
}

func (p *MakePlugin) Name() string                             { return "make" }
func (p *MakePlugin) Version() string                          { return p.source.Version }
func (p *MakePlugin) GetInstallScripts(archivePath string) []string { return nil }
func (p *MakePlugin) GetEnvVars() map[string]string            { return nil }
func (p *MakePlugin) GetCacheDirectories() []PackageCache      { return nil }

func (p *MakePlugin) GetDefaultBuildSteps() []string {
	return []string{"make"}
}

func (p *MakePlugin) GetDefaultInstallSteps() []string {
	return []string{"make install DESTDIR=$ZPKG_DEST"}
}
