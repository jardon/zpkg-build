package plugin

type GoPlugin struct {
	source PluginSource
}

func (p *GoPlugin) Name() string    { return "golang" }
func (p *GoPlugin) Version() string { return p.source.Version }

func (p *GoPlugin) GetInstallScripts(archivePath string) []string {
	return []string{
		"mkdir -p $HOME/.local",
		"tar -xzf " + archivePath + " -C $HOME/.local/",
	}
}

func (p *GoPlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH":    "$HOME/.local/go/bin:$PATH",
		"GOPATH":  "/zpkg-build-workspace/gopath",
		"GOCACHE": "/zpkg-build-workspace/cache/go-build",
		"GOROOT":  "$HOME/.local/go",
	}
}

func (p *GoPlugin) GetCacheDirectories() []PackageCache {
	return []PackageCache{
		{HostSubdir: "go-mod", GuestPath: "/zpkg-build-workspace/gopath/pkg/mod"},
		{HostSubdir: "go-build", GuestPath: "/zpkg-build-workspace/cache/go-build"},
	}
}

func (p *GoPlugin) GetDefaultBuildSteps() []string {
	return []string{
		"go mod download",
		"go build -v -o bin/out main.go",
	}
}

func (p *GoPlugin) GetDefaultInstallSteps() []string {
	return []string{
		"mkdir -p $ZPKG_DEST/usr/bin",
		"cp bin/out $ZPKG_DEST/usr/bin/",
	}
}
