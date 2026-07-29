package plugin

type MavenPlugin struct {
	source PluginSource
}

func (p *MavenPlugin) Name() string    { return "maven" }
func (p *MavenPlugin) Version() string { return p.source.Version }

func (p *MavenPlugin) GetExtractPath() string {
	return "/usr/share"
}

func (p *MavenPlugin) GetPostExtractSteps() []string {
	return nil
}

func (p *MavenPlugin) GetEnvVars() map[string]string {
	return map[string]string{
		"PATH":      "/usr/share/maven/bin:$PATH",
		"JAVA_HOME": "/usr/lib/jvm/default-jvm",
	}
}

func (p *MavenPlugin) GetCacheDirectories() []PackageCache {
	return []PackageCache{
		{HostSubdir: "m2-repo", GuestPath: "/root/.m2/repository"},
	}
}

func (p *MavenPlugin) GetBuildCommands() []string {
	return []string{
		"mvn clean package -DskipTests",
		"sh -c mkdir -p $ZPKG_DEST/usr/share/app && cp target/*.jar $ZPKG_DEST/usr/share/app/",
	}
}
