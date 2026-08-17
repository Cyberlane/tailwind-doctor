package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/plugins"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

// MaximumScore is the score a project with no findings receives, and therefore
// the highest threshold a caller can meaningfully gate on.
const MaximumScore = 100

// SchemaVersion is the version of the JSON report. A consumer reads this before
// anything else; the shape below only ever changes with it.
const SchemaVersion = 4

// Version is the build's version string, reported in JSON and SARIF so a finding
// can be traced back to the code that produced it.
var Version = "dev"

func init() {
	// `go install module@version` records the module version in Go build info.
	// Release archives override Version with -ldflags, while a checkout build
	// remains "dev". This keeps every supported installation path traceable.
	if Version != "dev" {
		return
	}
	if build, available := debug.ReadBuildInfo(); available &&
		build.Main.Version != "" && build.Main.Version != "(devel)" {
		Version = strings.TrimPrefix(build.Main.Version, "v")
	}
}

type ToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// CategoryWeights is published in every report so the score can be recomputed
// from its own output rather than taken on trust.
type CategoryWeights struct {
	Accessibility   int `json:"accessibility"`
	Correctness     int `json:"correctness"`
	Consistency     int `json:"consistency"`
	Maintainability int `json:"maintainability"`
}

// ScoreModel describes the arithmetic that produced the score. Two scores are
// comparable only when their Version and the reporting projects' rule
// configuration both match, which is why both travel with every report.
type ScoreModel struct {
	Version          int             `json:"version"`
	TransferFunction string          `json:"transferFunction"`
	HalfScoreDensity float64         `json:"halfScoreDensity"`
	Weights          CategoryWeights `json:"weights"`
}

// ConfiguredRule records what each rule actually did on this run. Turning a rule
// off raises the score, which cannot be prevented — so it is disclosed, and a
// published number becomes verifiable rather than merely asserted.
type ConfiguredRule struct {
	ID         string     `json:"id"`
	Severity   Severity   `json:"severity"`
	Confidence Confidence `json:"confidence"`
}

