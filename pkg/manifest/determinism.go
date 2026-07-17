package manifest

import (
	"fmt"
	"strings"
)

type Reproducibility struct {
	Deterministic bool     `json:"deterministic"`
	Warnings      []string `json:"warnings"`
}

func AnalyzeReproducibility(recipe map[string]interface{}) Reproducibility {
	var warnings []string

	if base, ok := recipe["base"].(string); ok {
		if !strings.Contains(base, "@sha256:") {
			warnings = append(warnings, "Base image '"+base+"' is not pinned to a SHA-256 digest.")
		}
	}

	if plugin, ok := recipe["plugin"].(map[string]interface{}); ok {
		if sha, ok := plugin["sha256"].(string); !ok || len(sha) != 64 {
			warnings = append(warnings, "Plugin compiler toolchain is missing a valid SHA-256 checksum verification.")
		}
	} else {
		warnings = append(warnings, "No toolchain plugin defined. Compilation depends entirely on the host configuration.")
	}

	if source, ok := recipe["source"].(map[string]interface{}); ok {
		if patches, ok := source["patches"].([]interface{}); ok {
			for idx, p := range patches {
				if patchMap, ok := p.(map[string]interface{}); ok {
					sha, hasSha := patchMap["sha256"].(string)
					if !hasSha || len(sha) != 64 {
						warnings = append(warnings, fmt.Sprintf("Patch [%d] is missing a valid SHA-256 checksum. This invalidates static caching.", idx))
					}
					if url, hasURL := patchMap["url"].(string); hasURL && url != "" {
						warnings = append(warnings, "Recipe relies on a remote patch URL: "+url+". Pre-downloading via a local patch directory is preferred for offline reliability.")
					}
				}
			}
		}
	}

	if build, ok := recipe["build"].(map[string]interface{}); ok {
		if steps, ok := build["steps"].([]interface{}); ok {
			for _, step := range steps {
				if stepStr, ok := step.(string); ok {
					if containsNetworkUtilities(stepStr) {
						warnings = append(warnings, "Build step '"+stepStr+"' contains raw download utilities. Fetch dependencies via pinned plugins instead.")
					}
				}
			}
		}
	}

	return Reproducibility{
		Deterministic: len(warnings) == 0,
		Warnings:      warnings,
	}
}

func containsNetworkUtilities(step string) bool {
	utils := []string{"curl ", "wget ", "git clone ", "npm install ", "pip install ", "cargo install "}
	stepLower := strings.ToLower(step)
	for _, util := range utils {
		if strings.Contains(stepLower, util) {
			return true
		}
	}
	return false
}
