package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jardon/zpkg-build/pkg/config"
	"github.com/jardon/zpkg-build/pkg/engine"
	"github.com/jardon/zpkg-build/pkg/manifest"
	"github.com/jardon/zpkg-build/pkg/packager"
	"github.com/jardon/zpkg-build/pkg/plugin"
)

type Builder struct {
	manifestPath   string
	cacheDir       string
	manifest       *manifest.RecipeManifest
	rawRecipe      map[string]interface{}
	recipeHash     string
	sourceHash     string
	reproducibility manifest.Reproducibility
	activePlugin   plugin.Plugin
	engine         engine.Engine
	workspace      string
	outputDir      string
	noArchive      bool
}

func New(manifestPath string) (*Builder, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".local", "share", "zpkg-build")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	b := &Builder{
		manifestPath: manifestPath,
		cacheDir:     cacheDir,
	}

	if err := b.loadManifest(); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *Builder) SetOutputDir(dir string) {
	b.outputDir = dir
}

func (b *Builder) SetNoArchive(noArchive bool) {
	b.noArchive = noArchive
}

func (b *Builder) loadManifest() error {
	rawRecipe, err := manifest.LoadManifestRaw(b.manifestPath)
	if err != nil {
		return err
	}
	b.rawRecipe = rawRecipe

	m, recipeHash, _, err := manifest.LoadAndHydrateManifest(b.manifestPath, nil)
	if err != nil {
		return err
	}

	b.manifest = m
	b.activePlugin = plugin.GetPlugin(b.manifest.Plugin)
	b.recipeHash = recipeHash
	b.reproducibility = manifest.AnalyzeReproducibility(b.rawRecipe)

	for _, w := range b.reproducibility.Warnings {
		fmt.Fprintf(os.Stderr, "  [determinism] %s\n", w)
	}

	return nil
}

func (b *Builder) setupWorkspace() error {
	projectName := b.manifest.Name
	b.workspace = filepath.Join(b.cacheDir, "workspaces", projectName+"-build")

	dirs := []string{
		filepath.Join(b.workspace, "parts", projectName, "src"),
		filepath.Join(b.workspace, "parts", projectName, "build"),
		filepath.Join(b.workspace, "parts", projectName, "dest"),
		filepath.Join(b.workspace, "pkg"),
		filepath.Join(b.workspace, "export"),
		filepath.Join(b.workspace, ".zpkg-build-state"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory %s: %w", dir, err)
		}
	}

	uid := os.Getuid()
	gid := os.Getgid()
	filepath.Walk(b.workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if int(stat.Uid) != uid || int(stat.Gid) != gid {
				os.Chown(path, uid, gid)
			}
		}
		return nil
	})

	return nil
}

