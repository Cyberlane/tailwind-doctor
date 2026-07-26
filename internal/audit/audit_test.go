package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// classList builds the extraction result a rule would receive for a literal
// attribute starting at line 1, column 1.
func classList(value string) ClassList {
	return ClassList{Value: value, Line: 1, Column: 1, Shape: shapeAttributeLiteral, Resolved: true, Verbatim: true}
}

func TestInspectFindsHighConfidenceProblems(t *testing.T) {
	findings := inspect("src/card.tsx",
		classList("p-4 p-2 text-[#123456] sm:p-2 md:p-4 lg:p-6 xl:p-8 2xl:p-10"),
		defaultUtilitySyntax())

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %#v", len(findings), findings)
	}
}

func TestInspectKeepsVariantsSeparate(t *testing.T) {
	findings := inspect("src/card.tsx", classList("p-4 md:p-6"), defaultUtilitySyntax())
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}

// Every utility below sets padding under the same condition, written in a
// different dialect. A tool that splits on the last colon misreads most of them.
func TestInspectUnderstandsUtilitySyntax(t *testing.T) {
	cases := []struct {
		name    string
		classes string
		syntax  UtilitySyntax
		want    int
	}{
		{
			name:    "important marker before the utility, Tailwind v3",
			classes: "p-4 !p-2",
			syntax:  defaultUtilitySyntax(),
			want:    1,
		},
		{
			name:    "important marker after the utility, Tailwind v4",
			classes: "p-4 p-2!",
			syntax:  defaultUtilitySyntax(),
			want:    1,
		},
		{
			name:    "negative values still set the same property",
			classes: "mt-4 -mt-2",
			syntax:  defaultUtilitySyntax(),
			want:    1,
		},
		{
			name:    "stacked variants in any order select the same elements",
			classes: "hover:md:p-4 md:hover:p-2",
			syntax:  defaultUtilitySyntax(),
			want:    1,
		},
		{
			name:    "a configured prefix does not hide the property",
			classes: "tw-p-4 tw-p-2",
			syntax:  UtilitySyntax{Prefix: "tw-", Separator: ":"},
			want:    1,
		},
		{
			name:    "a configured separator splits variants",
			classes: "md_p-4 md_p-2",
			syntax:  UtilitySyntax{Separator: "_"},
			want:    1,
		},
		{
			name:    "an arbitrary value containing a colon is one utility",
			classes: "text-[color:red] text-sm",
			syntax:  defaultUtilitySyntax(),
			want:    2, // the arbitrary value, and its conflict with text-sm
		},
		{
			name:    "an arbitrary variant is not an arbitrary value",
			classes: "[&_svg]:size-4",
			syntax:  defaultUtilitySyntax(),
			want:    0,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			findings := inspect("src/card.tsx", classList(test.classes), test.syntax)
			if len(findings) != test.want {
				t.Fatalf("got %d findings, want %d: %#v", len(findings), test.want, findings)
			}
		})
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
	if strings.Contains(buffer.String(), "null") {
		t.Fatalf("report serialized a null field: %s", buffer.String())
	}
	if !strings.Contains(buffer.String(), `"findings": []`) {
		t.Fatalf("expected an empty findings array, got: %s", buffer.String())
	}
}
