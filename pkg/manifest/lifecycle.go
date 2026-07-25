package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"github.com/jardon/zpkg-build/pkg/plugin"
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

func LoadAndHydrateManifest(manifestPath string, activePlugin plugin.Plugin) (*RecipeManifest, string, map[string]interface{}, error) {
	yamlData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest RecipeManifest
	if err := yaml.Unmarshal(yamlData, &manifest); err != nil {
		return nil, "", nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if activePlugin != nil {
		pluginCommands := activePlugin.GetBuildCommands()

		for name, args := range manifest.Build.Args {
			if _, ok := pluginCommands[name]; !ok {
				return nil, "", nil, fmt.Errorf("unknown build command %q for plugin %q", name, activePlugin.Name())
			}
			if err := ValidateArgs(args); err != nil {
				return nil, "", nil, fmt.Errorf("invalid args for command %q: %w", name, err)
			}
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
