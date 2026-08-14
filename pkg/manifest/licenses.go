package manifest

import (
	"fmt"
	"os"
	"path/filepath"
)

// spdxIdentifiers is a curated set of common SPDX license identifiers.
func IsSPDX(name string) bool {
	return spdxIdentifiers[name]
}

func (m *RecipeManifest) ValidateLicenses(manifestDir string) error {
	for idx, lic := range m.EffectiveLicenses() {
		if lic.Name == "" {
			return fmt.Errorf("license [%d] requires a name", idx)
		}

		if lic.File != "" && lic.URL != "" {
			return fmt.Errorf("license %q cannot set both file and url", lic.Name)
		}

		if lic.File != "" {
			path := lic.File
			if !filepath.IsAbs(path) {
				path = filepath.Join(manifestDir, path)
			}

			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("license %q file %q is not accessible: %w", lic.Name, lic.File, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("license %q file %q is not a regular file", lic.Name, lic.File)
			}
		}

		if lic.SHA256 != "" && len(lic.SHA256) != 64 {
			return fmt.Errorf("license %q sha256 must be a 64-character hex hash", lic.Name)
		}
	}

	return nil
}

func (m *RecipeManifest) LicenseWarnings() []string {
	var warnings []string

	for _, lic := range m.EffectiveLicenses() {
		if lic.File == "" && lic.URL == "" && !IsSPDX(lic.Name) {
			warnings = append(warnings, fmt.Sprintf(
				"License %q is not a known SPDX identifier — add a file or url to embed the license text.", lic.Name))
		}
	}

	return warnings
}

var spdxIdentifiers = map[string]bool{
	"0BSD":              true,
	"AGPL-1.0-only":     true,
	"AGPL-3.0-only":     true,
	"AGPL-3.0-or-later": true,
	"Apache-1.0":        true,
	"Apache-1.1":        true,
	"Apache-2.0":        true,
	"Artistic-1.0":      true,
	"Artistic-1.0-Perl": true,
	"Artistic-2.0":      true,
	"BlueOak-1.0.0":     true,
	"BSD-2-Clause":      true,
	"BSD-2-Clause-Patent": true,
	"BSD-3-Clause":        true,
	"BSD-3-Clause-Clear":  true,
	"BSD-4-Clause":        true,
	"BSL-1.0":            true,
	"CC-BY-1.0":          true,
	"CC-BY-2.0":          true,
	"CC-BY-2.5":          true,
	"CC-BY-3.0":          true,
	"CC-BY-4.0":          true,
	"CC-BY-NC-4.0":       true,
	"CC-BY-NC-SA-4.0":    true,
	"CC-BY-ND-4.0":       true,
	"CC-BY-SA-3.0":       true,
	"CC-BY-SA-4.0":       true,
	"CC0-1.0":            true,
	"CDDL-1.0":           true,
	"CDDL-1.1":           true,
	"EPL-1.0":            true,
	"EPL-2.0":            true,
	"EUPL-1.1":           true,
	"EUPL-1.2":           true,
	"GPL-1.0-only":       true,
	"GPL-1.0-or-later":   true,
	"GPL-2.0-only":       true,
	"GPL-2.0-or-later":   true,
	"GPL-3.0-only":       true,
	"GPL-3.0-or-later":   true,
	"IPL-1.0":            true,
	"ISC":                true,
	"JSON":               true,
	"LGPL-2.0-only":      true,
	"LGPL-2.0-or-later":  true,
	"LGPL-2.1-only":      true,
	"LGPL-2.1-or-later":  true,
	"LGPL-3.0-only":      true,
	"LGPL-3.0-or-later":  true,
	"MIT":                true,
	"MIT-0":              true,
	"MPL-1.0":            true,
	"MPL-1.1":            true,
	"MPL-2.0":            true,
	"MulanPSL-2.0":       true,
	"NCSA":               true,
	"OFL-1.0":            true,
	"OFL-1.1":            true,
	"OpenSSL":            true,
	"PostgreSQL":         true,
	"Python-2.0":         true,
	"Python-2.0.1":       true,
	"Ruby":               true,
	"Sleepycat":          true,
	"SSPL-1.0":           true,
	"Unlicense":          true,
	"WTFPL":              true,
	"X11":                true,
	"Zlib":               true,
}
