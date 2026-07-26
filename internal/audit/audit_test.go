package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectFindsHighConfidenceProblems(t *testing.T) {
	findings := inspect("src/card.tsx", "p-4 p-2 text-[#123456] sm:p-2 md:p-4 lg:p-6 xl:p-8 2xl:p-10")

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %#v", len(findings), findings)
	}
}

func TestInspectKeepsVariantsSeparate(t *testing.T) {
	findings := inspect("src/card.tsx", "p-4 md:p-6")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
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
