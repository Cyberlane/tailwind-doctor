package audit

import "testing"

func TestSourceAnalyzerUsesUnsavedContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.tsx", `<div className="p-4" />`)
	analyzer, err := NewSourceAnalyzer(root)
	if err != nil {
		t.Fatalf("NewSourceAnalyzer: %v", err)
	}
	findings, err := analyzer.AnalyzeSource("page.tsx", `<div className="p-4 p-2 text-[#123456]" />`)
	if err != nil {
		t.Fatalf("AnalyzeSource: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %#v", findings)
	}
	findings, err = analyzer.AnalyzeSource("page.tsx", `<div className="p-4" />`)
	if err != nil || len(findings) != 0 {
		t.Fatalf("clean findings = %#v, err = %v", findings, err)
	}
}

func TestSourceAnalyzerRefusesPathsOutsideRoot(t *testing.T) {
	analyzer, err := NewSourceAnalyzer(t.TempDir())
	if err != nil {
		t.Fatalf("NewSourceAnalyzer: %v", err)
	}
	if _, err := analyzer.AnalyzeSource("../outside.tsx", `<div className="p-4" />`); err == nil {
		t.Fatal("expected an error")
	}
}
