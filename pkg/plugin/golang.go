package plugin

type GoPlugin struct {
	source PluginSource
}

func (p *GoPlugin) Name() string    { return "golang" }
func (p *GoPlugin) Version() string { return p.source.Version }

func (p *GoPlugin) GetExtractPath() string {
	return "$HOME/.local"
}

func (p *GoPlugin) GetPostExtractSteps() []string {
	return []string{
		"mkdir -p /zpkg-build-workspace/gopath /zpkg-build-workspace/cache/go-build",
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

func (p *GoPlugin) GetBuildCommands() []string {
	return []string{
		"go mod download",
		"go build -v -o $ZPKG_DEST/usr/bin/$ZPKG_NAME main.go",
	}
}
