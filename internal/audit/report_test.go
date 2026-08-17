package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Two codebases with proportionally identical debt must score alike. This is the
// milestone's definition of done: under the old formula the large one scored 0
// and the small one 90.
func TestScoreIsSizeNormalized(t *testing.T) {
	small := generateProject(t, 5, 40, 2)   // 200 utilities, 10 arbitrary values
	large := generateProject(t, 250, 40, 2) // 10000 utilities, 500 arbitrary values

	smallReport, err := Run(small)
	if err != nil {
		t.Fatalf("Run small: %v", err)
	}
	largeReport, err := Run(large)
	if err != nil {
		t.Fatalf("Run large: %v", err)
	}

	if smallReport.Scanned.Utilities != 200 || largeReport.Scanned.Utilities != 10000 {
		t.Fatalf("fixtures are not the sizes intended: %d and %d utilities",
			smallReport.Scanned.Utilities, largeReport.Scanned.Utilities)
	}
	if len(largeReport.Findings) != 50*len(smallReport.Findings) {
		t.Fatalf("debt is not proportional: %d findings against %d",
			len(largeReport.Findings), len(smallReport.Findings))
	}
	if difference := smallReport.Score - largeReport.Score; difference > 1 || difference < -1 {
		t.Errorf("scores %d and %d differ by %d for a fiftyfold size difference at identical debt density",
			smallReport.Score, largeReport.Score, difference)
	}
	if largeReport.Score == 0 {
		t.Error("the large project scored zero, which is the failure this model replaced")
	}
}

// generateProject writes files of a fixed shape: each holds one class list of
// utilitiesPerFile utilities, of which arbitraryPerFile are arbitrary values.
// Generated rather than committed, so the ratio is legible at the call site.
func generateProject(t *testing.T, files, utilitiesPerFile, arbitraryPerFile int) string {
	t.Helper()
	root := t.TempDir()
	for file := 0; file < files; file++ {
		// gap- is not a group utilityGroup recognises, so these utilities carry
		// exactly the debt intended: arbitrary values, and no conflicts to
		// confound the ratio being measured.
		classes := make([]string, 0, utilitiesPerFile)
		for index := 0; index < arbitraryPerFile; index++ {
			classes = append(classes, fmt.Sprintf("gap-[%dpx]", index))
		}
		for index := len(classes); index < utilitiesPerFile; index++ {
			classes = append(classes, fmt.Sprintf("gap-%d", index))
		}
		writeFile(t, root, fmt.Sprintf("src/component%d.html", file),
			`<div class="`+strings.Join(classes, " ")+`"></div>`)
	}
	return root
}

// More debt never scores better. Without this a weight change can invert the
// metric and nothing notices.
func TestScoreIsMonotonicInDebt(t *testing.T) {
	previous := MaximumScore + 1
	for _, arbitrary := range []int{0, 1, 2, 4, 8, 16, 32} {
		report, err := Run(generateProject(t, 4, 40, arbitrary))
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if report.Score > previous {
			t.Errorf("%d arbitrary values per file scored %d, better than %d with less debt",
				arbitrary, report.Score, previous)
		}
		previous = report.Score
	}
}

