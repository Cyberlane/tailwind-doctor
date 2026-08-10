package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/plugins"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

var sourceExtensions = map[string]bool{
	".astro": true, ".cjs": true, ".css": true, ".cts": true, ".html": true,
	".js": true, ".jsx": true, ".mdx": true, ".mjs": true, ".mts": true,
	".ts": true, ".tsx": true, ".vue": true, ".svelte": true,
}

type Finding struct {
	Rule      string   `json:"rule"`
	Category  Category `json:"category"`
	Message   string   `json:"message"`
	File      string   `json:"file"`
	Class     string   `json:"class"`
	Line      int      `json:"line"`
	Column    int      `json:"column"`
	EndLine   int      `json:"endLine"`
	EndColumn int      `json:"endColumn"`
	Severity  Severity `json:"severity"`
	// Confidence is decided by the rule, not by configuration. Only high
	// confidence moves the score by default.
	Confidence Confidence `json:"confidence"`
	// Scored records whether this finding moved the score, so a reader never has
	// to re-derive the severity and confidence rules to explain the number.
	Scored bool `json:"scored"`

	// replacement is populated only when static token evidence proves an exact,
	// source-level substitution. It is deliberately not part of the report
	// contract: --fix consumes it internally, while users see the same stable
	// finding schema whether or not a run permits writes.
	replacement string
	fixable     bool
}

