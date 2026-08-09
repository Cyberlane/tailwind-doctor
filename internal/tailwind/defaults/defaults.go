// Package defaults holds Tailwind's own default theme values.
//
// The tables are needed twice: as the base that a project's theme.extend merges
// into, and as the fallback when a configuration cannot be read without executing
// it. They are Go literals rather than embedded JSON because a literal cannot fail
// to parse at startup, and the data is fixed at compile time.
//
// These values are taken from Tailwind CSS, which is MIT licensed. See the
// attribution note at the top of v3.go and v4.go.
package defaults

import "github.com/Cyberlane/tailwind-doctor/internal/tokens"

// entry is one row of a default table. Values are written exactly as Tailwind
// publishes them and normalized on the way into the inventory, so a table is
// reviewable against upstream in a diff.
type entry struct {
	family tokens.Family
	name   string
	raw    string
}

// Theme returns a fresh inventory of the default tokens for a major version. An
// unrecognised version yields an empty inventory rather than nil, so a caller
// never has to nil-check before iterating. Each call builds a new inventory
// because a caller is about to merge a project's own tokens over it.
func Theme(version string) *tokens.Inventory {
	inventory := tokens.NewInventory()
	for _, row := range table(version) {
		value, resolvable := tokens.Normalize(row.raw)
		inventory.Put(tokens.Token{
			Family:       row.family,
			Name:         row.name,
			Path:         row.name,
			Value:        value,
			Raw:          row.raw,
			Origin:       tokens.OriginDefault,
			Unresolvable: !resolvable,
		})
	}
	return inventory
}

func table(version string) []entry {
	switch version {
	case "3":
		return version3
	case "4":
		return version4
	}
	return nil
}