func (b *Builder) computeSourceHash() (string, error) {
	hash := sha256.New()

	sourceData := fmt.Sprintf("%s:%s:%s:%s:%s", b.manifest.Source.Git, b.manifest.Source.Ref, b.manifest.Source.Path, b.manifest.Source.URL, b.manifest.Source.SHA256)
	hash.Write([]byte(sourceData))

	for _, patch := range b.manifest.Source.Patches {
		hash.Write([]byte(patch.SHA256))
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (b *Builder) sourceDir() string {
	return filepath.Join(b.workspace, "parts", b.manifest.Name, "src")
}

func (b *Builder) buildDir() string {
	return filepath.Join(b.workspace, "parts", b.manifest.Name, "build")
}

func (b *Builder) destDir() string {
	return filepath.Join(b.workspace, "parts", b.manifest.Name, "dest")
}

func (b *Builder) pkgDir() string {
	return filepath.Join(b.workspace, "pkg")
}

func (b *Builder) exportDir() string {
	return filepath.Join(b.workspace, "export")
}

func (b *Builder) stateTracker() *config.Tracker {
	return config.NewTracker(b.workspace)
}

func (b *Builder) Pull(ctx context.Context) error {
	fmt.Println("==> Stage: pull")

	if err := b.setupWorkspace(); err != nil {
		return err
	}

	srcHash, err := b.computeSourceHash()
	if err != nil {
		return err
	}
	b.sourceHash = srcHash

	tracker := b.stateTracker()
	if tracker.IsStepCached(config.StepPull, b.sourceHash, b.recipeHash) {
		fmt.Println("    pull stage cached, skipping")
		return nil
	}

	if b.manifest.Source.Git != "" {
		fmt.Println("    Cloning source from git...")
		if err := b.cloneGitSource(ctx); err != nil {
			return err
		}
	} else if b.manifest.Source.URL != "" {
		fmt.Println("    Downloading source tarball...")
		if err := b.downloadTarballSource(); err != nil {
			return err
		}
	} else if b.manifest.Source.Path != "" {
		fmt.Println("    Copying local source...")
		if err := b.copyLocalSource(ctx); err != nil {
			return err
		}
	}

	if len(b.manifest.Source.Patches) > 0 {
		fmt.Println("    Verifying patches...")
		patchCacheDir := filepath.Join(b.cacheDir, "cache")
		_, err := manifest.ResolveAndVerifyPatches(b.manifestPath, b.manifest.Source.Patches, patchCacheDir)
		if err != nil {
			return err
		}
	}

	if err := tracker.MarkStepComplete(config.StepPull, b.sourceHash, b.recipeHash); err != nil {
		return err
	}

	fmt.Println("    pull stage complete")
	return nil
}

func (b *Builder) cloneGitSource(ctx context.Context) error {
	args := []string{"clone", "--depth", "1"}

	if b.manifest.Source.Ref != "" {
		args = append(args, "--branch", b.manifest.Source.Ref)
	}

	args = append(args, b.manifest.Source.Git, b.sourceDir())

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\noutput: %s", err, string(output))
	}

	return nil
}

func (b *Builder) copyLocalSource(ctx context.Context) error {
	srcPath := b.manifest.Source.Path
	if !filepath.IsAbs(srcPath) {
		manifestDir := filepath.Dir(b.manifestPath)
		srcPath = filepath.Join(manifestDir, srcPath)
	}

	cmd := exec.CommandContext(ctx, "cp", "-a", srcPath+"/.", b.sourceDir())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy local source: %w\noutput: %s", err, string(output))
	}

	return nil
}

func (b *Builder) downloadTarballSource() error {
	if b.manifest.Source.SHA256 == "" || len(b.manifest.Source.SHA256) != 64 {
		return fmt.Errorf("source url requires a valid 64-character SHA-256 hash")
	}

	cacheDir := filepath.Join(b.cacheDir, "cache", "tarballs")
	ext := filepath.Ext(b.manifest.Source.URL)
	cachedPath := filepath.Join(cacheDir, b.manifest.Source.SHA256+ext)

	if _, err := os.Stat(cachedPath); os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return err
		}

		fmt.Printf("    Downloading %s...\n", b.manifest.Source.URL)
		resp, err := http.Get(b.manifest.Source.URL)
		if err != nil {
			return fmt.Errorf("failed to download source: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status code pulling source: %s", resp.Status)
		}

		out, err := os.Create(cachedPath)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			return err
		}
	}

	if err := verifyTarballHash(cachedPath, b.manifest.Source.SHA256); err != nil {
		return fmt.Errorf("source integrity check failed: %w", err)
	}

	if err := engine.ExtractArchiveStrip(cachedPath, b.sourceDir(), 1); err != nil {
		return fmt.Errorf("failed to extract source tarball: %w", err)
	}

	return nil
}

func verifyTarballHash(filePath, expectedSHA string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expectedSHA) {
		return fmt.Errorf("mismatch (got %s, expected %s)", actual, expectedSHA)
	}
	return nil
}

func (b *Builder) Build(ctx context.Context) error {
	if err := b.Pull(ctx); err != nil {
		return err
	}

	fmt.Println("==> Stage: build")
	tracker := b.stateTracker()
	if tracker.IsStepCached(config.StepBuild, b.sourceHash, b.recipeHash) {
		fmt.Println("    build stage cached, skipping")
		return nil
	}

	if err := b.syncSrcToBuild(); err != nil {
		return err
	}

	if len(b.manifest.Source.Patches) > 0 {
		fmt.Println("    Applying patches...")
		if err := b.applyPatches(); err != nil {
			return err
		}
	}

	fmt.Println("    Starting engine and running build steps...")
	if err := b.runInEngine(ctx, "build"); err != nil {
		return err
	}

	if err := tracker.MarkStepComplete(config.StepBuild, b.sourceHash, b.recipeHash); err != nil {
		return err
	}

	fmt.Println("    build stage complete")
	return nil
}

