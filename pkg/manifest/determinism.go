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
		if source, hasSource := plugin["source"].(string); hasSource && source != "" {
			if sha, ok := plugin["sha256"].(string); !ok || len(sha) != 64 {
				warnings = append(warnings, "Plugin compiler toolchain is missing a valid SHA-256 checksum verification.")
			}
		}
	} else {
		warnings = append(warnings, "No toolchain plugin defined. Compilation depends entirely on the host configuration.")
	}

	if source, ok := recipe["source"].(map[string]interface{}); ok {
		if path, ok := source["path"].(string); ok && path != "" {
			warnings = append(warnings, "Local source path is not reproducible across environments.")
		}

		if git, ok := source["git"].(string); ok && git != "" {
			ref, _ := source["ref"].(string)
			if ref == "" {
				warnings = append(warnings, "Git source has no pinned ref — build depends on default branch.")
			} else if !IsCommitSHA(ref) {
				warnings = append(warnings, fmt.Sprintf("Git ref '%s' is not a commit SHA — cannot guarantee reproducibility.", ref))
			}
		}

		if url, ok := source["url"].(string); ok && url != "" {
			sha, _ := source["sha256"].(string)
			md5val, _ := source["md5"].(string)
			if sha == "" && md5val == "" {
				warnings = append(warnings, "URL source has no SHA-256 or MD5 verification — integrity cannot be guaranteed.")
			}
		}

		if patches, ok := source["patches"].([]interface{}); ok {
			for idx, p := range patches {
				if patchMap, ok := p.(map[string]interface{}); ok {
					sha, hasSha := patchMap["sha256"].(string)
					md5, hasMD5 := patchMap["md5"].(string)
					if (!hasSha || len(sha) != 64) && (!hasMD5 || len(md5) != 32) {
						warnings = append(warnings, fmt.Sprintf("Patch [%d] is missing a valid SHA-256 or MD5 checksum. This invalidates static caching.", idx))
					}
					if url, hasURL := patchMap["url"].(string); hasURL && url != "" {
						warnings = append(warnings, "Recipe relies on a remote patch URL: "+url+". Pre-downloading via a local patch directory is preferred for offline reliability.")
					}
				}
			}
		}
	}

	if build, ok := recipe["build"].(map[string]interface{}); ok {
		if overrides, ok := build["override-steps"].(string); ok && strings.TrimSpace(overrides) != "" {
			warnings = append(warnings, "Build uses override-steps — custom commands are not managed by a plugin and may not be repeatable.")
		}
	}

	if licenses, ok := recipe["licenses"].([]interface{}); ok {
		for idx, l := range licenses {
			if lm, ok := l.(map[string]interface{}); ok {
				if file, ok := lm["file"].(string); ok && file != "" {
					warnings = append(warnings, fmt.Sprintf("License [%d] references a local file — not reproducible across environments.", idx))
				}
			}
		}
	}

	return Reproducibility{
		Deterministic: len(warnings) == 0,
		Warnings:      warnings,
	}
}

func IsCommitSHA(ref string) bool {
	if len(ref) != 40 {
		return false
	}
	for _, c := range ref {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