type resolvedTheme struct {
	packageDirectory string
	version          tailwind.Version
	theme            tailwind.Theme
	resolvedLists    int
	unresolvedLists  int
	ambiguousLists   int
	usedTokens       map[string]bool
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
		Packages:      []TailwindPackageReport{},
		Tokens:        []TokenPackageReport{},
		Accessibility: AccessibilityReport{UnknownReasons: []AccessibilityUnknownReason{}},
		Coverage:      CoverageReport{ResolutionPercent: 100},
		Findings:      []Finding{},
	}
	contrastUnknownReasons := map[string]int{}
	rootDirectory, err := os.OpenRoot(root)
	if err != nil {
		return Report{}, fmt.Errorf("open analysis root: %w", err)
	}
	defer rootDirectory.Close()
	rootFileSystem := rootDirectory.FS()
	ignores := newIgnoreRules(config.IgnorePaths)
	if config.RespectGitignore {
		if err := loadGitignoreFiles(root, ignores); err != nil {
			return Report{}, err
		}
	}
	layout, err := tailwind.DiscoverFiltered(rootFileSystem, ignores.ignores)
	if err != nil {
		return Report{}, fmt.Errorf("discover Tailwind packages: %w", err)
	}
	report.Packages = packageReports(layout)
	report.themes, report.Diagnostics, err = loadThemes(rootFileSystem, layout)
	if err != nil {
		return Report{}, err
	}
	// Suppressed findings are kept, unreported, only so the score the project
	// would have without its baseline can be computed.
	suppressed := []Finding{}
	unscopedClassLists := 0

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
			if relative != "." && ignores.ignores(relative, true) {
				return filepath.SkipDir
			}
			return nil
		}
		// WalkDir does not follow directory symlinks, but ReadFile would follow a
		// source-file symlink. Skipping both keeps analysis inside the requested
		// tree and ensures --fix can never modify a target outside it.
		if entry.Type()&os.ModeSymlink != 0 {
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
		syntax, resolvedTheme := analysisContextForFile(relative, layout, report.themes, config)
		for _, list := range Extract(path, string(content)) {
			if list.Resolved {
				report.Coverage.ResolvedClassLists++
			} else {
				report.Coverage.UnresolvedClassLists++
			}
			if resolvedTheme == nil && len(report.themes) > 0 {
				unscopedClassLists++
				if list.Resolved {
					report.Coverage.UnscopedClassLists++
				}
			}
			// An unresolved site names an expression, not a set of utilities.
			// Linting it would report classes the source never contained, and
			// counting it would dilute the score's denominator.
			if !list.Resolved {
				if resolvedTheme != nil {
					resolvedTheme.unresolvedLists++
				}
				continue
			}
			if resolvedTheme != nil {
				resolvedTheme.resolvedLists++
			}
			report.Scanned.ClassLists++
			var inventory *tokens.Inventory
			allowSuggestions := false
			if resolvedTheme != nil {
				inventory = resolvedTheme.theme.Inventory
				allowSuggestions = !resolvedTheme.theme.Degraded
			}
			for _, utility := range splitUtilities(list) {
				parsed := tailwind.ParseUtility(utility.text, syntax)
				if parsed.Recognized && tailwind.IsKnownUtility(parsed.Base, inventory) {
					report.Scanned.Utilities++
				}
			}
			findings, usedTokens := inspectWithInventory(relative, list, syntax, inventory, allowSuggestions)
			trustedContrastTheme := resolvedTheme != nil && !resolvedTheme.theme.Degraded &&
				plugins.Complete(resolvedTheme.theme.PluginCoverage)
			contrast := inspectContrast(relative, list, syntax, inventory, trustedContrastTheme)
			report.Scanned.ColorPairs += contrast.resolvedPairs
			for reason, count := range contrast.unknownReasons {
				contrastUnknownReasons[reason] += count
			}
			findings = append(findings, contrast.findings...)
			if resolvedTheme != nil {
				for _, token := range usedTokens {
					resolvedTheme.usedTokens[tokenIdentity(token)] = true
				}
			}
			for _, finding := range findings {
				recordFinding(&report, &suppressed, finding, config, baseline)
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	totalClassLists := report.Coverage.ResolvedClassLists + report.Coverage.UnresolvedClassLists
	if totalClassLists > 0 {
		report.Coverage.ResolutionPercent = report.Coverage.ResolvedClassLists * 100 / totalClassLists
	}
	if unscopedClassLists > 0 {
		for index := range report.themes {
			report.themes[index].ambiguousLists = unscopedClassLists
		}
	}
	report.Accessibility = summarizeAccessibility(report.Scanned.ColorPairs, contrastUnknownReasons)

	tokenAnalysis := analyzeTokens(report.themes)
	report.Tokens = tokenAnalysis.packages
	report.Scanned.Tokens = tokenAnalysis.projectTokens
	report.Scanned.HighConfidenceTokens = tokenAnalysis.highConfidenceTokens
	report.Scanned.MediumConfidenceTokens = tokenAnalysis.mediumConfidenceTokens
	for _, finding := range tokenAnalysis.findings {
		recordFinding(&report, &suppressed, finding, config, baseline)
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

func packageReports(layout tailwind.Layout) []TailwindPackageReport {
	packages := make([]TailwindPackageReport, 0, len(layout.Packages))
	for _, pkg := range layout.Packages {
		evidence := make([]TailwindEvidenceReport, 0, len(pkg.Evidence))
		for _, item := range pkg.Evidence {
			evidence = append(evidence, TailwindEvidenceReport{
				Signal: item.Signal, File: item.File, Detail: item.Detail,
			})
		}
		packages = append(packages, TailwindPackageReport{
			Directory: pkg.Dir, Version: pkg.Version, UnsupportedVersion: pkg.UnsupportedVersion,
			ManifestFile: pkg.ManifestFile, ConfigFile: pkg.ConfigFile,
			Entries: append([]string{}, pkg.Entries...), Evidence: evidence,
		})
	}
	return packages
}

func loadGitignoreFiles(root string, ignores *ignoreRules) error {
	return filepath.WalkDir(root, func(file string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist", "build", ".next", "vendor":
				if relative != "." {
					return filepath.SkipDir
				}
			}
			if relative != "." && ignores.ignores(relative, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != ".gitignore" {
			return nil
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", relative, err)
		}
		ignores.addGitignore(filepath.ToSlash(filepath.Dir(relative)), string(content))
		return nil
	})
}

func loadThemes(fsys fs.FS, layout tailwind.Layout) ([]resolvedTheme, []ReportDiagnostic, error) {
	themes := make([]resolvedTheme, 0, len(layout.Packages))
	diagnostics := make([]tailwind.Diagnostic, 0)
	for _, pkg := range layout.Packages {
		adapter, found := tailwind.AdapterFor(pkg.Version)
		if !found {
			message := "Could not determine a supported Tailwind version for this package."
			if pkg.UnsupportedVersion != "" {
				message = fmt.Sprintf("Tailwind v%s is not supported; token-dependent rules are disabled for this package.", pkg.UnsupportedVersion)
			}
			diagnostics = append(diagnostics, tailwind.Diagnostic{
				Kind: tailwind.DiagnosticUnknownVersion, File: pkg.Dir,
				Message: message,
			})
			continue
		}
		theme, err := adapter.Load(fsys, pkg)
		if err != nil {
			return nil, nil, fmt.Errorf("load Tailwind theme for %s: %w", pkg.Dir, err)
		}
		themes = append(themes, resolvedTheme{
			packageDirectory: pkg.Dir, version: pkg.Version, theme: theme,
			usedTokens: map[string]bool{},
		})
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
	syntax, _ := analysisContextForFile(file, layout, themes, config)
	return syntax
}

func analysisContextForFile(file string, layout tailwind.Layout, themes []resolvedTheme, config Config) (tailwind.UtilitySyntax, *resolvedTheme) {
	syntax := config.Syntax
	var matched *resolvedTheme
	pkg, found := layout.PackageFor(file)
	if found {
		for index := range themes {
			if themes[index].packageDirectory == pkg.Dir {
				matched = &themes[index]
				syntax = matched.theme.Syntax
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
	return syntax, matched
}

func recordFinding(report *Report, suppressed *[]Finding, finding Finding, config Config, baseline *Baseline) {
	if finding.EndLine == 0 || finding.EndColumn == 0 {
		finding.EndLine, finding.EndColumn = endPosition(finding.Line, finding.Column, finding.Class)
	}
	finding.Severity = config.severityFor(finding.Rule)
	if finding.Severity == SeverityOff {
		return
	}
	if finding.Rule == "no-arbitrary-value" && config.AllowedArbitrary[finding.Class] {
		return
	}
	if baseline.suppresses(finding) {
		report.Suppressed++
		finding.Scored = config.scores(finding)
		*suppressed = append(*suppressed, finding)
		return
	}
	report.Findings = append(report.Findings, finding)
}

func endPosition(line, column int, text string) (int, int) {
	position := positionTracker{line: line, column: column}
	for index := 0; index < len(text); index++ {
		position.advance(text[index])
	}
	return position.line, position.column
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

var ambiguousConflictGroups = map[string]bool{
	"text-":   true,
	"bg-":     true,
	"border-": true,
}

func inspect(file string, list ClassList, syntax tailwind.UtilitySyntax) []Finding {
	findings, _ := inspectWithInventory(file, list, syntax, nil, false)
	return findings
}

func inspectWithInventory(file string, list ClassList, syntax tailwind.UtilitySyntax, inventory *tokens.Inventory, allowSuggestions bool) ([]Finding, []tokens.Token) {
	findings := make([]Finding, 0)
	usedTokens := make([]tokens.Token, 0)
	seen := make(map[string]string)
	seenProperties := make([]observedProperties, 0)
	variantUtilities := 0
	knownUtilities := 0

	for _, token := range splitUtilities(list) {
		parsed := tailwind.ParseUtility(token.text, syntax)
		if !parsed.Recognized || !tailwind.IsKnownUtility(parsed.Base, inventory) {
			continue
		}
		knownUtilities++
		meaning := tailwind.ClassifyUtility(parsed.Base, inventory)

		if parsed.HasArbitraryValue() {
			message := "Avoid arbitrary values; prefer a named design token."
			replacement := ""
			if allowSuggestions && inventory != nil && meaning.ArbitraryValue != "" {
				matched, found := inventory.Lookup(meaning.Family, meaning.ArbitraryValue)
				if !found && meaning.Family == tokens.FamilySpacing {
					if multiplier, generated := inventory.SpacingMultiple(meaning.ArbitraryValue); generated {
						matched = tokens.Token{Family: tokens.FamilySpacing, Name: multiplier, Path: "--spacing"}
						found = true
					}
				}
				if found {
					replacement = replaceArbitraryValue(token.text, matched.Name)
					message = fmt.Sprintf("Avoid arbitrary values; use %s, which matches %s.",
						replacement, tokenLabel(matched))
				}
			}
			findings = append(findings, Finding{
				Rule: "no-arbitrary-value", Category: CategoryConsistency,
				Confidence: ConfidenceHigh,
				File:       file, Class: token.text,
				Line: token.line, Column: token.column,
				EndLine: token.line, EndColumn: token.column + len(token.text),
				Message:     message,
				replacement: replacement,
				fixable:     list.Verbatim && replacement != "",
			})
		}

		if inventory != nil && meaning.Family != "" && meaning.TokenName != "" {
			if used, found := inventory.ByName(meaning.Family, meaning.TokenName); found {
				usedTokens = append(usedTokens, used)
			} else if meaning.Family == tokens.FamilySpacing && tokens.IsSpacingMultiplier(meaning.TokenName) {
				if generator, generated := inventory.ByName(tokens.FamilySpacing, "DEFAULT"); generated {
					usedTokens = append(usedTokens, generator)
				}
			}
		}
		if inventory != nil && meaning.Family == tokens.FamilyFontSize {
			if _, modifier, found := strings.Cut(strings.TrimPrefix(parsed.Base, "text-"), "/"); found {
				if used, exists := inventory.ByName(tokens.FamilyLineHeight, modifier); exists {
					usedTokens = append(usedTokens, used)
				}
			}
		}
		if inventory != nil {
			for _, variant := range parsed.Variants {
				name := strings.TrimPrefix(strings.TrimPrefix(variant, "min-"), "max-")
				if used, found := inventory.ByName(tokens.FamilyBreakpoint, name); found {
					usedTokens = append(usedTokens, used)
				}
			}
		}

		if len(parsed.Variants) > 0 {
			variantUtilities++
		}

		group := meaning.Property
		if group == "" {
			group = tailwind.UtilityGroup(parsed.Base)
		}
		if group == "" {
			continue
		}
		key := parsed.VariantKey() + "|" + group
		if previous, ok := seen[key]; ok {
			confidence := ConfidenceHigh
			if inventory == nil && ambiguousConflictGroups[group] {
				confidence = ConfidenceMedium
			}
			findings = append(findings, Finding{
				Rule: "no-conflicting-utilities", Category: CategoryCorrectness,
				Confidence: confidence,
				File:       file, Class: list.Value,
				Line: token.line, Column: token.column,
				EndLine: token.line, EndColumn: token.column + len(token.text),
				Message: fmt.Sprintf("%s conflicts with %s in the same variant.", previous, token.text),
			})
			continue
		}
		seen[key] = token.text

		properties := utilityProperties(parsed.Base)
		if len(properties) > 0 {
			variantKey := parsed.VariantKey()
			for _, previous := range seenProperties {
				if previous.variant != variantKey || previous.properties.equal(properties) ||
					!previous.properties.overlaps(properties) {
					continue
				}
				findings = append(findings, Finding{
					Rule: "no-overlapping-utilities", Category: CategoryCorrectness,
					Confidence: ConfidenceMedium,
					File:       file, Class: list.Value, Line: token.line, Column: token.column,
					EndLine: token.line, EndColumn: token.column + len(token.text),
					Message: fmt.Sprintf("%s and %s set overlapping properties in the same variant.",
						previous.utility, token.text),
				})
				break
			}
			seenProperties = append(seenProperties, observedProperties{
				variant: variantKey, utility: token.text, properties: properties,
			})
		}
	}

	if knownUtilities > 0 && variantUtilities >= 4 && variantUtilities*100/knownUtilities >= 60 {
		findings = append(findings, Finding{
			Rule: "variant-density", Category: CategoryMaintainability,
			Confidence: ConfidenceMedium,
			File:       file, Class: list.Value,
			Line: list.Line, Column: list.Column,
			Message: "Variant utilities make up at least 60% of this class list; consider extracting a component or variant.",
		})
	}
	return findings, usedTokens
}

type propertySet map[string]bool

type observedProperties struct {
	variant    string
	utility    string
	properties propertySet
}

func (properties propertySet) overlaps(other propertySet) bool {
	for property := range properties {
		if other[property] {
			return true
		}
	}
	return false
}

func (properties propertySet) equal(other propertySet) bool {
	if len(properties) != len(other) {
		return false
	}
	for property := range properties {
		if !other[property] {
			return false
		}
	}
	return true
}

func utilityProperties(base string) propertySet {
	for _, family := range []struct {
		prefix string
		name   string
	}{
		{prefix: "p", name: "padding"},
		{prefix: "m", name: "margin"},
	} {
		if base == family.prefix || !strings.HasPrefix(base, family.prefix+"-") &&
			!strings.HasPrefix(base, family.prefix+"x-") && !strings.HasPrefix(base, family.prefix+"y-") &&
			!strings.HasPrefix(base, family.prefix+"t-") && !strings.HasPrefix(base, family.prefix+"r-") &&
			!strings.HasPrefix(base, family.prefix+"b-") && !strings.HasPrefix(base, family.prefix+"l-") {
			continue
		}
		direction := strings.TrimPrefix(strings.SplitN(base, "-", 2)[0], family.prefix)
		switch direction {
		case "":
			return propertySet{family.name + "-top": true, family.name + "-right": true,
				family.name + "-bottom": true, family.name + "-left": true}
		case "x":
			return propertySet{family.name + "-right": true, family.name + "-left": true}
		case "y":
			return propertySet{family.name + "-top": true, family.name + "-bottom": true}
		case "t":
			return propertySet{family.name + "-top": true}
		case "r":
			return propertySet{family.name + "-right": true}
		case "b":
			return propertySet{family.name + "-bottom": true}
		case "l":
			return propertySet{family.name + "-left": true}
		}
	}
	return nil
}

func replaceArbitraryValue(utility, name string) string {
	start := strings.IndexByte(utility, '[')
	if start < 0 {
		return utility
	}
	end := strings.IndexByte(utility[start:], ']')
	if end < 0 {
		return utility
	}
	end += start
	return utility[:start] + name + utility[end+1:]
}

func tokenLabel(token tokens.Token) string {
	if token.Path != "" {
		return token.Path
	}
	return string(token.Family) + " token " + token.Name
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