func (b *Builder) syncSrcToBuild() error {
	src := b.sourceDir()
	dst := b.buildDir()

	_ = os.RemoveAll(dst)

	cmd := exec.Command("cp", "-a", src+"/.", dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to sync src to build: %w\noutput: %s", err, string(output))
	}

	return nil
}

func (b *Builder) applyPatches() error {
	patchCacheDir := filepath.Join(b.cacheDir, "cache")
	verifiedPatches, err := manifest.ResolveAndVerifyPatches(b.manifestPath, b.manifest.Source.Patches, patchCacheDir)
	if err != nil {
		return err
	}

	for i, patchPath := range verifiedPatches {
		fmt.Printf("    Applying patch %d: %s\n", i+1, filepath.Base(patchPath))
		cmd := exec.Command("git", "apply", patchPath)
		cmd.Dir = b.buildDir()
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to apply patch %s: %w\noutput: %s", patchPath, err, string(output))
		}
	}

	return nil
}

func (b *Builder) runInEngine(ctx context.Context, stage string) error {
	eng, err := engine.New(b.manifest.Engine, "")
	if err != nil {
		return err
	}
	b.engine = eng

	mounts := []engine.Mount{
		{HostPath: b.workspace, ContainerPath: "/zpkg-build-workspace"},
	}

	var pluginArchivePath string
	var pluginExtractPath string
	if b.activePlugin.Name() != "none" && b.activePlugin.Name() != "" {
		pluginCacheDir := filepath.Join(b.cacheDir, "cache")
		hostArchivePath, err := plugin.ResolveAndStage(b.manifest.Plugin, pluginCacheDir)
		if err != nil {
			return fmt.Errorf("failed to resolve plugin source: %w", err)
		}
		pluginArchivePath = hostArchivePath
		pluginExtractPath = b.activePlugin.GetExtractPath()
	}

	if err := eng.CreateEnvironment(ctx, b.manifest.Base, mounts); err != nil {
		return err
	}
	defer eng.Destroy(ctx)

	if pluginArchivePath != "" && pluginExtractPath != "" {
		if strings.Contains(pluginExtractPath, "$HOME") {
			homeDir, err := eng.RunOutput(ctx, engine.RunConfig{
				Commands: []string{"echo $HOME"},
			})
			if err != nil {
				return fmt.Errorf("failed to resolve HOME directory: %w", err)
			}
			homeDir = strings.TrimSpace(homeDir)
			pluginExtractPath = strings.Replace(pluginExtractPath, "$HOME", homeDir, 1)
		}

		fmt.Printf("    Creating plugin directory: %s\n", pluginExtractPath)
		if err := eng.Run(ctx, engine.RunConfig{
			Commands: []string{"mkdir -p " + pluginExtractPath},
		}); err != nil {
			return fmt.Errorf("failed to create plugin extract directory: %w", err)
		}

		tarReader, err := engine.DecompressArchive(pluginArchivePath)
		if err != nil {
			return fmt.Errorf("failed to decompress plugin archive: %w", err)
		}

		fmt.Printf("    Installing plugin to %s\n", pluginExtractPath)
		if err := eng.CopyTarStream(ctx, tarReader, pluginExtractPath); err != nil {
			return fmt.Errorf("failed to stream plugin into environment: %w", err)
		}
	}

	for _, step := range b.activePlugin.GetPostExtractSteps() {
		fmt.Printf("    Running post-extract: %s\n", step)
		if err := eng.Run(ctx, engine.RunConfig{
			Commands: []string{step},
		}); err != nil {
			return fmt.Errorf("post-extract step failed: %w", err)
		}
	}

	envVars := b.activePlugin.GetEnvVars()
	envVars["ZPKG_DEST"] = "/zpkg-build-workspace/parts/" + b.manifest.Name + "/dest"
	envVars["ZPKG_WORKSPACE"] = "/zpkg-build-workspace"

	workDir := "/zpkg-build-workspace/parts/" + b.manifest.Name + "/build"

	if stage == "build" || stage == "all" {
		pluginCommands := b.activePlugin.GetBuildCommands()

		fmt.Println("    Running build steps...")
		for name, bc := range pluginCommands {
			args := bc.DefaultArgs
			if override, ok := b.manifest.Build.Args[name]; ok {
				args = override
			}
			cmd := bc.Command
			if args != "" {
				cmd = bc.Command + " " + args
			}
			fmt.Printf("      > %s\n", cmd)
			if err := eng.Run(ctx, engine.RunConfig{
				EnvVars:    envVars,
				WorkingDir: workDir,
				Commands:   []string{cmd},
			}); err != nil {
				return fmt.Errorf("build step %q failed: %w", name, err)
			}
		}
	}

	return nil
}

