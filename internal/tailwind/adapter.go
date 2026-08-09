package tailwind

import (
	"fmt"
	"io/fs"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/defaults"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

// Theme is everything reading one package's configuration produced.
type Theme struct {
	Inventory   *tokens.Inventory
	Syntax      UtilitySyntax
	Plugins     []string
	Diagnostics []Diagnostic
	Degraded    bool
}

// Adapter reads one Tailwind configuration dialect.
type Adapter interface {
	Version() Version
	Load(fsys fs.FS, pkg Package) (Theme, error)
}

type themeLoader func(fsys fs.FS, file string, theme *Theme, active map[string]bool, depth int) error

func loadAdapterTheme(fsys fs.FS, pkg Package, source, version string, loader themeLoader) (Theme, error) {
	theme := Theme{
		Inventory: defaults.Theme(version),
		Syntax:    DefaultUtilitySyntax(),
		Plugins:   []string{},
	}
	if source == "" {
		return theme, fmt.Errorf("Tailwind v%s package %s has no theme source", version, pkg.Dir)
	}
	if err := loader(fsys, cleanPath(source), &theme, map[string]bool{}, 0); err != nil {
		return Theme{}, err
	}
	SortDiagnostics(theme.Diagnostics)
	return theme, nil
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
