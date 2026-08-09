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
			Utilities int `json:"utilities"`
		} `json:"scanned"`
		ConfiguredRules []struct {
			ID string `json:"id"`
		} `json:"configuredRules"`
		Diagnostics []ReportDiagnostic `json:"diagnostics"`
		Findings    []struct {
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
			t.Errorf("accessibility has no rule yet and must report null, got %d", *category.Score)
		}
	}
	if len(decoded.ConfiguredRules) != len(ruleRegistry) {
		t.Errorf("configuredRules lists %d rules, want %d", len(decoded.ConfiguredRules), len(ruleRegistry))
	}
	if decoded.Diagnostics == nil {
		t.Fatal("diagnostics must be an array, not null")
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

func TestHumanReportShowsSubScoresAndUnscoredFindings(t *testing.T) {
	root := t.TempDir()
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
		"responsive-bloat",
		"not scored",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("human output is missing %q:\n%s", want, output)
		}
	}
	if report.Score != MaximumScore {
		t.Errorf("a medium-confidence finding must not move the score, got %d", report.Score)
	}
}