// ReportDiagnostic records configuration Tailwind Doctor could not read. It is
// separate from a finding because incomplete static configuration evidence must
// never affect the design-system score.
type ReportDiagnostic struct {
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

type TailwindEvidenceReport struct {
	Signal string `json:"signal"`
	File   string `json:"file"`
	Detail string `json:"detail"`
}

// TailwindPackageReport publishes the static evidence behind package scope and
// version selection so consumers can audit which configuration governed a file.
type TailwindPackageReport struct {
	Directory          string                   `json:"directory"`
	Version            tailwind.Version         `json:"version"`
	UnsupportedVersion string                   `json:"unsupportedVersion,omitempty"`
	ManifestFile       string                   `json:"manifestFile,omitempty"`
	ConfigFile         string                   `json:"configFile,omitempty"`
	Entries            []string                 `json:"entries"`
	Evidence           []TailwindEvidenceReport `json:"evidence"`
}

// TokenReport is one deterministic inventory entry and whether analysis found
// a named utility that consumes it.
type TokenReport struct {
	Family       tokens.Family `json:"family"`
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	Value        string        `json:"value"`
	Raw          string        `json:"raw"`
	Origin       tokens.Origin `json:"origin"`
	Declaration  tokens.Site   `json:"declaration"`
	Unresolvable bool          `json:"unresolvable"`
	Used         bool          `json:"used"`
}

// TokenPackageReport publishes the inventory and the evidence that controls
// unused-token confidence for one independently scoped Tailwind package.
type TokenPackageReport struct {
	Package              string             `json:"package"`
	TailwindVersion      tailwind.Version   `json:"tailwindVersion"`
	Confidence           Confidence         `json:"confidence"`
	ConfidenceReasons    []string           `json:"confidenceReasons"`
	ResolvedClassLists   int                `json:"resolvedClassLists"`
	UnresolvedClassLists int                `json:"unresolvedClassLists"`
	Plugins              []plugins.Coverage `json:"plugins"`
	Inventory            []TokenReport      `json:"inventory"`
	Unused               []TokenReport      `json:"unused"`
}

// AccessibilityUnknownReason counts one conservative boundary that prevented a
// candidate foreground/background context from becoming a measured pair.
type AccessibilityUnknownReason struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// AccessibilityReport publishes contrast coverage separately from findings. A
// context the static analyzer cannot resolve must stay visible without diluting
// the score or being misrepresented as an accessibility failure.
type AccessibilityReport struct {
	ResolvedColorPairs int                          `json:"resolvedColorPairs"`
	UnknownColorPairs  int                          `json:"unknownColorPairs"`
	UnknownReasons     []AccessibilityUnknownReason `json:"unknownReasons"`
}

// CoverageReport makes the static-analysis boundary explicit. A resolved class
// list contributes utilities and findings; an unresolved list contributes only
// to coverage, never to the score denominator.
type CoverageReport struct {
	ResolvedClassLists   int `json:"resolvedClassLists"`
	UnresolvedClassLists int `json:"unresolvedClassLists"`
	UnscopedClassLists   int `json:"unscopedClassLists"`
	ResolutionPercent    int `json:"resolutionPercent"`
}

type Report struct {
	SchemaVersion int      `json:"schemaVersion"`
	Tool          ToolInfo `json:"tool"`
	Score         int      `json:"score"`
	// ScoreExcludingBaseline is the score the project would have with no
	// suppressions. A baseline that made debt invisible would be a lie with a
	// file format.
	ScoreExcludingBaseline int                     `json:"scoreExcludingBaseline"`
	ScoreModel             ScoreModel              `json:"scoreModel"`
	Categories             []CategoryScore         `json:"categories"`
	Scanned                Scanned                 `json:"scanned"`
	ConfiguredRules        []ConfiguredRule        `json:"configuredRules"`
	Diagnostics            []ReportDiagnostic      `json:"diagnostics"`
	Packages               []TailwindPackageReport `json:"packages"`
	Tokens                 []TokenPackageReport    `json:"tokens"`
	Accessibility          AccessibilityReport     `json:"accessibility"`
	Coverage               CoverageReport          `json:"coverage"`
	Findings               []Finding               `json:"findings"`
	themes                 []resolvedTheme
	// Suppressed counts findings that matched the baseline. Reporting the count
	// keeps accepted debt visible rather than letting it vanish.
	Suppressed int `json:"suppressed"`
}

func scoreModel() ScoreModel {
	return ScoreModel{
		Version:          ScoreModelVersion,
		TransferFunction: "100 * H / (H + D)",
		HalfScoreDensity: 0.2,
		Weights: CategoryWeights{
			Accessibility:   int(categoryWeights[CategoryAccessibility]),
			Correctness:     int(categoryWeights[CategoryCorrectness]),
			Consistency:     int(categoryWeights[CategoryConsistency]),
			Maintainability: int(categoryWeights[CategoryMaintainability]),
		},
	}
}

func configuredRules(config Config) []ConfiguredRule {
	rules := make([]ConfiguredRule, 0, len(ruleRegistry))
	for _, rule := range ruleRegistry {
		rules = append(rules, ConfiguredRule{
			ID:         rule.ID,
			Severity:   config.severityFor(rule.ID),
			Confidence: rule.DefaultConfidence,
		})
	}
	return rules
}

func WriteJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteHuman(writer io.Writer, report Report) {
	fmt.Fprintf(writer, "Tailwind Doctor: %d/%d", report.Score, MaximumScore)
	if report.ScoreExcludingBaseline != report.Score {
		fmt.Fprintf(writer, " (%d ignoring baseline)", report.ScoreExcludingBaseline)
	}
	fmt.Fprintln(writer)
	writeChecks(writer, report)
	// A score computed while the theme applied to almost nothing looks exactly
	// as confident as a fully themed run, and a one-line coverage check is easy
	// to skim past. Half is the point where the themeless portion stops being
	// an edge of the project and becomes the project.
	if len(report.Packages) > 0 && report.Coverage.ResolvedClassLists > 0 &&
		report.Coverage.UnscopedClassLists*2 >= report.Coverage.ResolvedClassLists {
		fmt.Fprintf(writer, "Warning: %d of %d resolved class list(s) are outside every detected Tailwind package; theme-dependent rules ran without a theme for those files. The package root may be misdetected.\n",
			report.Coverage.UnscopedClassLists, report.Coverage.ResolvedClassLists)
	}

	fmt.Fprintln(writer)
	for _, category := range report.Categories {
		name := strings.ToUpper(string(category.Name[:1])) + string(category.Name[1:])
		if category.Score == nil {
			fmt.Fprintf(writer, "  %-16s not measured\n", name)
			continue
		}
		fmt.Fprintf(writer, "  %-16s %d\n", name, *category.Score)
	}

	if len(report.Diagnostics) > 0 {
		fmt.Fprintf(writer, "\nConfiguration (%d diagnostic(s)):\n", len(report.Diagnostics))
		for _, diagnostic := range report.Diagnostics {
			fmt.Fprintf(writer, "- [%s] %s:%d:%d: %s\n",
				diagnostic.Kind, diagnostic.File, diagnostic.Line, diagnostic.Column, diagnostic.Message)
		}
	}

	if len(report.Tokens) > 0 {
		fmt.Fprintln(writer, "\nToken coverage:")
		for _, tokenPackage := range report.Tokens {
			projectTokens := 0
			for _, token := range tokenPackage.Inventory {
				if token.Origin == tokens.OriginProject {
					projectTokens++
				}
			}
			fmt.Fprintf(writer, "- %s (Tailwind v%s): %d project token(s), %d unused, %d total inventory entries, %s confidence\n",
				tokenPackage.Package, tokenPackage.TailwindVersion, projectTokens,
				len(tokenPackage.Unused), len(tokenPackage.Inventory), tokenPackage.Confidence)
			for _, reason := range tokenPackage.ConfidenceReasons {
				fmt.Fprintf(writer, "  - %s\n", reason)
			}
		}
	}

	if len(report.Findings) == 0 {
		fmt.Fprintln(writer, "\nNo findings. Your class lists look healthy.")
		if report.Suppressed > 0 {
			fmt.Fprintf(writer, "\n%d finding(s) suppressed by the baseline.\n", report.Suppressed)
		}
		return
	}
	scored := 0
	for _, finding := range report.Findings {
		if finding.Scored {
			scored++
		}
	}
	unscored := len(report.Findings) - scored

	writeRuleSummary(writer, report.Findings)
	writeRepeatedArbitraryValues(writer, report.Findings)
	if scored == len(report.Findings) {
		fmt.Fprintf(writer, "\n%d finding(s):\n", len(report.Findings))
	} else {
		fmt.Fprintf(writer, "\n%d finding(s), %d scored:\n", len(report.Findings), scored)
	}
	writeGroupedFindings(writer, report.Findings)
	// Unscored findings are still real observations, but at medium or low
	// confidence they drown out the scored list in large projects. The machine
	// formats carry every finding; the human format carries the count.
	if unscored > 0 {
		fmt.Fprintf(writer, "\n%d unscored finding(s) hidden (medium or low confidence); use --json or --sarif to review them.\n", unscored)
	}
	fixableCount := 0
	for _, finding := range report.Findings {
		if finding.fixable {
			fixableCount++
		}
	}
	if fixableCount > 0 {
		fmt.Fprintf(writer, "\n%d finding(s) can be fixed automatically; run tw-doctor --fix.\n", fixableCount)
	}
	if report.Suppressed > 0 {
		fmt.Fprintf(writer, "\n%d finding(s) suppressed by the baseline.\n", report.Suppressed)
	}
}

// writeChecks renders the run's coverage as doctor-style check lines: each
// number arrives with a judgment (✓ fine, ✗ needs attention, • informational)
// instead of as bare telemetry. Reason-code detail stays in --json.
func writeChecks(writer io.Writer, report Report) {
	if len(report.Packages) > 0 {
		fmt.Fprintf(writer, "✓ %d Tailwind package(s) detected\n", len(report.Packages))
	} else {
		fmt.Fprintln(writer, "✗ No Tailwind package detected; theme-dependent rules are disabled")
	}
	if report.Scanned.Tokens > 0 {
		fmt.Fprintf(writer, "✓ Theme inventoried: %d project token(s)\n", report.Scanned.Tokens)
	}
	fmt.Fprintf(writer, "✓ Scanned %d file(s): %d class list(s), %d utilities\n",
		report.Scanned.Files, report.Scanned.ClassLists, report.Scanned.Utilities)

	totalClassLists := report.Coverage.ResolvedClassLists + report.Coverage.UnresolvedClassLists
	if report.Coverage.UnresolvedClassLists > 0 {
		fmt.Fprintf(writer, "✗ %d of %d class list(s) (%d%%) are dynamic expressions and were not analyzed\n",
			report.Coverage.UnresolvedClassLists, totalClassLists, 100-report.Coverage.ResolutionPercent)
	} else if totalClassLists > 0 {
		fmt.Fprintln(writer, "✓ Every class list resolved statically")
	}
	if len(report.Packages) > 0 {
		if report.Coverage.UnscopedClassLists > 0 {
			fmt.Fprintf(writer, "✗ %d resolved class list(s) matched no Tailwind package\n",
				report.Coverage.UnscopedClassLists)
		} else if report.Coverage.ResolvedClassLists > 0 {
			fmt.Fprintln(writer, "✓ Every resolved class list matched a Tailwind package")
		}
	}

	if report.Accessibility.ResolvedColorPairs > 0 || report.Accessibility.UnknownColorPairs > 0 {
		contrastEnabled := false
		for _, rule := range report.ConfiguredRules {
			if rule.ID == "color-contrast" && rule.Severity != SeverityOff {
				contrastEnabled = true
			}
		}
		if contrastEnabled {
			fmt.Fprintf(writer, "✓ %d color pair(s) measured, %d unknown; unknown reasons are in --json\n",
				report.Accessibility.ResolvedColorPairs, report.Accessibility.UnknownColorPairs)
		} else {
			fmt.Fprintf(writer, "• %d color pair(s) measured, %d unknown; enable the color-contrast rule to score accessibility\n",
				report.Accessibility.ResolvedColorPairs, report.Accessibility.UnknownColorPairs)
		}
	}
}

// findingsShown caps the findings list. The human report is for reading, not
// archiving: past this point more lines carry less information, and the
// machine formats stay exhaustive. Truncation happens at a file boundary so a
// file is never half-reported.
const findingsShown = 100

// writeGroupedFindings lists scored findings under one header per file, in the
// report's existing total order, truncating after findingsShown findings.
func writeGroupedFindings(writer io.Writer, findings []Finding) {
	printed := 0
	index := 0
	for index < len(findings) {
		if !findings[index].Scored {
			index++
			continue
		}
		file := findings[index].File
		block := []Finding{}
		for cursor := index; cursor < len(findings) && findings[cursor].File == file; cursor++ {
			if findings[cursor].Scored {
				block = append(block, findings[cursor])
			}
			index = cursor + 1
		}
		if printed >= findingsShown {
			remainingFindings := len(block)
			remainingFiles := 1
			for cursor := index; cursor < len(findings); cursor++ {
				if !findings[cursor].Scored {
					continue
				}
				remainingFindings++
				if findings[cursor].File != file {
					file = findings[cursor].File
					remainingFiles++
				}
			}
			fmt.Fprintf(writer, "… and %d more finding(s) in %d file(s); use --json or --sarif for the full list.\n",
				remainingFindings, remainingFiles)
			return
		}
		fmt.Fprintln(writer, file)
		for _, finding := range block {
			fmt.Fprintf(writer, "  %d:%d [%s] %s\n", finding.Line, finding.Column, finding.Rule, finding.Message)
		}
		printed += len(block)
	}
}

// sortByCountDescending orders a summary block by descending count with a
// name tie-break, so identical inputs always render identically.
func sortByCountDescending[T any](items []T, count func(T) int, name func(T) string) {
	sort.Slice(items, func(left, right int) bool {
		if count(items[left]) != count(items[right]) {
			return count(items[left]) > count(items[right])
		}
		return name(items[left]) < name(items[right])
	})
}

// repeatedValuesShown caps the repeated-values block: the point is the handful
// of missing tokens that dominate, not a second exhaustive list.
const repeatedValuesShown = 10

// writeRepeatedArbitraryValues aggregates no-arbitrary-value findings by the
// offending utility. One value used 165 times is one missing design token, not
// 165 separate problems, and this block is where that becomes visible.
func writeRepeatedArbitraryValues(writer io.Writer, findings []Finding) {
	type valueCount struct {
		class       string
		count       int
		replacement string
	}
	byClass := map[string]*valueCount{}
	for _, finding := range findings {
		if finding.Rule != "no-arbitrary-value" {
			continue
		}
		count, found := byClass[finding.Class]
		if !found {
			count = &valueCount{class: finding.Class}
			byClass[finding.Class] = count
		}
		count.count++
		if count.replacement == "" {
			count.replacement = finding.replacement
		}
	}
	repeated := make([]*valueCount, 0, len(byClass))
	for _, count := range byClass {
		if count.count > 1 {
			repeated = append(repeated, count)
		}
	}
	if len(repeated) == 0 {
		return
	}
	sortByCountDescending(repeated,
		func(value *valueCount) int { return value.count },
		func(value *valueCount) string { return value.class })
	fmt.Fprintln(writer, "\nRepeated arbitrary values:")
	shown := repeated
	if len(shown) > repeatedValuesShown {
		shown = shown[:repeatedValuesShown]
	}
	for _, count := range shown {
		fmt.Fprintf(writer, "- %d × %s", count.count, count.class)
		if count.replacement != "" {
			fmt.Fprintf(writer, " → %s", count.replacement)
		}
		fmt.Fprintln(writer)
	}
	if len(repeated) > len(shown) {
		fmt.Fprintf(writer, "- … and %d more repeated value(s)\n", len(repeated)-len(shown))
	}
}

// writeRuleSummary answers "what kind of debt dominates" before the per-file
// list answers "where". Rules are ordered by total count, then by name, so the
// output stays deterministic when counts tie.
func writeRuleSummary(writer io.Writer, findings []Finding) {
	type ruleCount struct {
		rule   string
		total  int
		scored int
	}
	byRule := map[string]*ruleCount{}
	for _, finding := range findings {
		count, found := byRule[finding.Rule]
		if !found {
			count = &ruleCount{rule: finding.Rule}
			byRule[finding.Rule] = count
		}
		count.total++
		if finding.Scored {
			count.scored++
		}
	}
	counts := make([]*ruleCount, 0, len(byRule))
	for _, count := range byRule {
		counts = append(counts, count)
	}
	sortByCountDescending(counts,
		func(count *ruleCount) int { return count.total },
		func(count *ruleCount) string { return count.rule })
	fmt.Fprintln(writer, "\nFindings by rule:")
	for _, count := range counts {
		if count.scored == count.total {
			fmt.Fprintf(writer, "- %s: %d\n", count.rule, count.total)
			continue
		}
		fmt.Fprintf(writer, "- %s: %d (%d scored)\n", count.rule, count.total, count.scored)
	}
}
