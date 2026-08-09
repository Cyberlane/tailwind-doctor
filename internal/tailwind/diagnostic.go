package tailwind

import "sort"

// A Diagnostic records something this tool could not read, not something wrong
// with the user's code. That distinction is why it is not a Finding: an
// unreadable construct must never reach a score.
type Diagnostic struct {
	Kind    DiagnosticKind
	File    string
	Line    int
	Column  int
	Message string
}

// DiagnosticKind is a stable machine-readable identifier.
type DiagnosticKind string

const (
	DiagnosticUnreadableConfig DiagnosticKind = "unreadable-config"
	DiagnosticUnknownVersion   DiagnosticKind = "unknown-version"
	DiagnosticExternalPreset   DiagnosticKind = "external-preset"
	DiagnosticImportCycle      DiagnosticKind = "import-cycle"
	DiagnosticImportDepth      DiagnosticKind = "import-depth"
)

// SortDiagnostics orders diagnostics by file, position, and kind.
func SortDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(first, second int) bool {
		left, right := diagnostics[first], diagnostics[second]
		switch {
		case left.File != right.File:
			return left.File < right.File
		case left.Line != right.Line:
			return left.Line < right.Line
		case left.Column != right.Column:
			return left.Column < right.Column
		}
		return left.Kind < right.Kind
	})
}
