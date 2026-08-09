package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MaximumScore is the score a project with no findings receives, and therefore
// the highest threshold a caller can meaningfully gate on.
const MaximumScore = 100

// SchemaVersion is the version of the JSON report. A consumer reads this before
// anything else; the shape below only ever changes with it.
const SchemaVersion = 1

// Version is the build's version string, reported in JSON and SARIF so a finding
// can be traced back to the code that produced it.
const Version = "dev"

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

type Report struct {
	SchemaVersion int      `json:"schemaVersion"`
	Tool          ToolInfo `json:"tool"`
	Score         int      `json:"score"`
	// ScoreExcludingBaseline is the score the project would have with no
	// suppressions. A baseline that made debt invisible would be a lie with a
	// file format.
	ScoreExcludingBaseline int                `json:"scoreExcludingBaseline"`
	ScoreModel             ScoreModel         `json:"scoreModel"`
	Categories             []CategoryScore    `json:"categories"`
	Scanned                Scanned            `json:"scanned"`
	ConfiguredRules        []ConfiguredRule   `json:"configuredRules"`
	Diagnostics            []ReportDiagnostic `json:"diagnostics"`
	Findings               []Finding          `json:"findings"`
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
