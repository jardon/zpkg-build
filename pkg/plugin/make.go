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

func (p *MakePlugin) GetBuildCommands() map[string]BuildCommand {
	return map[string]BuildCommand{
		"make":    {Command: "make", DefaultArgs: ""},
		"install": {Command: "make install", DefaultArgs: "DESTDIR=$ZPKG_DEST"},
	}
}