func TestJSONCarriesTheVersionedSchema(t *testing.T) {
	report, err := Run(generateProject(t, 2, 40, 2))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var buffer bytes.Buffer
	if err := WriteJSON(&buffer, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var decoded struct {
		SchemaVersion int `json:"schemaVersion"`
		Tool          struct {
			Name string `json:"name"`
		} `json:"tool"`
		Score      int `json:"score"`
		ScoreModel struct {
			Version          int     `json:"version"`
			HalfScoreDensity float64 `json:"halfScoreDensity"`
		} `json:"scoreModel"`
		Categories []struct {
			Name  string `json:"name"`
			Score *int   `json:"score"`
		} `json:"categories"`
		Scanned struct {
			Utilities  int `json:"utilities"`
			Tokens     int `json:"tokens"`
			ColorPairs int `json:"colorPairs"`
		} `json:"scanned"`
		ConfiguredRules []struct {
			ID string `json:"id"`
		} `json:"configuredRules"`
		Diagnostics   []ReportDiagnostic   `json:"diagnostics"`
		Tokens        []TokenPackageReport `json:"tokens"`
		Accessibility AccessibilityReport  `json:"accessibility"`
		Findings      []struct {
			Category   string `json:"category"`
			Confidence string `json:"confidence"`
			Scored     bool   `json:"scored"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.SchemaVersion != SchemaVersion {
		t.Errorf("schemaVersion = %d, want %d", decoded.SchemaVersion, SchemaVersion)
	}
	if decoded.Tool.Name != "tw-doctor" {
		t.Errorf("tool.name = %q", decoded.Tool.Name)
	}
	if decoded.ScoreModel.Version != ScoreModelVersion || decoded.ScoreModel.HalfScoreDensity != 0.2 {
		t.Errorf("scoreModel = %#v", decoded.ScoreModel)
	}
	if len(decoded.Categories) != len(categoryOrder) {
		t.Errorf("got %d categories, want %d", len(decoded.Categories), len(categoryOrder))
	}
	for _, category := range decoded.Categories {
		if category.Name == string(CategoryAccessibility) && category.Score != nil {
			t.Errorf("accessibility rule is disabled by default and must report null, got %d", *category.Score)
		}
	}
	if len(decoded.ConfiguredRules) != len(ruleRegistry) {
		t.Errorf("configuredRules lists %d rules, want %d", len(decoded.ConfiguredRules), len(ruleRegistry))
	}
	if decoded.Diagnostics == nil {
		t.Fatal("diagnostics must be an array, not null")
	}
	if decoded.Tokens == nil {
		t.Fatal("tokens must be an array, not null")
	}
	if decoded.Accessibility.UnknownReasons == nil {
		t.Fatal("accessibility.unknownReasons must be an array, not null")
	}
	if len(decoded.Findings) == 0 {
		t.Fatal("the fixture has arbitrary values and should report findings")
	}
	for _, finding := range decoded.Findings {
		if finding.Category == "" || finding.Confidence == "" {
			t.Errorf("finding is missing category or confidence: %#v", finding)
		}
		if !finding.Scored {
			t.Errorf("a high-confidence arbitrary value should be scored: %#v", finding)
		}
	}
}

func TestReportPublishesConfigurationDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0"}}`)
	writeFile(t, root, "tailwind.config.js", `const defaults = require("tailwindcss/defaultTheme")
module.exports = { theme: { extend: { fontFamily: { ...defaults.fontFamily } } } }`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Kind != "unreadable-config" {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
	if report.Score != MaximumScore || len(report.Findings) != 0 {
		t.Fatalf("configuration diagnostic changed findings or score: score %d, findings %#v",
			report.Score, report.Findings)
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	for _, want := range []string{"Configuration (1 diagnostic(s))", "[unreadable-config]", "tailwind.config.js"} {
		if !strings.Contains(buffer.String(), want) {
			t.Errorf("human output is missing %q:\n%s", want, buffer.String())
		}
	}
}

func TestUnsupportedTailwindMajorDisablesThemeWithoutFailing(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^5.0.0"}}`)
	writeFile(t, root, "tailwind.config.js", `module.exports = { theme: {} }`)
	writeFile(t, root, "page.html", `<div class="p-4"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.themes) != 0 {
		t.Fatalf("unsupported version loaded themes: %+v", report.themes)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Kind != "unknown-version" ||
		!strings.Contains(report.Diagnostics[0].Message, "v5") {
		t.Fatalf("diagnostics = %+v", report.Diagnostics)
	}
}

// A baseline must not make debt invisible. Both numbers are always reported.
func TestReportKeepsTheUnsuppressedScoreVisible(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html", `<div class="p-4 p-2"></div>`)

	config, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	first, err := RunWithConfig(root, config, nil)
	if err != nil {
		t.Fatalf("RunWithConfig: %v", err)
	}
	// One conflict over two utilities: D = 3 x 1/2, so 100 x 0.2/1.7 = 12.
	if first.Score != 12 || first.ScoreExcludingBaseline != 12 {
		t.Fatalf("unsuppressed run scored %d/%d, want 12/12", first.Score, first.ScoreExcludingBaseline)
	}

	if err := WriteBaseline(filepath.Join(root, BaselineFileName), NewBaseline(first)); err != nil {
		t.Fatalf("WriteBaseline: %v", err)
	}
	second, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if second.Score != MaximumScore {
		t.Errorf("suppressed debt should not move the score, got %d", second.Score)
	}
	if second.ScoreExcludingBaseline != 12 {
		t.Errorf("scoreExcludingBaseline = %d, want 12: a baseline must not hide debt",
			second.ScoreExcludingBaseline)
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, second)
	if !strings.Contains(buffer.String(), "(12 ignoring baseline)") {
		t.Errorf("the human report should show both numbers:\n%s", buffer.String())
	}
}

func TestHumanReportSummarizesUnscoredFindings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ConfigFileName, "[rules]\nvariant-density = \"error\"\n")
	writeFile(t, root, "page.html", `<div class="sm:p-2 md:p-4 lg:m-6 xl:m-8 2xl:mt-10"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	output := buffer.String()

	for _, want := range []string{
		"Consistency",
		"Accessibility",
		"not measured",
		"1 unscored finding(s) hidden",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("human output is missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "[variant-density]") {
		t.Errorf("unscored findings must not be listed in human output:\n%s", output)
	}
	if report.Score != MaximumScore {
		t.Errorf("a medium-confidence finding must not move the score, got %d", report.Score)
	}
}

// A score computed while the theme applied to almost nothing looks exactly as
// confident as a fully themed run. The human report must say so loudly.
func TestHumanReportWarnsWhenMostListsAreUnscoped(t *testing.T) {
	report := Report{
		Packages: []TailwindPackageReport{{Directory: "src/styles", Version: "4"}},
		Coverage: CoverageReport{
			ResolvedClassLists:   100,
			UnresolvedClassLists: 10,
			UnscopedClassLists:   80,
			ResolutionPercent:    90,
		},
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	output := buffer.String()

	if !strings.Contains(output, "Warning: 80 of 100 resolved class list(s) are outside every detected Tailwind package") {
		t.Errorf("human output is missing the unscoped-coverage warning:\n%s", output)
	}

	report.Coverage.UnscopedClassLists = 10
	buffer.Reset()
	WriteHuman(&buffer, report)
	if strings.Contains(buffer.String(), "Warning:") {
		t.Errorf("a mostly scoped project must not warn:\n%s", buffer.String())
	}
}

// The header stats were telemetry — dense counts with no judgment attached.
// Doctor-style check lines say what each number means for the run.
func TestHumanReportRendersCheckLines(t *testing.T) {
	report := Report{
		Score:    60,
		Scanned:  Scanned{Files: 10, ClassLists: 90, Utilities: 300, Tokens: 9},
		Packages: []TailwindPackageReport{{Directory: ".", Version: "4"}},
		Coverage: CoverageReport{ResolvedClassLists: 90, UnresolvedClassLists: 10,
			UnscopedClassLists: 0, ResolutionPercent: 90},
		Accessibility:   AccessibilityReport{ResolvedColorPairs: 5, UnknownColorPairs: 7},
		ConfiguredRules: []ConfiguredRule{{ID: "color-contrast", Severity: SeverityOff}},
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	output := buffer.String()

	for _, want := range []string{
		"✓ 1 Tailwind package(s) detected",
		"✓ Theme inventoried: 9 project token(s)",
		"✓ Scanned 10 file(s): 90 class list(s), 300 utilities",
		"✗ 10 of 100 class list(s) (10%) are dynamic expressions and were not analyzed",
		"✓ Every resolved class list matched a Tailwind package",
		"• 5 color pair(s) measured, 7 unknown; enable the color-contrast rule to score accessibility",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("human output is missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Accessibility coverage gaps") {
		t.Errorf("reason codes belong in --json, not the human header:\n%s", output)
	}

	report.Packages = nil
	buffer.Reset()
	WriteHuman(&buffer, report)
	if !strings.Contains(buffer.String(), "✗ No Tailwind package detected; theme-dependent rules are disabled") {
		t.Errorf("a package-less run must say so:\n%s", buffer.String())
	}
}

// Repeating the full path on every finding buries the findings; grouping under
// one file header is how every mainstream linter keeps long lists readable.
func TestHumanReportGroupsFindingsByFile(t *testing.T) {
	report := Report{Findings: []Finding{
		{Rule: "no-arbitrary-value", File: "src/a.tsx", Line: 3, Column: 7, Message: "first", Scored: true},
		{Rule: "no-arbitrary-value", File: "src/a.tsx", Line: 9, Column: 2, Message: "second", Scored: true},
		{Rule: "no-conflicting-utilities", File: "src/b.tsx", Line: 1, Column: 1, Message: "third", Scored: true},
	}}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)

	block := "src/a.tsx\n" +
		"  3:7 [no-arbitrary-value] first\n" +
		"  9:2 [no-arbitrary-value] second\n" +
		"src/b.tsx\n" +
		"  1:1 [no-conflicting-utilities] third\n"
	if !strings.Contains(buffer.String(), block) {
		t.Errorf("human output is missing the grouped findings:\n%s", buffer.String())
	}
}

// The human report is for reading, not archiving: past a point, more lines are
// less information. The machine formats stay exhaustive.
func TestHumanReportTruncatesLongFindingLists(t *testing.T) {
	findings := []Finding{}
	for index := range 130 {
		findings = append(findings, Finding{Rule: "no-arbitrary-value",
			File: fmt.Sprintf("src/f%03d.tsx", index), Line: 1, Column: 1, Message: "m", Scored: true})
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, Report{Findings: findings})
	output := buffer.String()

	if !strings.Contains(output, "src/f099.tsx") {
		t.Errorf("finding 100 should still be listed:\n%s", output)
	}
	if strings.Contains(output, "src/f100.tsx") {
		t.Errorf("finding 101 should be truncated:\n%s", output)
	}
	if !strings.Contains(output, "… and 30 more finding(s) in 30 file(s); use --json or --sarif for the full list.") {
		t.Errorf("human output is missing the truncation notice:\n%s", output)
	}
}

// text-[11px] appearing 165 times is one missing token, not 165 separate
// problems. Aggregating repeated arbitrary values turns a wall of findings
// into a handful of design decisions.
func TestHumanReportAggregatesRepeatedArbitraryValues(t *testing.T) {
	findings := []Finding{
		{Rule: "no-conflicting-utilities", Class: "p-2 p-4", File: "src/x.tsx", Message: "conflict", Scored: true},
	}
	for range 3 {
		findings = append(findings, Finding{Rule: "no-arbitrary-value", Class: "text-[11px]",
			File: "src/a.tsx", Message: "avoid", Scored: true})
	}
	for range 2 {
		findings = append(findings, Finding{Rule: "no-arbitrary-value", Class: "w-[13px]",
			File: "src/b.tsx", Message: "avoid", Scored: true, replacement: "w-3.5"})
	}
	findings = append(findings, Finding{Rule: "no-arbitrary-value", Class: "h-[9px]",
		File: "src/c.tsx", Message: "avoid", Scored: true})

	var buffer bytes.Buffer
	WriteHuman(&buffer, Report{Findings: findings})
	output := buffer.String()

	block := "Repeated arbitrary values:\n" +
		"- 3 × text-[11px]\n" +
		"- 2 × w-[13px] → w-3.5\n"
	if !strings.Contains(output, block) {
		t.Errorf("human output is missing the repeated-values block:\n%s", output)
	}
	if strings.Contains(output, "- 1 × h-[9px]") {
		t.Errorf("a value seen once must not be aggregated:\n%s", output)
	}
}

// A doctor that only diagnoses buries its own cure: --fix exists, so the
// report must say how many findings it would resolve.
func TestHumanReportCountsAutoFixableFindings(t *testing.T) {
	report := Report{
		Findings: []Finding{
			{Rule: "no-arbitrary-value", File: "src/a.tsx", Message: "one", Scored: true, fixable: true},
			{Rule: "no-arbitrary-value", File: "src/b.tsx", Message: "two", Scored: true, fixable: true},
			{Rule: "no-arbitrary-value", File: "src/c.tsx", Message: "three", Scored: true},
		},
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	if !strings.Contains(buffer.String(), "2 finding(s) can be fixed automatically; run tw-doctor --fix.") {
		t.Errorf("human output is missing the fixable count:\n%s", buffer.String())
	}

	report.Findings = report.Findings[2:]
	buffer.Reset()
	WriteHuman(&buffer, report)
	if strings.Contains(buffer.String(), "--fix") {
		t.Errorf("a report with nothing fixable must not advertise --fix:\n%s", buffer.String())
	}
}

// On a project with thousands of findings the list alone answers "where", not
// "what kind of debt dominates"; the per-rule counts answer that in one glance.
func TestHumanReportSummarizesFindingsByRule(t *testing.T) {
	report := Report{
		Findings: []Finding{
			{Rule: "no-arbitrary-value", File: "src/a.tsx", Message: "one", Scored: true},
			{Rule: "no-arbitrary-value", File: "src/b.tsx", Message: "two", Scored: true},
			{Rule: "no-conflicting-utilities", File: "src/c.tsx", Message: "three", Scored: true},
			{Rule: "no-conflicting-utilities", File: "src/d.tsx", Message: "four", Confidence: ConfidenceMedium},
			{Rule: "variant-density", File: "src/e.tsx", Message: "five", Confidence: ConfidenceMedium},
		},
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	output := buffer.String()

	summary := "Findings by rule:\n" +
		"- no-arbitrary-value: 2\n" +
		"- no-conflicting-utilities: 2 (1 scored)\n" +
		"- variant-density: 1 (0 scored)\n"
	if !strings.Contains(output, summary) {
		t.Errorf("human output is missing the per-rule summary:\n%s", output)
	}
}

func TestHumanReportListsScoredFindingsOnly(t *testing.T) {
	report := Report{
		Scanned: Scanned{ClassLists: 2, Utilities: 4},
		Findings: []Finding{
			{Rule: "no-arbitrary-value", Category: CategoryConsistency, Confidence: ConfidenceHigh,
				File: "src/a.tsx", Line: 1, Column: 2, Message: "Avoid arbitrary values.", Scored: true},
			{Rule: "no-conflicting-utilities", Category: CategoryCorrectness, Confidence: ConfidenceMedium,
				File: "src/b.tsx", Line: 3, Column: 4, Message: "text-a conflicts with text-b in the same variant."},
		},
	}

	var buffer bytes.Buffer
	WriteHuman(&buffer, report)
	output := buffer.String()

	if !strings.Contains(output, "[no-arbitrary-value]") {
		t.Errorf("scored finding missing from human output:\n%s", output)
	}
	if strings.Contains(output, "[no-conflicting-utilities]") {
		t.Errorf("unscored finding listed in human output:\n%s", output)
	}
	for _, want := range []string{
		"2 finding(s), 1 scored:",
		"1 unscored finding(s) hidden (medium or low confidence); use --json or --sarif to review them.",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("human output is missing %q:\n%s", want, output)
		}
	}
}
