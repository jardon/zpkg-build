package plugin

type GenericPlugin struct {
	source PluginSource
}

func (p *GenericPlugin) Name() string                             { return p.source.Name }
func (p *GenericPlugin) Version() string                          { return p.source.Version }
func (p *GenericPlugin) GetInstallScripts(baseOS string) []string { return nil }
func (p *GenericPlugin) GetEnvVars() map[string]string            { return nil }
func (p *GenericPlugin) GetCacheDirectories() []PackageCache      { return nil }
func (p *GenericPlugin) GetDefaultBuildSteps() []string           { return nil }
func (p *GenericPlugin) GetDefaultInstallSteps() []string         { return nil }
