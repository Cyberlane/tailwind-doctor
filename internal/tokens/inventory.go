// Package tokens holds the canonical design-token inventory. It is deliberately
// version-agnostic: Tailwind v3 spells a border radius theme.borderRadius and v4
// spells it --radius-*, and a rule should not have to know which dialect produced
// the project it is looking at. Nothing here imports the format readers, so the
// contract the rules bind to cannot drift with a Tailwind release.
package tokens

import "sort"

// Family names the kind of value a token carries. A rule that suggests a font
// size must never offer a line height, so the distinction is a type-level fact
// rather than a naming convention.
type Family string

const (
	FamilyColor         Family = "color"
	FamilySpacing       Family = "spacing"
	FamilyFontFamily    Family = "font-family"
	FamilyFontSize      Family = "font-size"
	FamilyFontWeight    Family = "font-weight"
	FamilyLineHeight    Family = "line-height"
	FamilyLetterSpacing Family = "letter-spacing"
	FamilyRadius        Family = "radius"
	FamilyShadow        Family = "shadow"
	FamilyBreakpoint    Family = "breakpoint"
	FamilyContainer     Family = "container"
)

// familyOrder fixes the order families appear in every report. Ranging over a map
// would order them differently between runs, and byte-identical output is a
// product boundary.
var familyOrder = []Family{
	FamilyColor,
	FamilySpacing,
	FamilyFontFamily,
	FamilyFontSize,
	FamilyFontWeight,
	FamilyLineHeight,
	FamilyLetterSpacing,
	FamilyRadius,
	FamilyShadow,
	FamilyBreakpoint,
	FamilyContainer,
}

// Families returns the families in report order. The slice is a copy, so a caller
// cannot reorder every future report by accident.
func Families() []Family {
	return append([]Family(nil), familyOrder...)
}

// Origin records where a token came from. Only a project token can be reported as
// unused: flagging one of Tailwind's own defaults as unused would be a fabricated
// finding, and the rule is about custom tokens that nobody adopted.
type Origin string

const (
	OriginDefault Origin = "default"
	OriginProject Origin = "project"
	OriginPlugin  Origin = "plugin"
)

// Site is where a token was declared. It is empty for a default token, which has
// no declaration in the user's project to point at.
type Site struct {
	File   string
	Line   int
	Column int
}

// Token is one entry in the inventory.
type Token struct {
	// Family is the kind of value this token carries.
	Family Family
	// Name is the canonical, dialect-independent name: both theme.colors.brand.500
	// and --color-brand-500 yield "brand-500".
	Name string
	// Path is how the project spelled it, for a message a human can act on.
	Path string
	// Value is normalized for comparison. It is empty when Unresolvable is set.
	Value string
	// Raw is the value verbatim as written.
	Raw    string
	Origin Origin
	Decl   Site
	// Unresolvable marks a value this tool will not compare — one containing
	// var() or calc(), whose real value depends on cascade this tool does not
	// evaluate. Suggesting a token whose value is unknown is worse than silence.
	Unresolvable bool
}

// Inventory is the set of tokens available to one package. It is not safe for
// concurrent writes; a package's theme is built once and then read.
type Inventory struct {
	byFamily map[Family]map[string]Token
}

// NewInventory returns an empty token inventory.
func NewInventory() *Inventory {
	return &Inventory{byFamily: map[Family]map[string]Token{}}
}

// Put adds a token, replacing any existing token with the same family and name.
// Replacement is how a project value overrides a default and how a later
// declaration wins over an earlier one, which is the cascade both dialects use.
func (inventory *Inventory) Put(token Token) {
	family, present := inventory.byFamily[token.Family]
	if !present {
		family = map[string]Token{}
		inventory.byFamily[token.Family] = family
	}
	family[token.Name] = token
}

// Clear empties one family. Tailwind v4 spells this "--color-*: initial", and a
// project that clears the colours and declares twelve has twelve, not twelve plus
// Tailwind's defaults.
func (inventory *Inventory) Clear(family Family) {
	delete(inventory.byFamily, family)
}

// ClearAll empties every family, for v4's "--*: initial".
func (inventory *Inventory) ClearAll() {
	inventory.byFamily = map[Family]map[string]Token{}
}

// ByName returns a token by canonical family and name.
func (inventory *Inventory) ByName(family Family, name string) (Token, bool) {
	token, found := inventory.byFamily[family][name]
	return token, found
}

// Count returns the number of tokens in a family.
func (inventory *Inventory) Count(family Family) int {
	return len(inventory.byFamily[family])
}

// Tokens returns every token in family order, then name order. Sorted here rather
// than at each call site, because every caller feeds output.
func (inventory *Inventory) Tokens() []Token {
	total := 0
	for _, family := range inventory.byFamily {
		total += len(family)
	}
	collected := make([]Token, 0, total)

	for _, family := range familyOrder {
		named := inventory.byFamily[family]
		names := make([]string, 0, len(named))
		for name := range named {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			collected = append(collected, named[name])
		}
	}
	return collected
}