func (b *Builder) Package(ctx context.Context) error {
	if err := b.Build(ctx); err != nil {
		return err
	}

	fmt.Println("==> Stage: package")
	tracker := b.stateTracker()
	if tracker.IsStepCached(config.StepPackage, b.sourceHash, b.recipeHash) {
		fmt.Println("    package stage cached, skipping")
		return nil
	}

	fmt.Println("    Assembling package...")
	if err := b.assemblePackage(); err != nil {
		return err
	}

	if !b.noArchive {
		fmt.Println("    Generating metadata...")
		if err := packager.GenerateMetadata(
			b.manifest.Name,
			b.manifest.Version,
			b.pkgDir(),
			b.recipeHash,
			b.rawRecipe,
			b.manifest.Plugin,
			b.manifest.BuildDeps,
			b.manifest.RuntimeDeps,
		); err != nil {
			return err
		}
	}

	if err := tracker.MarkStepComplete(config.StepPackage, b.sourceHash, b.recipeHash); err != nil {
		return err
	}

	fmt.Println("    package stage complete")
	return nil
}

func (b *Builder) assemblePackage() error {
	destPath := b.destDir()
	pkgPath := b.pkgDir()

	includes := normalizePatterns(b.manifest.Package["include"])
	excludes := normalizePatterns(b.manifest.Package["exclude"])

	return filepath.Walk(destPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(destPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if matchesAnyExclude(relPath, excludes) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if len(includes) > 0 && !matchesAnyInclude(relPath, includes) {
			if !(info.IsDir() && isIncludePrefix(relPath, includes)) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		dest := filepath.Join(pkgPath, relPath)

		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode())
		}

		return copyFile(path, dest)
	})
}

func normalizePatterns(patterns []string) []string {
	var normalized []string
	for _, p := range patterns {
		p = strings.TrimPrefix(p, "/")
		normalized = append(normalized, p)
	}
	return normalized
}

func matchesAnyInclude(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPath(relPath, pattern) {
			return true
		}
	}
	return false
}

func matchesAnyExclude(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPath(relPath, pattern) {
			return true
		}
	}
	return false
}

