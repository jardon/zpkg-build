package plugin

type MavenPlugin struct {
	source PluginSource
}

func (p *MavenPlugin) Name() string    { return "maven" }
func (p *MavenPlugin) Version() string { return p.source.Version }

func (p *MavenPlugin) GetInstallScripts(baseOS string) []string {
	return []string{
		"tar -xzf /opt/plugin/toolchain.tar.gz -C /usr/share/",
	}
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

func (p *MavenPlugin) GetDefaultBuildSteps() []string {
	return []string{
		"mvn clean package -DskipTests",
	}
}

func (p *MavenPlugin) GetDefaultInstallSteps() []string {
	return []string{
		"mkdir -p $ZPKG_DEST/usr/share/app",
		"cp target/*.jar $ZPKG_DEST/usr/share/app/",
	}
}
