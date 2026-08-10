package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
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
	fmt.Fprintf(writer, "Scanned %d file(s), %d class list(s), %d utilities\n",
		report.Scanned.Files, report.Scanned.ClassLists, report.Scanned.Utilities)
	fmt.Fprintf(writer, "Resolved %d/%d candidate class list(s) (%d%%); %d resolved list(s) were outside a Tailwind package\n",
		report.Coverage.ResolvedClassLists,
		report.Coverage.ResolvedClassLists+report.Coverage.UnresolvedClassLists,
		report.Coverage.ResolutionPercent, report.Coverage.UnscopedClassLists)
	if report.Accessibility.ResolvedColorPairs > 0 || report.Accessibility.UnknownColorPairs > 0 {
		fmt.Fprintf(writer, "Resolved %d color pair(s); %d candidate pair(s) remain unknown\n",
			report.Accessibility.ResolvedColorPairs, report.Accessibility.UnknownColorPairs)
	}
	if len(report.Tokens) > 0 {
		fmt.Fprintf(writer, "Inventoried %d project token(s) across %d Tailwind package(s)\n",
			report.Scanned.Tokens, len(report.Tokens))
	}
	if len(report.Packages) > 0 {
		fmt.Fprintf(writer, "Detected %d Tailwind package(s) from static evidence\n", len(report.Packages))
	}

	if len(report.Accessibility.UnknownReasons) > 0 {
		fmt.Fprintln(writer, "\nAccessibility coverage gaps:")
		for _, reason := range report.Accessibility.UnknownReasons {
			fmt.Fprintf(writer, "- %s: %d\n", reason.Reason, reason.Count)
		}
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
	fmt.Fprintf(writer, "\n%d finding(s):\n", len(report.Findings))
	for _, finding := range report.Findings {
		fmt.Fprintf(writer, "- [%s] %s:%d:%d: %s",
			finding.Rule, finding.File, finding.Line, finding.Column, finding.Message)
		if !finding.Scored {
			fmt.Fprintf(writer, " (%s confidence, not scored)", finding.Confidence)
		}
		fmt.Fprintln(writer)
	}
	if report.Suppressed > 0 {
		fmt.Fprintf(writer, "\n%d finding(s) suppressed by the baseline.\n", report.Suppressed)
	}
}
