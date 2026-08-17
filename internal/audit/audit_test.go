package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
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

func TestInspectKeepsOrderSensitiveVariantStacksSeparate(t *testing.T) {
	findings := inspect("src/list.tsx", classList("*:first:pt-0 first:*:pt-4"), tailwind.DefaultUtilitySyntax())
	if len(findings) != 0 {
		t.Fatalf("expected order-sensitive variants to stay separate, got %#v", findings)
	}
}

func TestInspectIgnoresUnprefixedApplicationClasses(t *testing.T) {
	syntax := tailwind.UtilitySyntax{Prefix: "tw-", Separator: ":"}
	findings := inspect("src/card.tsx", classList("p-4 p-2 text-[#123456] tw-p-4 tw-p-2"), syntax)
	if len(findings) != 1 || findings[0].Rule != "no-conflicting-utilities" ||
		findings[0].Message != "tw-p-4 conflicts with tw-p-2 in the same variant." {
		t.Fatalf("findings = %#v", findings)
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
	if finding.EndLine != 2 || finding.EndColumn <= finding.Column {
		t.Fatalf("finding ends at %d:%d, want a non-empty range on line 2", finding.EndLine, finding.EndColumn)
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
		{"font size and colour", "text-4xl text-gray-600", 0},
		{"colour and font size", "text-white text-xs", 0},
		{"table border model and colour", "border-collapse border-transparent", 0},
		{"two font sizes", "text-xs text-4xl", 1},
		{"two text colours", "text-gray-600 text-white", 1},
		{"side width and side colour", "border-l border-l-transparent", 0},
		{"different border sides", "border-y border-r", 0},
		{"width for all sides and one side", "border border-r", 0},
		{"same border side twice", "border-t-2 border-t-4", 1},
		{"colour keyword and background clip", "bg-transparent bg-clip-padding", 0},
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
// When neither utility could be classified, the conflict is a guess from a
// shared lexical prefix; a resolved theme does not make that guess certain.
func TestConflictOnUnclassifiedPrefixStaysMedium(t *testing.T) {
	findings, _ := inspectWithInventory("src/card.tsx", classList("text-brandy text-blurple"),
		tailwind.DefaultUtilitySyntax(), tokens.NewInventory(), false)
	if len(findings) != 1 || findings[0].Rule != "no-conflicting-utilities" {
		t.Fatalf("expected one conflict finding, got %#v", findings)
	}
	if findings[0].Confidence != ConfidenceMedium {
		t.Fatalf("confidence = %q, want medium", findings[0].Confidence)
	}
}

// The human report shows one line per finding; without the offending class in
// the message, "avoid arbitrary values" forces the reader to open the file and
// count columns to learn which value the finding is about.
func TestArbitraryValueMessageNamesTheClass(t *testing.T) {
	findings := inspect("src/card.tsx", classList("w-[13px]"), tailwind.DefaultUtilitySyntax())
	if len(findings) != 1 || findings[0].Rule != "no-arbitrary-value" {
		t.Fatalf("expected one arbitrary-value finding, got %#v", findings)
	}
	if !strings.Contains(findings[0].Message, "w-[13px]") {
		t.Fatalf("message %q does not name the class", findings[0].Message)
	}
}

func TestInspectReportsVariantDensityAtMediumConfidence(t *testing.T) {
	findings := inspect("src/card.tsx",
		classList("sm:p-2 md:p-4 lg:m-6 xl:m-8 2xl:mt-10"),
		tailwind.DefaultUtilitySyntax())

	if len(findings) != 1 || findings[0].Rule != "variant-density" {
		t.Fatalf("expected one variant-density finding, got %#v", findings)
	}
	if findings[0].Category != CategoryMaintainability {
		t.Errorf("category = %q, want maintainability", findings[0].Category)
	}
	if findings[0].Confidence != ConfidenceMedium {
		t.Errorf("confidence = %q, want medium", findings[0].Confidence)
	}
}

func TestInspectReportsOverlappingUtilitiesSeparately(t *testing.T) {
	findings := inspect("src/card.tsx", classList("px-4 pl-2"), tailwind.DefaultUtilitySyntax())
	if len(findings) != 1 || findings[0].Rule != "no-overlapping-utilities" {
		t.Fatalf("expected one overlap finding, got %#v", findings)
	}
	if findings[0].Confidence != ConfidenceMedium {
		t.Fatalf("confidence = %q, want medium", findings[0].Confidence)
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
	if report.Coverage.ResolvedClassLists != 1 || report.Coverage.UnresolvedClassLists != 1 ||
		report.Coverage.ResolutionPercent != 50 {
		t.Errorf("coverage = %+v, want one resolved, one unresolved, and 50%%", report.Coverage)
	}
}

func TestRunDoesNotCountApplicationClassesAsUtilities(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html", `<div class="card prose-shell p-4"></div>`)
	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Utilities != 1 {
		t.Fatalf("utilities = %d, want only p-4", report.Scanned.Utilities)
	}
}

func TestRunScansJavaScriptTypeScriptAndMDX(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "component.js", `export const Component = () => <div className="p-4 p-2" />`)
	writeFile(t, root, "helper.ts", `import clsx from "clsx"; export const value = clsx("m-4 m-2")`)
	writeFile(t, root, "page.mdx", `<div className="pt-4 pt-2">content</div>`)
	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Files != 3 || len(report.Findings) != 3 {
		t.Fatalf("scanned = %+v, findings = %#v", report.Scanned, report.Findings)
	}
}

func TestRunAnalyzesManifestOnlyTailwindPackage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0"}}`)
	writeFile(t, root, "page.html", `<div class="p-4 p-2"></div>`)
	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Tokens) != 1 || len(report.Findings) != 1 {
		t.Fatalf("tokens = %#v, findings = %#v", report.Tokens, report.Findings)
	}
	if len(report.Packages) != 1 || len(report.Packages[0].Evidence) == 0 ||
		report.Packages[0].Evidence[0].Signal != "package-json" {
		t.Fatalf("package evidence = %#v", report.Packages)
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
