package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	for _, dep := range manifest.BuildDeps {
		if dep.Source != "" && (dep.SHA256 == "" || len(dep.SHA256) != 64) {
			return nil, "", nil, fmt.Errorf("build dependency %q with a source URL requires a valid 64-character SHA-256 hash", dep.Name)
		}
		if dep.ExtractTo != "" && !filepath.IsAbs(dep.ExtractTo) {
			return nil, "", nil, fmt.Errorf("build dependency %q extract-to path must be absolute", dep.Name)
		}
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
