package tailwind

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/defaults"
	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/plugins"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

// Theme is everything reading one package's configuration produced.
type Theme struct {
	Inventory      *tokens.Inventory
	Syntax         UtilitySyntax
	Plugins        []string
	PluginCoverage []plugins.Coverage
	Diagnostics    []Diagnostic
	Degraded       bool
}

// Adapter reads one Tailwind configuration dialect.
type Adapter interface {
	Version() Version
	Load(fsys fs.FS, pkg Package) (Theme, error)
}

type themeLoader func(fsys fs.FS, file string, theme *Theme, active map[string]bool, depth int) error

func loadAdapterTheme(fsys fs.FS, pkg Package, sources []string, version string, loader themeLoader) (Theme, error) {
	theme := Theme{
		Inventory: defaults.Theme(version),
		Syntax:    DefaultUtilitySyntax(),
		Plugins:   []string{},
	}
	if len(sources) == 0 {
		return theme, fmt.Errorf("Tailwind v%s package %s has no theme source", version, pkg.Dir)
	}
	for _, source := range sources {
		if err := loader(fsys, cleanPath(source), &theme, map[string]bool{}, 0); err != nil {
			return Theme{}, err
		}
	}
	SortDiagnostics(theme.Diagnostics)
	pluginCoverage, err := resolvePluginCoverage(fsys, pkg, theme.Plugins)
	if err != nil {
		return Theme{}, err
	}
	theme.PluginCoverage = pluginCoverage
	return theme, nil
}

func resolvePluginCoverage(fsys fs.FS, pkg Package, specifiers []string) ([]plugins.Coverage, error) {
	versions := map[string]string{}
	if pkg.ManifestFile != "" {
		content, err := fs.ReadFile(fsys, pkg.ManifestFile)
		if err != nil {
			return nil, fmt.Errorf("read plugin versions from %s: %w", pkg.ManifestFile, err)
		}
		var manifest packageManifest
		if json.Unmarshal(content, &manifest) == nil {
			for _, dependencies := range []map[string]string{
				manifest.Dependencies, manifest.DevDependencies, manifest.PeerDependencies,
			} {
				for name, versionRange := range dependencies {
					if _, found := versions[name]; !found {
						versions[name] = versionRange
					}
				}
			}
		}
	}
	return plugins.Resolve(specifiers, versions), nil
}

// AdapterFor returns the adapter for a supported version.
func AdapterFor(version Version) (Adapter, bool) {
	switch version {
	case Version3:
		return adapterVersion3{}, true
	case Version4:
		return adapterVersion4{}, true
	}
	return nil, false
}
