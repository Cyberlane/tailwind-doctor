package tailwind

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

func loadVersion3(t *testing.T, config string) Theme {
	t.Helper()
	files := fstest.MapFS{"tailwind.config.js": &fstest.MapFile{Data: []byte(config)}}
	theme, err := adapterVersion3{}.Load(files, Package{Dir: ".", Version: Version3, ConfigFile: "tailwind.config.js"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return theme
}

func TestVersion3ExtendMergesOverDefaults(t *testing.T) {
	theme := loadVersion3(t, `module.exports = { theme: { extend: { colors: { brand: { 500: "#3b82f6" } } } } }`)
	brand, found := theme.Inventory.ByName(tokens.FamilyColor, "brand-500")
	if !found || brand.Origin != tokens.OriginProject || brand.Path != "colors.brand.500" || brand.Decl.Line == 0 {
		t.Fatalf("brand = %+v, found %v", brand, found)
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "red-500"); !found {
		t.Error("extend removed defaults")
	}
	if theme.Degraded {
		t.Error("readable config degraded")
	}
}

func TestVersion3BareKeyReplacesDefaults(t *testing.T) {
	theme := loadVersion3(t, `module.exports = { theme: { colors: { brand: "#3b82f6" }, extend: { spacing: { gutter: "1.5rem" } } } }`)
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "red-500"); found {
		t.Error("bare colors kept defaults")
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "brand"); !found {
		t.Error("brand missing")
	}
	if _, found := theme.Inventory.ByName(tokens.FamilySpacing, "4"); !found {
		t.Error("spacing defaults missing")
	}
	if _, found := theme.Inventory.ByName(tokens.FamilySpacing, "gutter"); !found {
		t.Error("gutter missing")
	}
}

func TestVersion3ReadsSyntaxAndPlugins(t *testing.T) {
	theme := loadVersion3(t, `module.exports = { prefix: "tw-", separator: "_", plugins: [require("@tailwindcss/typography"), require("daisyui")] }`)
	if theme.Syntax.Prefix != "tw-" || theme.Syntax.Separator != "_" {
		t.Errorf("syntax = %+v", theme.Syntax)
	}
	if len(theme.Plugins) != 2 || theme.Plugins[0] != "@tailwindcss/typography" || theme.Plugins[1] != "daisyui" {
		t.Errorf("plugins = %v", theme.Plugins)
	}
}

func TestVersion3ResolvesPluginCoverageFromManifest(t *testing.T) {
	files := fstest.MapFS{
		"package.json":       &fstest.MapFile{Data: []byte(`{"devDependencies":{"@tailwindcss/typography":"^0.5.20","daisyui":"^5.7.16"}}`)},
		"tailwind.config.js": &fstest.MapFile{Data: []byte(`module.exports = { plugins: [require("@tailwindcss/typography"), require("daisyui/plugin")] }`)},
	}
	theme, err := adapterVersion3{}.Load(files, Package{
		Dir: ".", Version: Version3, ConfigFile: "tailwind.config.js", ManifestFile: "package.json",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(theme.PluginCoverage) != 2 {
		t.Fatalf("plugin coverage = %+v", theme.PluginCoverage)
	}
	if !theme.PluginCoverage[0].Complete || theme.PluginCoverage[0].VersionRange != "^0.5.20" {
		t.Errorf("typography coverage = %+v", theme.PluginCoverage[0])
	}
	if theme.PluginCoverage[1].Complete || theme.PluginCoverage[1].Support != "partial" ||
		theme.PluginCoverage[1].VersionRange != "^5.7.16" {
		t.Errorf("daisyUI coverage = %+v", theme.PluginCoverage[1])
	}
}

func TestVersion3DegradesOnUnreadableTheme(t *testing.T) {
	theme := loadVersion3(t, `const defaultTheme = require("tailwindcss/defaultTheme")
module.exports = { theme: { extend: { fontFamily: { ...defaultTheme.fontFamily } } } }`)
	if !theme.Degraded || len(theme.Diagnostics) == 0 {
		t.Fatalf("theme = %+v", theme)
	}
	diagnostic := theme.Diagnostics[0]
	if diagnostic.Kind != DiagnosticUnreadableConfig || diagnostic.File != "tailwind.config.js" || diagnostic.Line != 2 || !strings.Contains(diagnostic.Message, "spread") {
		t.Errorf("diagnostic = %+v", diagnostic)
	}
	for _, token := range theme.Inventory.Tokens() {
		if token.Origin != tokens.OriginDefault {
			t.Fatalf("partial project token survived: %+v", token)
		}
	}
}

func TestVersion3DegradesWhenNoExportedObjectCanBeRead(t *testing.T) {
	theme := loadVersion3(t, `defineConfig({ theme: {} })`)
	if !theme.Degraded || len(theme.Diagnostics) != 1 {
		t.Fatalf("theme = %+v", theme)
	}
	if theme.Diagnostics[0].Kind != DiagnosticUnreadableConfig ||
		!strings.Contains(theme.Diagnostics[0].Message, "no exported object") {
		t.Fatalf("diagnostic = %+v", theme.Diagnostics[0])
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "red-500"); !found {
		t.Error("parse degradation did not retain the default theme")
	}
}

func TestVersion3DefeatOutsideThemeDoesNotDegrade(t *testing.T) {
	theme := loadVersion3(t, `module.exports = { content: globs, theme: { extend: { colors: { brand: "#3b82f6" } } } }`)
	if theme.Degraded {
		t.Error("content defeat degraded theme")
	}
	if _, found := theme.Inventory.ByName(tokens.FamilyColor, "brand"); !found {
		t.Error("brand missing")
	}
}

func TestVersion3FollowsLocalPresetAndReportsPackagePreset(t *testing.T) {
	files := fstest.MapFS{
		"tailwind.config.js": &fstest.MapFile{Data: []byte(`module.exports = { presets: [require("./base.config.js"), require("@acme/preset")], theme: { extend: { colors: { brand: "#3b82f6" } } } }`)},
		"base.config.js":     &fstest.MapFile{Data: []byte(`module.exports = { theme: { extend: { colors: { base: "#111111" } } } }`)},
	}
	theme, err := adapterVersion3{}.Load(files, Package{Dir: ".", Version: Version3, ConfigFile: "tailwind.config.js"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, name := range []string{"base", "brand"} {
		if _, found := theme.Inventory.ByName(tokens.FamilyColor, name); !found {
			t.Errorf("%s missing", name)
		}
	}
	if !theme.Degraded {
		t.Error("external preset did not degrade")
	}
	var external bool
	for _, diagnostic := range theme.Diagnostics {
		external = external || diagnostic.Kind == DiagnosticExternalPreset
	}
	if !external {
		t.Error("external preset diagnostic missing")
	}
}

func TestVersion3FontSizePairUsesSizeOnly(t *testing.T) {
	theme := loadVersion3(t, `module.exports = { theme: { extend: { fontSize: { huge: ["3rem", { lineHeight: "1" }] } } } }`)
	size, found := theme.Inventory.ByName(tokens.FamilyFontSize, "huge")
	if !found || size.Value != "3rem" {
		t.Fatalf("size = %+v, found %v", size, found)
	}
}

func TestVersion3SkipsContainerOptions(t *testing.T) {
	theme := loadVersion3(t, `module.exports = { theme: { container: { center: true, padding: "2rem" } } }`)
	if theme.Inventory.Count(tokens.FamilyContainer) != 0 {
		t.Errorf("container count = %d", theme.Inventory.Count(tokens.FamilyContainer))
	}
}
