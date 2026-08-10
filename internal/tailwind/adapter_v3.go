package tailwind

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/defaults"
	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/jsobject"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

type adapterVersion3 struct{}

func (adapterVersion3) Version() Version { return Version3 }

func (adapterVersion3) Load(fsys fs.FS, pkg Package) (Theme, error) {
	sources := []string{}
	if pkg.ConfigFile != "" {
		sources = append(sources, pkg.ConfigFile)
	}
	return loadAdapterTheme(fsys, pkg, sources, "3", loadVersion3Config)
}

func loadVersion3Config(fsys fs.FS, file string, theme *Theme, active map[string]bool, depth int) error {
	if depth > 8 {
		theme.Degraded = true
		theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
			Kind: DiagnosticImportDepth, File: file,
			Message: "Preset chain exceeds the maximum depth of 8.",
		})
		return nil
	}
	if active[file] {
		theme.Degraded = true
		theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
			Kind: DiagnosticImportCycle, File: file,
			Message: "Preset chain contains a cycle.",
		})
		return nil
	}
	active[file] = true
	defer delete(active, file)

	content, err := fs.ReadFile(fsys, file)
	if err != nil {
		return fmt.Errorf("read Tailwind config %s: %w", file, err)
	}
	result, err := jsobject.Parse(string(content))
	if err != nil {
		theme.Inventory = defaults.Theme("3")
		theme.Degraded = true
		theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
			Kind: DiagnosticUnreadableConfig, File: file,
			Message: fmt.Sprintf("Cannot read Tailwind config statically: %v", err),
		})
		return nil
	}

	loadVersion3Presets(fsys, file, result.Root, theme, active, depth)

	if themeEntry, found := result.Root.Entry("theme"); found {
		defeats := defeatsWithinEntry(result.Root, themeEntry, result.Defeats)
		if themeEntry.Value.Kind == jsobject.KindUnreadable || len(defeats) > 0 {
			theme.Inventory = defaults.Theme("3")
			theme.Degraded = true
			if len(defeats) == 0 {
				defeats = []jsobject.Defeat{{Construct: "unreadable theme", Line: themeEntry.Line, Column: themeEntry.Column}}
			}
			for _, defeat := range defeats {
				theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
					Kind: DiagnosticUnreadableConfig, File: file,
					Line: defeat.Line, Column: defeat.Column,
					Message: fmt.Sprintf("Cannot read Tailwind theme without executing its %s.", defeat.Construct),
				})
			}
		} else if themeEntry.Value.Kind == jsobject.KindObject {
			applyVersion3Theme(theme.Inventory, file, themeEntry.Value)
		}
	}

	loadVersion3Syntax(result.Root, &theme.Syntax)
	loadVersion3Plugins(result.Root, &theme.Plugins)
	return nil
}

func loadVersion3Presets(fsys fs.FS, file string, root jsobject.Value, theme *Theme, active map[string]bool, depth int) {
	presets, found := root.Get("presets")
	if !found || presets.Kind != jsobject.KindArray {
		return
	}
	for _, preset := range presets.Items {
		specifier, readable := requireSpecifier(preset)
		if !readable || (!strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../")) {
			theme.Degraded = true
			theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
				Kind: DiagnosticExternalPreset, File: file,
				Line: preset.Line, Column: preset.Column,
				Message: fmt.Sprintf("Preset %q is external and was not read.", specifier),
			})
			continue
		}
		presetFile := cleanPath(path.Clean(path.Join(path.Dir(file), specifier)))
		if presetFile == ".." || strings.HasPrefix(presetFile, "../") {
			theme.Degraded = true
			theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
				Kind: DiagnosticExternalPreset, File: file,
				Line: preset.Line, Column: preset.Column,
				Message: fmt.Sprintf("Preset %q resolves outside the project and was not read.", specifier),
			})
			continue
		}
		if err := loadVersion3Config(fsys, presetFile, theme, active, depth+1); err != nil {
			theme.Degraded = true
			theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
				Kind: DiagnosticExternalPreset, File: file,
				Line: preset.Line, Column: preset.Column,
				Message: fmt.Sprintf("Preset %q could not be read: %v", specifier, err),
			})
		}
	}
}

func requireSpecifier(value jsobject.Value) (string, bool) {
	if value.Kind != jsobject.KindCall || value.Callee != "require" || len(value.Args) != 1 || value.Args[0].Kind != jsobject.KindString {
		return "", false
	}
	return value.Args[0].Str, true
}

