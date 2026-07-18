package plugin

type RustPlugin struct {
	source PluginSource
}

func (p *RustPlugin) Name() string    { return "rust" }
func (p *RustPlugin) Version() string { return p.source.Version }

func (p *RustPlugin) GetInstallScripts(archivePath string) []string {
	return []string{
		"tar -xzf " + archivePath + " -C /usr/local/",
	}
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

func (p *RustPlugin) GetDefaultBuildSteps() []string {
	return []string{
		"cargo build --release",
	}
}

func (p *RustPlugin) GetDefaultInstallSteps() []string {
	return []string{
		"mkdir -p $ZPKG_DEST/usr/bin",
		"cp target/release/out $ZPKG_DEST/usr/bin/",
	}
}
