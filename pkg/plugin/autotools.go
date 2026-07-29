package plugin

type AutotoolsPlugin struct {
	source PluginSource
}

func (p *AutotoolsPlugin) Name() string    { return "autotools" }
func (p *AutotoolsPlugin) Version() string { return p.source.Version }

func (p *AutotoolsPlugin) GetExtractPath() string       { return "" }
func (p *AutotoolsPlugin) GetPostExtractSteps() []string { return nil }
func (p *AutotoolsPlugin) GetEnvVars() map[string]string { return nil }
func (p *AutotoolsPlugin) GetCacheDirectories() []PackageCache { return nil }

func (p *AutotoolsPlugin) GetBuildCommands() []string {
	return []string{
		"./configure --prefix=/usr",
		"make",
		"make install DESTDIR=$ZPKG_DEST",
	}
}
