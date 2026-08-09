package tailwind

import (
	"io/fs"

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

// AdapterFor returns the adapter for a supported version.
func AdapterFor(version Version) (Adapter, bool) {
	switch version {
	case Version3:
		return adapterVersion3{}, true
	}
	return nil, false
}
