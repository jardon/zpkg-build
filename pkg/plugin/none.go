package plugin

type NoOpPlugin struct {
	source PluginSource
}

func (p *NoOpPlugin) Name() string                                  { return "none" }
func (p *NoOpPlugin) Version() string                               { return "" }
func (p *NoOpPlugin) GetExtractPath() string                { return "" }
func (p *NoOpPlugin) GetPostExtractSteps() []string         { return nil }
func (p *NoOpPlugin) GetEnvVars() map[string]string         { return nil }
func (p *NoOpPlugin) GetCacheDirectories() []PackageCache   { return nil }
func (p *NoOpPlugin) GetDefaultBuildSteps() []string        { return nil }
func (p *NoOpPlugin) GetDefaultInstallSteps() []string      { return nil }
