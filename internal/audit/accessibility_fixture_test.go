package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessibilityFixtureSuiteHasNoFalseFailures(t *testing.T) {
	report, err := Run(filepath.Join("testdata", "projects", "accessibility"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	const expectedFailures = 6
	reportedFailures := 0
	falseFailures := 0
	for _, finding := range report.Findings {
		if finding.Rule != "color-contrast" {
			continue
		}
		if finding.File == "known-fail.html" {
			reportedFailures++
		} else {
			falseFailures++
			t.Errorf("false color-contrast failure in %s: %s", finding.File, finding.Message)
		}
	}
	missedFailures := expectedFailures - reportedFailures
	if missedFailures < 0 {
		falseFailures += -missedFailures
		t.Errorf("reported %d known failures, fixture declares %d", reportedFailures, expectedFailures)
		missedFailures = 0
	}
	t.Logf("accessibility fixture: %d expected, %d reported, %d missed, %d false; %d resolved pairs, %d unknown pairs",
		expectedFailures, reportedFailures, missedFailures, falseFailures,
		report.Accessibility.ResolvedColorPairs, report.Accessibility.UnknownColorPairs)
	if falseFailures != 0 {
		t.Fatalf("accessibility fixture produced %d false failure(s)", falseFailures)
	}
	if reportedFailures == 0 {
		t.Fatal("fixture did not exercise any known contrast failure")
	}
	var accessibility CategoryScore
	for _, category := range report.Categories {
		if category.Name == CategoryAccessibility {
			accessibility = category
		}
	}
	if accessibility.Score == nil || *accessibility.Score >= MaximumScore || accessibility.ScoredFindings != reportedFailures {
		t.Errorf("accessibility category = %#v, want a measured score with %d findings", accessibility, reportedFailures)
	}
}

func TestVersion3ThemeColorsFeedContrastAnalysis(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0"}}`)
	writeFile(t, root, "tailwind.config.js", `module.exports = { theme: { extend: {
  colors: { ink: "#777777", panel: "#ffffff" },
  fontSize: { body: "16px" }
} } }`)
	writeFile(t, root, "page.html", `<p class="text-ink bg-panel text-body">Low contrast</p>`)
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("[rules]\ncolor-contrast = \"error\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.ColorPairs != 1 {
		t.Fatalf("color pairs = %d, want 1", report.Scanned.ColorPairs)
	}
	var contrast *Finding
	for index := range report.Findings {
		if report.Findings[index].Rule == "color-contrast" {
			contrast = &report.Findings[index]
		}
	}
	if contrast == nil || !strings.Contains(contrast.Message, "4.5:1") {
		t.Fatalf("contrast finding = %#v", contrast)
	}
}

func TestAccessibilityCoverageReasonsAreSorted(t *testing.T) {
	report, err := Run(filepath.Join("testdata", "projects", "accessibility"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.ColorPairs != report.Accessibility.ResolvedColorPairs {
		t.Errorf("scanned color pairs = %d, accessibility resolved pairs = %d",
			report.Scanned.ColorPairs, report.Accessibility.ResolvedColorPairs)
	}
	for index := 1; index < len(report.Accessibility.UnknownReasons); index++ {
		if report.Accessibility.UnknownReasons[index-1].Reason >= report.Accessibility.UnknownReasons[index].Reason {
			t.Fatalf("unknown reasons are not sorted: %#v", report.Accessibility.UnknownReasons)
		}
	}
	if report.Accessibility.UnknownColorPairs == 0 {
		t.Fatal("fixture did not exercise any unknown contrast context")
	}
}
