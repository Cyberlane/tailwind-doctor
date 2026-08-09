package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
)

var sourceExtensions = map[string]bool{
	".astro": true, ".html": true, ".jsx": true, ".tsx": true,
	".vue": true, ".svelte": true, ".css": true,
}

type Finding struct {
	Rule     string   `json:"rule"`
	Category Category `json:"category"`
	Message  string   `json:"message"`
	File     string   `json:"file"`
	Class    string   `json:"class"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Severity Severity `json:"severity"`
	// Confidence is decided by the rule, not by configuration. Only high
	// confidence moves the score by default.
	Confidence Confidence `json:"confidence"`
	// Scored records whether this finding moved the score, so a reader never has
	// to re-derive the severity and confidence rules to explain the number.
	Scored bool `json:"scored"`
}

type resolvedTheme struct {
	packageDirectory string
	theme            tailwind.Theme
}

// Run analyses a directory tree. Configuration and the baseline are read from
// the root, so a caller needs nothing but a path.
func Run(root string) (Report, error) {
	config, err := LoadConfig(root)
	if err != nil {
		return Report{}, err
	}
	baseline, err := LoadBaseline(root, config.BaselinePath)
	if err != nil {
		return Report{}, err
	}
	return RunWithConfig(root, config, baseline)
}

// RunWithConfig analyses a directory tree with configuration supplied by the
// caller, which is what lets the baseline be regenerated from a run that ignores
// the existing one.
func RunWithConfig(root string, config Config, baseline *Baseline) (Report, error) {
	// Findings starts non-nil so a clean project serializes as [] rather than
	// null, which every JSON consumer would otherwise have to special-case.
	report := Report{
		SchemaVersion: SchemaVersion,
		Tool:          ToolInfo{Name: "tw-doctor", Version: Version},
		ScoreModel:    scoreModel(),
		Diagnostics:   []ReportDiagnostic{},
		Findings:      []Finding{},
	}
	layout, err := tailwind.Discover(os.DirFS(root))
	if err != nil {
		return Report{}, fmt.Errorf("discover Tailwind packages: %w", err)
	}
	report.themes, report.Diagnostics, err = loadThemes(os.DirFS(root), layout)
	if err != nil {
		return Report{}, err
	}
	// Suppressed findings are kept, unreported, only so the score the project
	// would have without its baseline can be computed.
	suppressed := []Finding{}
	ignores := newIgnoreRules(config.IgnorePaths)

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist", "build", ".next", "vendor":
				return filepath.SkipDir
			}
			if config.RespectGitignore {
				// WalkDir visits a directory before its contents, so patterns are
				// recorded in time to apply to everything they govern.
				if content, err := os.ReadFile(filepath.Join(path, ".gitignore")); err == nil {
					ignores.addGitignore(relative, string(content))
				}
			}
			if relative != "." && ignores.ignores(relative, true) {
				return filepath.SkipDir
			}
			return nil
		}

		if !sourceExtensions[filepath.Ext(path)] || ignores.ignores(relative, false) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		report.Scanned.Files++
		syntax := syntaxForFile(relative, layout, report.themes, config)
		for _, list := range Extract(path, string(content)) {
			// An unresolved site names an expression, not a set of utilities.
			// Linting it would report classes the source never contained, and
			// counting it would dilute the score's denominator.
			if !list.Resolved {
				continue
			}
			report.Scanned.ClassLists++
			report.Scanned.Utilities += len(splitUtilities(list))
			for _, finding := range inspect(relative, list, syntax) {
				finding.Severity = config.severityFor(finding.Rule)
				if finding.Severity == SeverityOff {
					continue
				}
				if finding.Rule == "no-arbitrary-value" && config.AllowedArbitrary[finding.Class] {
					continue
				}
				if baseline.suppresses(finding) {
					report.Suppressed++
					finding.Scored = config.scores(finding)
					suppressed = append(suppressed, finding)
					continue
				}
				report.Findings = append(report.Findings, finding)
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	sortFindings(report.Findings)

	for index := range report.Findings {
		report.Findings[index].Scored = config.scores(report.Findings[index])
	}

	report.Score = transfer(weightedDensity(report.Findings, report.Scanned, config, nil))
	// The same arithmetic over the findings a baseline hides, so accepted debt
	// stays visible rather than disappearing into a file.
	unsuppressed := append(append([]Finding(nil), report.Findings...), suppressed...)
	report.ScoreExcludingBaseline = transfer(weightedDensity(unsuppressed, report.Scanned, config, nil))
	report.Categories = categoryScores(report.Findings, report.Scanned, config)
	report.ConfiguredRules = configuredRules(config)
	return report, nil
}

func loadThemes(fsys fs.FS, layout tailwind.Layout) ([]resolvedTheme, []ReportDiagnostic, error) {
	themes := make([]resolvedTheme, 0, len(layout.Packages))
	diagnostics := make([]tailwind.Diagnostic, 0)
	for _, pkg := range layout.Packages {
		adapter, found := tailwind.AdapterFor(pkg.Version)
		if !found {
			diagnostics = append(diagnostics, tailwind.Diagnostic{
				Kind: tailwind.DiagnosticUnknownVersion, File: pkg.Dir,
				Message: "Could not determine a supported Tailwind version for this package.",
			})
			continue
		}
		theme, err := adapter.Load(fsys, pkg)
		if err != nil {
			return nil, nil, fmt.Errorf("load Tailwind theme for %s: %w", pkg.Dir, err)
		}
		themes = append(themes, resolvedTheme{packageDirectory: pkg.Dir, theme: theme})
		diagnostics = append(diagnostics, theme.Diagnostics...)
	}
	tailwind.SortDiagnostics(diagnostics)
	reportDiagnostics := make([]ReportDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		reportDiagnostics = append(reportDiagnostics, ReportDiagnostic{
			Kind: string(diagnostic.Kind), File: diagnostic.File,
			Line: diagnostic.Line, Column: diagnostic.Column, Message: diagnostic.Message,
		})
	}
	return themes, reportDiagnostics, nil
}

func syntaxForFile(file string, layout tailwind.Layout, themes []resolvedTheme, config Config) tailwind.UtilitySyntax {
	syntax := config.Syntax
	pkg, found := layout.PackageFor(file)
	if found {
		for _, resolved := range themes {
			if resolved.packageDirectory == pkg.Dir {
				syntax = resolved.theme.Syntax
				break
			}
		}
	}

	prefixConfigured := config.prefixConfigured || config.Syntax.Prefix != ""
	if prefixConfigured {
		syntax.Prefix = config.Syntax.Prefix
		syntax.PrefixIsVariant = config.Syntax.PrefixIsVariant
	}
	separatorConfigured := config.separatorConfigured ||
		(config.Syntax.Separator != "" && config.Syntax.Separator != tailwind.DefaultUtilitySyntax().Separator)
	if separatorConfigured {
		syntax.Separator = config.Syntax.Separator
	}
	return syntax
}

// utilityToken is one utility and where it sits in the source. Positions are
// only meaningful when the class list was taken verbatim from the file; a list
// assembled from the literal parts of an interpolated value has no single span,
// so every utility in it reports the position of the list itself.
type utilityToken struct {
	text   string
	line   int
	column int
}

func splitUtilities(list ClassList) []utilityToken {
	tokens := make([]utilityToken, 0, 8)
	position := &positionTracker{line: list.Line, column: list.Column}
	start := -1

	for index := 0; index <= len(list.Value); index++ {
		atEnd := index == len(list.Value)
		if !atEnd && !isSpace(list.Value[index]) {
			if start < 0 {
				start = index
				if list.Verbatim {
					tokens = append(tokens, utilityToken{line: position.line, column: position.column})
				} else {
					tokens = append(tokens, utilityToken{line: list.Line, column: list.Column})
				}
			}
		} else if start >= 0 {
			tokens[len(tokens)-1].text = list.Value[start:index]
			start = -1
		}
		if !atEnd {
			position.advance(list.Value[index])
		}
	}
	return tokens
}

// ambiguousConflictGroups are the groups where utilityGroup cannot separate a
// shorthand from a colour: border-r sets a width and border-gray-200 a colour,
// and both land in "border-". A conflict inside one of these may be a false
// positive, so it is reported at medium confidence and stays out of the score
// until the property taxonomy arrives with the token inventory.
var ambiguousConflictGroups = map[string]bool{
	"text-":   true,
	"bg-":     true,
	"border-": true,
}

func inspect(file string, list ClassList, syntax tailwind.UtilitySyntax) []Finding {
	findings := make([]Finding, 0)
	seen := make(map[string]string)
	variants := 0

	for _, token := range splitUtilities(list) {
		parsed := tailwind.ParseUtility(token.text, syntax)

		if parsed.HasArbitraryValue() {
			findings = append(findings, Finding{
				Rule: "no-arbitrary-value", Category: CategoryConsistency,
				Confidence: ConfidenceHigh,
				File:       file, Class: token.text,
				Line: token.line, Column: token.column,
				Message: "Avoid arbitrary values; prefer a named design token.",
			})
		}

		if len(parsed.Variants) > 0 {
			variants++
		}

		group := tailwind.UtilityGroup(parsed.Base)
		if group == "" {
			continue
		}
		key := parsed.VariantKey() + "|" + group
		if previous, ok := seen[key]; ok {
			confidence := ConfidenceHigh
			if ambiguousConflictGroups[group] {
				confidence = ConfidenceMedium
			}
			findings = append(findings, Finding{
				Rule: "no-conflicting-utilities", Category: CategoryCorrectness,
				Confidence: confidence,
				File:       file, Class: list.Value,
				Line: token.line, Column: token.column,
				Message: fmt.Sprintf("%s conflicts with %s in the same variant.", previous, token.text),
			})
			continue
		}
		seen[key] = token.text
	}

	if variants >= 5 {
		findings = append(findings, Finding{
			Rule: "responsive-bloat", Category: CategoryMaintainability,
			Confidence: ConfidenceMedium,
			File:       file, Class: list.Value,
			Line: list.Line, Column: list.Column,
			Message: "Five or more variant utilities make this class list difficult to maintain.",
		})
	}
	return findings
}

// sortFindings gives the report a total order, so that identical input produces
// byte-identical output whatever order the filesystem hands files over in.
func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		switch {
		case left.File != right.File:
			return left.File < right.File
		case left.Line != right.Line:
			return left.Line < right.Line
		case left.Column != right.Column:
			return left.Column < right.Column
		case left.Rule != right.Rule:
			return left.Rule < right.Rule
		}
		return left.Message < right.Message
	})
}
