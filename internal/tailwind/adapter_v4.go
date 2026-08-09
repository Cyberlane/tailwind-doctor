package tailwind

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/cssdecl"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

type adapterVersion4 struct{}

func (adapterVersion4) Version() Version { return Version4 }

func (adapterVersion4) Load(fsys fs.FS, pkg Package) (Theme, error) {
	return loadAdapterTheme(fsys, pkg, pkg.Entry, "4", loadVersion4File)
}

func loadVersion4File(fsys fs.FS, file string, theme *Theme, active map[string]bool, depth int) error {
	if depth > 8 {
		theme.Degraded = true
		theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
			Kind: DiagnosticImportDepth, File: file,
			Message: "CSS import chain exceeds the maximum depth of 8.",
		})
		return nil
	}
	if active[file] {
		theme.Degraded = true
		theme.Diagnostics = append(theme.Diagnostics, Diagnostic{
			Kind: DiagnosticImportCycle, File: file,
			Message: "CSS import chain contains a cycle.",
		})
		return nil
	}
	active[file] = true
	defer delete(active, file)

	content, err := fs.ReadFile(fsys, file)
	if err != nil {
		return fmt.Errorf("read Tailwind CSS entry %s: %w", file, err)
	}
	sheet, err := cssdecl.Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse Tailwind CSS entry %s: %w", file, err)
	}
	return applyVersion4Nodes(fsys, file, sheet.Nodes, theme, active, depth)
}

func applyVersion4Nodes(fsys fs.FS, file string, nodes []cssdecl.Node, theme *Theme, active map[string]bool, depth int) error {
	for _, node := range nodes {
		if node.Kind == cssdecl.NodeAtRule {
			switch node.Name {
			case "import":
				if prefix, found := functionArgument(node.Prelude, "prefix"); found && prefix != "" {
					theme.Syntax.Prefix = prefix
					theme.Syntax.PrefixIsVariant = true
				}
				specifier, found := firstQuoted(node.Prelude)
				if found && (strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../")) {
					imported := cleanPath(path.Clean(path.Join(path.Dir(file), specifier)))
					if imported != ".." && !strings.HasPrefix(imported, "../") {
						if err := loadVersion4File(fsys, imported, theme, active, depth+1); err != nil {
							return err
						}
					}
				}
			case "theme":
				applyVersion4Declarations(theme.Inventory, file, node.Children)
			case "plugin":
				if specifier, found := firstQuoted(node.Prelude); found {
					theme.Plugins = append(theme.Plugins, specifier)
				}
			case "config":
				if specifier, found := firstQuoted(node.Prelude); found && (strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../")) {
					configFile := cleanPath(path.Clean(path.Join(path.Dir(file), specifier)))
					if configFile != ".." && !strings.HasPrefix(configFile, "../") {
						legacy, err := adapterVersion3{}.Load(fsys, Package{Dir: cleanDir(path.Dir(configFile)), Version: Version3, ConfigFile: configFile})
						if err != nil {
							return err
						}
						for _, token := range legacy.Inventory.Tokens() {
							theme.Inventory.Put(token)
						}
						theme.Plugins = append(theme.Plugins, legacy.Plugins...)
						theme.Diagnostics = append(theme.Diagnostics, legacy.Diagnostics...)
						theme.Degraded = theme.Degraded || legacy.Degraded
					}
				}
			default:
				if err := applyVersion4Nodes(fsys, file, node.Children, theme, active, depth); err != nil {
					return err
				}
			}
			continue
		}
		if node.Kind == cssdecl.NodeRule {
			if strings.TrimSpace(node.Selector) == ":root" {
				applyVersion4Declarations(theme.Inventory, file, node.Children)
			} else if err := applyVersion4Nodes(fsys, file, node.Children, theme, active, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

var version4Namespaces = []struct {
	stem   string
	family tokens.Family
}{
	{"font-weight", tokens.FamilyFontWeight},
	{"breakpoint", tokens.FamilyBreakpoint},
	{"container", tokens.FamilyContainer},
	{"tracking", tokens.FamilyLetterSpacing},
	{"leading", tokens.FamilyLineHeight},
	{"spacing", tokens.FamilySpacing},
	{"radius", tokens.FamilyRadius},
	{"shadow", tokens.FamilyShadow},
	{"color", tokens.FamilyColor},
	{"text", tokens.FamilyFontSize},
	{"font", tokens.FamilyFontFamily},
}

func namespaceFor(family tokens.Family) string {
	for _, namespace := range version4Namespaces {
		if namespace.family == family {
			return namespace.stem
		}
	}
	return ""
}

func applyVersion4Declarations(inventory *tokens.Inventory, file string, nodes []cssdecl.Node) {
	for _, node := range nodes {
		if node.Kind != cssdecl.NodeDeclaration {
			continue
		}
		property := strings.TrimSpace(node.Property)
		value := strings.TrimSpace(node.Value)
		if property == "--*" && strings.EqualFold(value, "initial") {
			inventory.ClearAll()
			continue
		}

		family, name, found := version4Property(property)
		if !found {
			continue
		}
		if name == "*" && strings.EqualFold(value, "initial") {
			inventory.Clear(family)
			continue
		}
		if name == "*" || strings.Contains(name, "--") {
			continue
		}
		normalized, resolvable := tokens.Normalize(value)
		inventory.Put(tokens.Token{
			Family: family, Name: name, Path: property,
			Value: normalized, Raw: value, Origin: tokens.OriginProject,
			Decl:         tokens.Site{File: file, Line: node.Line, Column: node.Column},
			Unresolvable: !resolvable,
		})
	}
}

func version4Property(property string) (tokens.Family, string, bool) {
	if property == "--spacing" {
		return tokens.FamilySpacing, "DEFAULT", true
	}
	for _, namespace := range version4Namespaces {
		prefix := "--" + namespace.stem + "-"
		if strings.HasPrefix(property, prefix) && len(property) > len(prefix) {
			return namespace.family, property[len(prefix):], true
		}
	}
	return "", "", false
}

func firstQuoted(text string) (string, bool) {
	for index := 0; index < len(text); index++ {
		quote := text[index]
		if quote != '\'' && quote != '"' {
			continue
		}
		var builder strings.Builder
		for index++; index < len(text); index++ {
			character := text[index]
			if character == '\\' && index+1 < len(text) {
				index++
				builder.WriteByte(text[index])
				continue
			}
			if character == quote {
				return builder.String(), true
			}
			builder.WriteByte(character)
		}
		return "", false
	}
	return "", false
}

func functionArgument(text, name string) (string, bool) {
	marker := name + "("
	start := strings.Index(text, marker)
	if start < 0 {
		return "", false
	}
	start += len(marker)
	end := strings.IndexByte(text[start:], ')')
	if end < 0 {
		return "", false
	}
	argument := strings.TrimSpace(text[start : start+end])
	argument = strings.Trim(argument, "\"'")
	return argument, true
}