func isIncludePrefix(dirPath string, patterns []string) bool {
	prefix := dirPath + "/"
	for _, p := range patterns {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func matchPath(relPath, pattern string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(relPath, "/")

	return matchParts(pathParts, patternParts)
}

func matchParts(pathParts, patternParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}

	if patternParts[0] == "**" {
		rest := patternParts[1:]
		for i := 0; i <= len(pathParts); i++ {
			if matchParts(pathParts[i:], rest) {
				return true
			}
		}
		return false
	}

	if len(pathParts) == 0 {
		return false
	}

	matched, err := filepath.Match(patternParts[0], pathParts[0])
	if err != nil || !matched {
		return false
	}

	return matchParts(pathParts[1:], patternParts[1:])
}

func (b *Builder) Export(ctx context.Context) error {
	if err := b.Package(ctx); err != nil {
		return err
	}

	fmt.Println("==> Stage: export")
	tracker := b.stateTracker()
	if tracker.IsStepCached(config.StepExport, b.sourceHash, b.recipeHash) {
		fmt.Println("    export stage cached, skipping")
		return nil
	}

	outputDir := b.outputDir
	if outputDir == "" {
		outputDir = "."
	}

	outputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if b.noArchive {
		fmt.Println("    Copying package contents to output directory...")
		if err := b.copyPackageToOutput(outputDir); err != nil {
			return err
		}

		if err := tracker.MarkStepComplete(config.StepExport, b.sourceHash, b.recipeHash); err != nil {
			return err
		}

		fmt.Printf("    Exported to: %s\n", outputDir)
		fmt.Println("    export stage complete")
		return nil
	}

	format := b.manifest.Export.Format
	if format == "" {
		format = "tar.gz"
	}

	archiveName := fmt.Sprintf("%s-%s-%s.%s", b.manifest.Name, b.manifest.Version, b.manifest.Arch, format)
	archivePath := filepath.Join(outputDir, archiveName)

	fmt.Printf("    Creating %s archive...\n", format)

	switch format {
	case "tar.gz":
		if err := b.createTarGz(archivePath); err != nil {
			return err
		}
	case "zip":
		if err := b.createZip(archivePath); err != nil {
			return err
		}
	default:
		if err := b.createTarGz(archivePath); err != nil {
			return err
		}
	}

	if err := tracker.MarkStepComplete(config.StepExport, b.sourceHash, b.recipeHash); err != nil {
		return err
	}

	fmt.Printf("    Exported to: %s\n", archivePath)
	fmt.Println("    export stage complete")
	return nil
}

func (b *Builder) createTarGz(archivePath string) error {
	cmd := exec.Command("tar", "-czf", archivePath, "-C", b.pkgDir(), ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (b *Builder) createZip(archivePath string) error {
	cmd := exec.Command("zip", "-r", archivePath, ".")
	cmd.Dir = b.pkgDir()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zip failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (b *Builder) copyPackageToOutput(outputDir string) error {
	pkgPath := b.pkgDir()
	return filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(pkgPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		dest := filepath.Join(outputDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode())
		}

		return copyFile(path, dest)
	})
}

func (b *Builder) Clean(step string) error {
	if err := b.setupWorkspace(); err != nil {
		return err
	}

	var targetStep config.Step
	switch step {
	case "pull":
		targetStep = config.StepPull
	case "build":
		targetStep = config.StepBuild
	case "package":
		targetStep = config.StepPackage
	case "export":
		targetStep = config.StepExport
	default:
		return fmt.Errorf("unknown step: %s", step)
	}

	tracker := b.stateTracker()
	if err := tracker.InvalidateFrom(targetStep); err != nil {
		return err
	}

	dirsToClean := map[config.Step][]string{
		config.StepPull: {
			filepath.Join(b.workspace, "parts", b.manifest.Name, "src"),
		},
		config.StepBuild: {
			filepath.Join(b.workspace, "parts", b.manifest.Name, "build"),
			filepath.Join(b.workspace, "parts", b.manifest.Name, "dest"),
		},
		config.StepPackage: {
			filepath.Join(b.workspace, "pkg"),
		},
		config.StepExport: {
			filepath.Join(b.workspace, "export"),
		},
	}

	for _, s := range config.StepOrder {
		found := false
		for _, ts := range config.StepOrder {
			if ts == targetStep {
				found = true
			}
			if ts == s {
				break
			}
		}
		if found || s == targetStep {
	for _, dir := range dirsToClean[s] {
		fmt.Printf("    Cleaning: %s\n", dir)
		_ = os.RemoveAll(dir)
		_ = os.MkdirAll(dir, 0755)
	}
		}
	}

	fmt.Printf("    Cleaned from stage '%s' forward\n", step)
	return nil
}

func (b *Builder) Status() error {
	if err := b.setupWorkspace(); err != nil {
		return err
	}

	srcHash, err := b.computeSourceHash()
	if err != nil {
		return err
	}
	b.sourceHash = srcHash

	tracker := b.stateTracker()

	fmt.Printf("Project: %s v%s\n", b.manifest.Name, b.manifest.Version)
	fmt.Printf("Engine:  %s\n", b.manifest.Engine)
	fmt.Printf("Plugin:  %s %s\n", b.manifest.Plugin.Name, b.manifest.Plugin.Version)
	fmt.Printf("Recipe:  %s\n", b.recipeHash[:16]+"...")
	fmt.Printf("Source:  %s\n", b.sourceHash[:16]+"...")
	fmt.Println()

	for _, step := range config.StepOrder {
		state, err := tracker.GetState(step)
		if err != nil {
			fmt.Printf("  [%s] not started\n", step)
			continue
		}

		symbol := "ok"
		if state.Status != "completed" {
			symbol = state.Status
		}
		fmt.Printf("  [%s] %s (at %s)\n", step, symbol, state.Timestamp.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (b *Builder) findManifest() (string, error) {
	candidates := []string{"package.yaml", "package.yml", "zpkg.yaml", "zpkg.yml"}

	for _, candidate := range candidates {
		path := filepath.Join(".", candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no manifest file found (looked for: %s)", strings.Join(candidates, ", "))
}
