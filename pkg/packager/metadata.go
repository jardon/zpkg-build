package packager

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jardon/zpkg-build/pkg/manifest"
	"github.com/jardon/zpkg-build/pkg/plugin"
)

type FileMeta struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256,omitempty"`
}

type BuildMetadata struct {
	Dependencies []manifest.Dependency `json:"dependencies,omitempty"`
}

type PackageMetadata struct {
	Name            string                   `json:"name"`
	Version         string                   `json:"version"`
	RecipeHash      string                   `json:"recipe_hash"`
	Reproducibility manifest.Reproducibility `json:"reproducibility"`
	BuiltAt         time.Time                `json:"built_at"`
	Plugin          plugin.PluginSource      `json:"plugin"`
	Licenses        []manifest.License       `json:"licenses,omitempty"`
	Build           BuildMetadata            `json:"build,omitempty"`
	RuntimeDeps     []manifest.Dependency    `json:"runtime_deps,omitempty"`
	Contents        []FileMeta               `json:"contents"`
}

func GenerateMetadata(pkgName, pkgVersion, pkgDir, recipeHash string, rawRecipe map[string]interface{}, toolchain plugin.PluginSource, licenses []manifest.License, buildDeps []manifest.Dependency, runtimeDeps []manifest.Dependency) error {
	var files []FileMeta

	err := filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Base(path) == "metadata.json" {
			return nil
		}

		relPath, err := filepath.Rel(pkgDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(filepath.Join("/", relPath))

		fileType := "file"
		var fileHash string

		if info.IsDir() {
			fileType = "directory"
		} else if info.Mode()&os.ModeSymlink != 0 {
			fileType = "symlink"
		} else {
			fileHash, _ = calculateFileHash(path)
		}

		files = append(files, FileMeta{
			Path:      relPath,
			Type:      fileType,
			SizeBytes: info.Size(),
			SHA256:    fileHash,
		})
		return nil
	})

	if err != nil {
		return err
	}

	reproStats := manifest.AnalyzeReproducibility(rawRecipe)

	metadata := PackageMetadata{
		Name:            pkgName,
		Version:         pkgVersion,
		RecipeHash:      recipeHash,
		Reproducibility: reproStats,
		BuiltAt:         time.Now().UTC(),
		Plugin:          toolchain,
		Licenses:        licenses,
		Build:           BuildMetadata{Dependencies: buildDeps},
		RuntimeDeps:     runtimeDeps,
		Contents:        files,
	}

	metaFile, err := os.Create(filepath.Join(pkgDir, "metadata.json"))
	if err != nil {
		return err
	}
	defer metaFile.Close()

	encoder := json.NewEncoder(metaFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metadata)
}

func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