func defeatsWithinEntry(root jsobject.Value, target jsobject.Entry, defeats []jsobject.Defeat) []jsobject.Defeat {
	endLine, endColumn := int(^uint(0)>>1), int(^uint(0)>>1)
	for _, entry := range root.Entries {
		if positionAfter(entry.Line, entry.Column, target.Line, target.Column) && positionBefore(entry.Line, entry.Column, endLine, endColumn) {
			endLine, endColumn = entry.Line, entry.Column
		}
	}
	inside := []jsobject.Defeat{}
	for _, defeat := range defeats {
		if !positionBefore(defeat.Line, defeat.Column, target.Value.Line, target.Value.Column) && positionBefore(defeat.Line, defeat.Column, endLine, endColumn) {
			inside = append(inside, defeat)
		}
	}
	return inside
}

func positionBefore(line, column, otherLine, otherColumn int) bool {
	return line < otherLine || (line == otherLine && column < otherColumn)
}

func positionAfter(line, column, otherLine, otherColumn int) bool {
	return line > otherLine || (line == otherLine && column > otherColumn)
}

var version3Families = []struct {
	key    string
	family tokens.Family
}{
	{"colors", tokens.FamilyColor},
	{"spacing", tokens.FamilySpacing},
	{"fontFamily", tokens.FamilyFontFamily},
	{"fontSize", tokens.FamilyFontSize},
	{"fontWeight", tokens.FamilyFontWeight},
	{"lineHeight", tokens.FamilyLineHeight},
	{"letterSpacing", tokens.FamilyLetterSpacing},
	{"borderRadius", tokens.FamilyRadius},
	{"boxShadow", tokens.FamilyShadow},
	{"screens", tokens.FamilyBreakpoint},
	{"zIndex", tokens.FamilyZIndex},
}

func applyVersion3Theme(inventory *tokens.Inventory, file string, theme jsobject.Value) {
	for _, mapping := range version3Families {
		if entry, found := theme.Entry(mapping.key); found {
			inventory.Clear(mapping.family)
			putVersion3Tokens(inventory, file, mapping.family, mapping.key, nil, entry.Value)
		}
	}
	extend, found := theme.Get("extend")
	if !found || extend.Kind != jsobject.KindObject {
		return
	}
	for _, mapping := range version3Families {
		if entry, found := extend.Entry(mapping.key); found {
			putVersion3Tokens(inventory, file, mapping.family, mapping.key, nil, entry.Value)
		}
	}
}

func putVersion3Tokens(inventory *tokens.Inventory, file string, family tokens.Family, themeKey string, parents []string, value jsobject.Value) {
	if value.Kind != jsobject.KindObject {
		return
	}
	for _, entry := range value.Entries {
		if entry.Spread {
			continue
		}
		parts := append(append([]string(nil), parents...), entry.Key)
		if entry.Value.Kind == jsobject.KindObject {
			putVersion3Tokens(inventory, file, family, themeKey, parts, entry.Value)
			continue
		}
		raw, readable := version3RawValue(family, entry.Value)
		if !readable {
			continue
		}
		nameParts := parts
		if len(parts) > 1 && parts[len(parts)-1] == "DEFAULT" {
			nameParts = parts[:len(parts)-1]
		}
		name := strings.Join(nameParts, "-")
		if name == "" {
			name = "DEFAULT"
		}
		normalized, resolvable := tokens.Normalize(raw)
		inventory.Put(tokens.Token{
			Family: family, Name: name,
			Path:  strings.Join(append([]string{themeKey}, parts...), "."),
			Value: normalized, Raw: raw, Origin: tokens.OriginProject,
			Decl:         tokens.Site{File: file, Line: entry.Line, Column: entry.Column},
			Unresolvable: !resolvable,
		})
	}
}

func version3RawValue(family tokens.Family, value jsobject.Value) (string, bool) {
	switch value.Kind {
	case jsobject.KindString:
		return value.Str, true
	case jsobject.KindNumber:
		return value.Num, true
	case jsobject.KindArray:
		if family == tokens.FamilyFontSize && len(value.Items) > 0 {
			return version3RawValue(family, value.Items[0])
		}
		if family == tokens.FamilyFontFamily {
			parts := value.Strings()
			if len(parts) > 0 {
				return strings.Join(parts, ", "), true
			}
		}
	}
	return "", false
}

func loadVersion3Syntax(root jsobject.Value, syntax *UtilitySyntax) {
	if prefix, found := root.Get("prefix"); found && prefix.Kind == jsobject.KindString {
		syntax.Prefix = prefix.Str
	}
	if separator, found := root.Get("separator"); found && separator.Kind == jsobject.KindString && separator.Str != "" {
		syntax.Separator = separator.Str
	}
}

func loadVersion3Plugins(root jsobject.Value, plugins *[]string) {
	configured, found := root.Get("plugins")
	if !found || configured.Kind != jsobject.KindArray {
		return
	}
	for _, value := range configured.Items {
		if specifier, readable := requireSpecifier(value); readable {
			*plugins = append(*plugins, specifier)
		}
	}
}
