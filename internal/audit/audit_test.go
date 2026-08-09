package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
)

// classList builds the extraction result a rule would receive for a literal
// attribute starting at line 1, column 1.
func classList(value string) ClassList {
	return ClassList{Value: value, Line: 1, Column: 1, Shape: shapeAttributeLiteral, Resolved: true, Verbatim: true}
}

func TestInspectFindsHighConfidenceProblems(t *testing.T) {
	findings := inspect("src/card.tsx",
		classList("p-4 p-2 text-[#123456] sm:p-2 md:p-4 lg:p-6 xl:p-8 2xl:p-10"),
		tailwind.DefaultUtilitySyntax())

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %#v", len(findings), findings)
	}
}

func TestInspectKeepsVariantsSeparate(t *testing.T) {
	findings := inspect("src/card.tsx", classList("p-4 md:p-6"), tailwind.DefaultUtilitySyntax())
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}

// A finding has to say where to look, or a user has to search for it by hand.
func TestFindingsCarryTheirPosition(t *testing.T) {
	root := t.TempDir()
	source := "<div class=\"p-4\">\n  <span class=\"m-2 text-[#abcdef]\"></span>\n</div>\n"
	if err := os.WriteFile(filepath.Join(root, "card.html"), []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %#v", report.Findings)
	}
	finding := report.Findings[0]
	if finding.Line != 2 || finding.Column != 20 {
		t.Fatalf("finding at %d:%d, want 2:20", finding.Line, finding.Column)
	}
}

// A clean project must serialize findings as [] rather than null, so consumers
// can iterate the field without a nil check.
func TestWriteJSONEmitsAnEmptyFindingsArray(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "card.tsx"), []byte(`<div className="p-4 md:p-6" />`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Score != MaximumScore {
		t.Fatalf("score = %d, want %d", report.Score, MaximumScore)
	}

	var buffer bytes.Buffer
	if err := WriteJSON(&buffer, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	// The only null in a report is an unmeasured category's score, which is
	// deliberate: 100 there would read as a clean bill of health.
	if strings.Contains(buffer.String(), `"findings": null`) {
		t.Fatalf("findings serialized as null: %s", buffer.String())
	}
	if !strings.Contains(buffer.String(), `"findings": []`) {
		t.Fatalf("expected an empty findings array, got: %s", buffer.String())
	}
}

// A finding carries its category and confidence because both decide whether it
// moves the score, and a user who cannot see them cannot argue with the number.
func TestInspectAttributesCategoryAndConfidence(t *testing.T) {
	findings := inspect("src/card.tsx", classList("p-4 p-2 text-[#123456]"), tailwind.DefaultUtilitySyntax())

	byRule := map[string]Finding{}
	for _, finding := range findings {
		byRule[finding.Rule] = finding
	}

	conflict, found := byRule["no-conflicting-utilities"]
	if !found {
		t.Fatalf("expected a conflict finding, got %#v", findings)
	}
	if conflict.Category != CategoryCorrectness || conflict.Confidence != ConfidenceHigh {
		t.Errorf("conflict on p-: category %q confidence %q, want correctness/high",
			conflict.Category, conflict.Confidence)
	}

	arbitrary, found := byRule["no-arbitrary-value"]
	if !found {
		t.Fatalf("expected an arbitrary-value finding, got %#v", findings)
	}
	if arbitrary.Category != CategoryConsistency || arbitrary.Confidence != ConfidenceHigh {
		t.Errorf("arbitrary value: category %q confidence %q, want consistency/high",
			arbitrary.Category, arbitrary.Confidence)
	}
}

func TestInspectSeparatesAmbiguousPropertyFamilies(t *testing.T) {
	cases := []struct {
		name    string
		classes string
		count   int
	}{
		{"padding conflict", "p-4 p-2", 1},
		{"margin conflict", "mt-4 mt-2", 1},
		{"border width and colour", "border-r border-gray-200", 0},
		{"background colour and size", "bg-red-500 bg-cover", 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			findings := inspect("src/card.tsx", classList(testCase.classes), tailwind.DefaultUtilitySyntax())
			if len(findings) != testCase.count {
				t.Fatalf("expected %d finding(s), got %#v", testCase.count, findings)
			}
		})
	}
}

// Responsive bloat is a maintainability heuristic, not a defect. It is reported
// but must not move a number people publish in a README.
func TestInspectReportsResponsiveBloatAtMediumConfidence(t *testing.T) {
	findings := inspect("src/card.tsx",
		classList("sm:p-2 md:p-4 lg:m-6 xl:m-8 2xl:mt-10"),
		tailwind.DefaultUtilitySyntax())

	if len(findings) != 1 || findings[0].Rule != "responsive-bloat" {
		t.Fatalf("expected one responsive-bloat finding, got %#v", findings)
	}
	if findings[0].Category != CategoryMaintainability {
		t.Errorf("category = %q, want maintainability", findings[0].Category)
	}
	if findings[0].Confidence != ConfidenceMedium {
		t.Errorf("confidence = %q, want medium", findings[0].Confidence)
	}
}

// Exposure is the score's denominator, so it counts resolved lists only. A
// className the tool cannot read must not enlarge the denominator: that would
// dilute measured debt in proportion to how much of a codebase is unanalysable.
func TestRunCountsResolvedExposureOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html", `<div class="p-4 text-sm"></div>`)
	writeFile(t, root, "src/card.tsx", "export const Card = ({x}) => <div className={x} />;\n")

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Files != 2 {
		t.Errorf("files = %d, want 2", report.Scanned.Files)
	}
	if report.Scanned.ClassLists != 1 {
		t.Errorf("class lists = %d, want 1: the dynamic className is unresolved", report.Scanned.ClassLists)
	}
	if report.Scanned.Utilities != 2 {
		t.Errorf("utilities = %d, want 2", report.Scanned.Utilities)
	}
}

func TestRunUsesTheNearestDetectedTailwindSyntax(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0"}}`)
	writeFile(t, root, "tailwind.config.js", `module.exports = { prefix: "tw-", separator: "_" }`)
	writeFile(t, root, "page.html", `<div class="hover_tw-p-4 hover_tw-p-2"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "no-conflicting-utilities" {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if len(report.themes) != 1 || report.themes[0].theme.Inventory == nil {
		t.Fatalf("resolved themes = %#v", report.themes)
	}
}

func TestExplicitSyntaxOverridesDetectedSyntax(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0"}}`)
	writeFile(t, root, "tailwind.config.js", `module.exports = { prefix: "detected-", separator: "_" }`)
	writeFile(t, root, ConfigFileName, "[tailwind]\nprefix = \"tw-\"\nseparator = \":\"\n")
	writeFile(t, root, "page.html", `<div class="hover:tw-p-4 hover:tw-p-2"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "no-conflicting-utilities" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}
