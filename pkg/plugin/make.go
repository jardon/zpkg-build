package plugin

type MakePlugin struct {
	source PluginSource
}

func (p *MakePlugin) Name() string                             { return "make" }
func (p *MakePlugin) Version() string                          { return p.source.Version }
func (p *MakePlugin) GetExtractPath() string              { return "" }
func (p *MakePlugin) GetPostExtractSteps() []string        { return nil }
func (p *MakePlugin) GetEnvVars() map[string]string        { return nil }
func (p *MakePlugin) GetCacheDirectories() []PackageCache  { return nil }

func (p *MakePlugin) GetBuildCommands() []string {
	return []string{
		commandWithArgs("make", p.source.Args, ""),
		"make install DESTDIR=$ZPKG_DEST",
	}
}
