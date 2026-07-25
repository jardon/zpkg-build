package plugin

type RustPlugin struct {
	source PluginSource
}

func (p *RustPlugin) Name() string    { return "rust" }
func (p *RustPlugin) Version() string { return p.source.Version }

func (p *RustPlugin) GetExtractPath() string {
	return "/usr/local"
}

func (p *RustPlugin) GetPostExtractSteps() []string {
	return nil
}

func (p *RustPlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH":       "/usr/local/cargo/bin:$PATH",
		"CARGO_HOME": "/usr/local/cargo",
	}
}

func (p *RustPlugin) GetCacheDirectories() []PackageCache {
	return []PackageCache{
		{HostSubdir: "cargo-registry", GuestPath: "/usr/local/cargo/registry"},
		{HostSubdir: "cargo-git", GuestPath: "/usr/local/cargo/git"},
	}
}

func (p *RustPlugin) GetBuildCommands() map[string]BuildCommand {
	return map[string]BuildCommand{
		"build":   {Command: "cargo build", DefaultArgs: "--release"},
		"install": {Command: "sh -c", DefaultArgs: "mkdir -p $ZPKG_DEST/usr/bin && cp target/release/out $ZPKG_DEST/usr/bin/"},
	}
}
