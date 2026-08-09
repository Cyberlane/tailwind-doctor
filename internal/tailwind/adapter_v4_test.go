package tailwind

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

func loadVersion4(t *testing.T, files fstest.MapFS, entry string) Theme {
	t.Helper()
	theme, err := adapterVersion4{}.Load(files, Package{Dir: ".", Version: Version4, Entries: []string{entry}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return theme
}

func TestVersion4ReadsThemeNamespaces(t *testing.T) {
	files := fstest.MapFS{"app.css": &fstest.MapFile{Data: []byte(`@import "tailwindcss";
@theme {
  --color-brand-500: #3b82f6;
  --spacing-gutter: 1.5rem;
  --text-huge: 3rem;
  --font-display: "Satoshi", sans-serif;
  --font-weight-chunky: 850;
  --leading-loose-ish: 1.8;
  --tracking-tightish: -0.01em;
  --radius-card: 0.75rem;
  --shadow-card: 0 1px 2px #0000001a;
  --breakpoint-3xl: 120rem;
  --container-prose: 65ch;
}`)}}
	theme := loadVersion4(t, files, "app.css")
	testCases := []struct {
		family tokens.Family
		name   string
		value  string
	}{
		{tokens.FamilyColor, "brand-500", "#3b82f6"},
		{tokens.FamilySpacing, "gutter", "1.5rem"},
		{tokens.FamilyFontSize, "huge", "3rem"},
		{tokens.FamilyFontWeight, "chunky", "850"},
		{tokens.FamilyLineHeight, "loose-ish", "1.8"},
		{tokens.FamilyLetterSpacing, "tightish", "-0.01em"},
		{tokens.FamilyRadius, "card", "0.75rem"},
		{tokens.FamilyBreakpoint, "3xl", "120rem"},
		{tokens.FamilyContainer, "prose", "65ch"},
	}
	for _, testCase := range testCases {
		token, found := theme.Inventory.ByName(testCase.family, testCase.name)
		if !found || token.Value != testCase.value || token.Origin != tokens.OriginProject || token.Decl.File != "app.css" || token.Decl.Line == 0 {
			t.Errorf("%s/%s = %+v, found %v", testCase.family, testCase.name, token, found)
		}
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyFontFamily, "weight-chunky"); found {
		t.Error("font weight misclassified as family")
	}
}

func TestVersion4LoadsEveryPackageEntryInOrder(t *testing.T) {
	files := fstest.MapFS{
		"a.css": &fstest.MapFile{Data: []byte(`@theme { --color-brand: red; --spacing-card: 1rem; }`)},
		"b.css": &fstest.MapFile{Data: []byte(`@theme { --color-brand: blue; --radius-card: 0.5rem; }`)},
	}
	theme, err := adapterVersion4{}.Load(files, Package{
		Dir: ".", Version: Version4, Entries: []string{"a.css", "b.css"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	brand, found := theme.Inventory.ByName(tokens.FamilyColor, "brand")
	if !found || brand.Value != "blue" {
		t.Fatalf("brand = %+v, found %v", brand, found)
	}
	for family, name := range map[tokens.Family]string{
		tokens.FamilySpacing: "card",
		tokens.FamilyRadius:  "card",
	} {
		if _, found := theme.Inventory.ByName(family, name); !found {
			t.Errorf("%s/%s missing", family, name)
		}
	}
}

func TestVersion4DegradesOnMalformedTailwindCSS(t *testing.T) {
	theme := loadVersion4(t, fstest.MapFS{
		"app.css": &fstest.MapFile{Data: []byte(`@theme { --color-brand: red;`)},
	}, "app.css")
	if !theme.Degraded || len(theme.Diagnostics) != 1 {
		t.Fatalf("theme = %+v", theme)
	}
	if theme.Diagnostics[0].Kind != DiagnosticUnreadableConfig ||
		!strings.Contains(theme.Diagnostics[0].Message, "unterminated CSS block") {
		t.Fatalf("diagnostic = %+v", theme.Diagnostics[0])
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "red-500"); !found {
		t.Error("parse degradation did not retain the default theme")
	}
}

func TestVersion4ClearsNamespacesInOrder(t *testing.T) {
	theme := loadVersion4(t, fstest.MapFS{"app.css": &fstest.MapFile{Data: []byte(`@theme { --color-brand: #3b82f6; --color-*: initial; }`)}}, "app.css")
	if theme.Inventory.Count(tokens.FamilyColor) != 0 {
		t.Errorf("colour count = %d", theme.Inventory.Count(tokens.FamilyColor))
	}
	theme = loadVersion4(t, fstest.MapFS{"app.css": &fstest.MapFile{Data: []byte(`@theme { --color-*: initial; --color-brand: #3b82f6; }`)}}, "app.css")
	if theme.Inventory.Count(tokens.FamilyColor) != 1 {
		t.Errorf("colour count after clear then add = %d", theme.Inventory.Count(tokens.FamilyColor))
	}
	if _, found := theme.Inventory.ByName(tokens.FamilySpacing, "DEFAULT"); !found {
		t.Error("clearing colours removed base spacing")
	}
}

func TestVersion4ClearsEverything(t *testing.T) {
	theme := loadVersion4(t, fstest.MapFS{"app.css": &fstest.MapFile{Data: []byte(`@theme { --*: initial; --spacing: 4px; --color-lagoon: #3ab7bf; }`)}}, "app.css")
	if theme.Inventory.Count(tokens.FamilyColor) != 1 {
		t.Errorf("colour count = %d", theme.Inventory.Count(tokens.FamilyColor))
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyBreakpoint, "md"); found {
		t.Error("breakpoint survived clear all")
	}
}

func TestVersion4MarksUnresolvableAndReadsPrefix(t *testing.T) {
	theme := loadVersion4(t, fstest.MapFS{"app.css": &fstest.MapFile{Data: []byte(`@import "tailwindcss" prefix(tw); @theme inline { --font-sans: var(--font-inter); }`)}}, "app.css")
	token, found := theme.Inventory.ByName(tokens.FamilyFontFamily, "sans")
	if !found || !token.Unresolvable || token.Raw != "var(--font-inter)" {
		t.Fatalf("sans = %+v, found %v", token, found)
	}
	if theme.Syntax.Prefix != "tw" || !theme.Syntax.PrefixIsVariant {
		t.Errorf("syntax = %+v", theme.Syntax)
	}
}

func TestVersion4FollowsRelativeImportsOnly(t *testing.T) {
	files := fstest.MapFS{
		"app.css":                      &fstest.MapFile{Data: []byte(`@import "tailwindcss"; @import "./theme.css"; @import "@acme/theme.css";`)},
		"theme.css":                    &fstest.MapFile{Data: []byte(`@theme { --color-brand: #3b82f6; }`)},
		"node_modules/@acme/theme.css": &fstest.MapFile{Data: []byte(`@theme { --color-vendored: red; }`)},
	}
	theme := loadVersion4(t, files, "app.css")
	brand, found := theme.Inventory.ByName(tokens.FamilyColor, "brand")
	if !found || brand.Decl.File != "theme.css" {
		t.Fatalf("brand = %+v, found %v", brand, found)
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "vendored"); found {
		t.Error("bare import followed into node_modules")
	}
}

func TestVersion4ReportsImportCycleWithoutDiscardingTokens(t *testing.T) {
	files := fstest.MapFS{
		"app.css": &fstest.MapFile{Data: []byte(`@import "./a.css";`)},
		"a.css":   &fstest.MapFile{Data: []byte(`@import "./b.css";`)},
		"b.css":   &fstest.MapFile{Data: []byte(`@import "./a.css"; @theme { --color-brand: #3b82f6; }`)},
	}
	theme := loadVersion4(t, files, "app.css")
	if !theme.Degraded {
		t.Error("cycle did not degrade")
	}
	var cycle bool
	for _, diagnostic := range theme.Diagnostics {
		cycle = cycle || diagnostic.Kind == DiagnosticImportCycle
	}
	if !cycle {
		t.Error("cycle diagnostic missing")
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "brand"); !found {
		t.Error("token beside cycle discarded")
	}
}

func TestVersion4ReadsConfigPluginsAndRootProperties(t *testing.T) {
	files := fstest.MapFS{
		"app.css":            &fstest.MapFile{Data: []byte(`@config "./tailwind.config.js"; @plugin "@tailwindcss/typography"; :root { --color-loose: #abcdef; --my-own-thing: 4px; }`)},
		"tailwind.config.js": &fstest.MapFile{Data: []byte(`module.exports = { theme: { extend: { colors: { legacy: "#123456" } } } }`)},
	}
	theme := loadVersion4(t, files, "app.css")
	for _, name := range []string{"legacy", "loose"} {
		if _, found := theme.Inventory.ByName(tokens.FamilyColor, name); !found {
			t.Errorf("%s missing", name)
		}
	}
	if len(theme.Plugins) != 1 || theme.Plugins[0] != "@tailwindcss/typography" {
		t.Errorf("plugins = %v", theme.Plugins)
	}
	for _, token := range theme.Inventory.Tokens() {
		if strings.Contains(token.Path, "my-own-thing") {
			t.Error("non-namespaced property became a token")
		}
	}
}
