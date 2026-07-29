package plugin

type PoetryPlugin struct {
	source PluginSource
}

func (p *PoetryPlugin) Name() string    { return "poetry" }
func (p *PoetryPlugin) Version() string { return p.source.Version }

func (p *PoetryPlugin) GetExtractPath() string {
	return "/usr/local"
}

func (p *PoetryPlugin) GetPostExtractSteps() []string {
	return nil
}

func (p *PoetryPlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH":       "/usr/local/bin:/usr/local/poetry/bin:$PATH",
		"POETRY_HOME": "/usr/local/poetry",
	}
}

func (p *PoetryPlugin) GetCacheDirectories() []PackageCache {
	return []PackageCache{
		{HostSubdir: "pip", GuestPath: "/workspace/.cache/pip"},
		{HostSubdir: "pypoetry", GuestPath: "/workspace/.cache/pypoetry"},
	}
}

func (p *PoetryPlugin) GetBuildCommands() []string {
	return []string{
		"poetry install --no-root",
		"mkdir -p $ZPKG_DEST/usr/lib/$ZPKG_NAME",
		"cp -r . $ZPKG_DEST/usr/lib/$ZPKG_NAME/",
	}
}
