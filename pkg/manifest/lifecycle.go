package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jardon/zpkg-build/pkg/plugin"
	"gopkg.in/yaml.v3"
)

func ComputeRecipeHash(manifestPath string) (string, error) {
	yamlData, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest: %w", err)
	}

	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &rawMap); err != nil {
		return "", fmt.Errorf("failed to parse YAML for hashing: %w", err)
	}

	canonicalJSON, err := json.Marshal(rawMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal canonical json: %w", err)
	}

	hash := sha256.New()
	hash.Write(canonicalJSON)
	recipeHash := hex.EncodeToString(hash.Sum(nil))

	return recipeHash, nil
}

func ComputeHydratedRecipeHash(recipeMap map[string]interface{}) (string, error) {
	canonicalJSON, err := json.Marshal(recipeMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal canonical map: %w", err)
	}

	hash := sha256.New()
	hash.Write(canonicalJSON)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func LoadAndHydrateManifest(manifestPath string, _ any) (*RecipeManifest, string, map[string]interface{}, error) {
	yamlData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest RecipeManifest
	if err := yaml.Unmarshal(yamlData, &manifest); err != nil {
		return nil, "", nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if len(manifest.Plugin.Args) > 0 {
		if err := plugin.ValidateArgs(manifest.Plugin.Name, manifest.Plugin.Args); err != nil {
			return nil, "", nil, fmt.Errorf("invalid plugin build args: %w", err)
		}
	}

	for _, dep := range manifest.BuildDeps {
		if dep.Source != "" {
			hasSHA := len(dep.SHA256) == 64
			hasMD5 := len(dep.MD5) == 32
			if !hasSHA && !hasMD5 {
				return nil, "", nil, fmt.Errorf("build dependency %q with a source URL requires a valid SHA-256 (64 hex) or MD5 (32 hex) checksum", dep.Name)
			}
		}
		if dep.ExtractTo != "" && !filepath.IsAbs(dep.ExtractTo) {
			return nil, "", nil, fmt.Errorf("build dependency %q extract-to path must be absolute", dep.Name)
		}
		if dep.Rename != "" && strings.Contains(dep.Rename, "/") {
			return nil, "", nil, fmt.Errorf("build dependency %q rename must not contain path separators", dep.Name)
		}
	}

	if err := manifest.ValidateLicenses(filepath.Dir(manifestPath)); err != nil {
		return nil, "", nil, err
	}

	hydratedJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to serialize hydrated manifest: %w", err)
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(hydratedJSON, &rawMap); err != nil {
		return nil, "", nil, err
	}

	canonicalJSON, err := json.Marshal(rawMap)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to marshal canonical config: %w", err)
	}

	hash := sha256.New()
	hash.Write(canonicalJSON)
	recipeHash := hex.EncodeToString(hash.Sum(nil))

	return &manifest, recipeHash, rawMap, nil
}

func LoadManifestRaw(manifestPath string) (map[string]interface{}, error) {
	yamlData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return rawMap, nil
}
