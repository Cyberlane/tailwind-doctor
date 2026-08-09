package tailwind

import "testing"

func TestSortDiagnosticsIsDeterministic(t *testing.T) {
	diagnostics := []Diagnostic{
		{Kind: DiagnosticImportCycle, File: "src/b.css", Line: 1, Column: 1},
		{Kind: DiagnosticUnreadableConfig, File: "src/a.css", Line: 9, Column: 2},
		{Kind: DiagnosticUnreadableConfig, File: "src/a.css", Line: 2, Column: 7},
		{Kind: DiagnosticImportDepth, File: "src/a.css", Line: 2, Column: 7},
	}

	SortDiagnostics(diagnostics)
	want := []struct {
		file string
		line int
		kind DiagnosticKind
	}{
		{"src/a.css", 2, DiagnosticImportDepth},
		{"src/a.css", 2, DiagnosticUnreadableConfig},
		{"src/a.css", 9, DiagnosticUnreadableConfig},
		{"src/b.css", 1, DiagnosticImportCycle},
	}
	for index, expected := range want {
		got := diagnostics[index]
		if got.File != expected.file || got.Line != expected.line || got.Kind != expected.kind {
			t.Errorf("diagnostic %d = %s %s:%d, want %s %s:%d", index, got.Kind, got.File, got.Line, expected.kind, expected.file, expected.line)
		}
	}
}
