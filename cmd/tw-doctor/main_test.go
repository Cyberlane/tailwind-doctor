package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProject creates a directory holding a single source file, so a test can
// control the score the audit produces.
func writeProject(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "card.tsx"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

const (
	cleanSource = `<div className="p-4 md:p-6">ok</div>`
	debtSource  = `<div className="p-4 p-2 text-[#123456]">debt</div>`
)

func TestRunExitCodes(t *testing.T) {
	cleanProject := writeProject(t, cleanSource)
	debtProject := writeProject(t, debtSource)

	tests := []struct {
		name string
		args []string
		want int
	}{
		{
			name: "no threshold reports without gating",
			args: []string{debtProject},
			want: exitSuccess,
		},
		{
			name: "zero threshold accepts any score",
			args: []string{"--fail-under", "0", debtProject},
			want: exitSuccess,
		},
		{
			// Two findings over three utilities is dense debt, so the fixture
			// scores 11: D = 3 x 1/3 + 2 x 1/3, and 100 x 0.2/1.8667 rounds to 11.
			name: "score at the threshold passes",
			args: []string{"--fail-under", "11", debtProject},
			want: exitSuccess,
		},
		{
			name: "score below the threshold fails",
			args: []string{"--fail-under", "12", debtProject},
			want: exitBelowThreshold,
		},
		{
			name: "a perfect score satisfies the strictest threshold",
			args: []string{"--fail-under", "100", cleanProject},
			want: exitSuccess,
		},
		{
			name: "a negative threshold is an operational error",
			args: []string{"--fail-under", "-1", cleanProject},
			want: exitOperationalError,
		},
		{
			name: "a threshold above the maximum score is an operational error",
			args: []string{"--fail-under", "101", cleanProject},
			want: exitOperationalError,
		},
		{
			name: "an unreadable path is an operational error",
			args: []string{filepath.Join(cleanProject, "missing")},
			want: exitOperationalError,
		},
		{
			name: "an unknown flag is an operational error",
			args: []string{"--nope", cleanProject},
			want: exitOperationalError,
		},
		{
			name: "the threshold is ignored when printing the version",
			args: []string{"--version", "--fail-under", "100"},
			want: exitSuccess,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := run(test.args, &stdout, &stderr)
			if got != test.want {
				t.Fatalf("run(%q) = %d, want %d (stderr: %s)", test.args, got, test.want, stderr.String())
			}
		})
	}
}

func TestRunRejectsOutOfRangeThresholdWithAnExplanation(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{"--fail-under", "150", writeProject(t, cleanSource)}, &stdout, &stderr); code != exitOperationalError {
		t.Fatalf("exit code = %d, want %d", code, exitOperationalError)
	}
	if !strings.Contains(stderr.String(), "--fail-under") {
		t.Fatalf("stderr does not explain the rejected flag: %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no report on stdout, got %q", stdout.String())
	}
}

func TestRunWritesTheReportBeforeGating(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{"--fail-under", "100", writeProject(t, debtSource)}, &stdout, &stderr); code != exitBelowThreshold {
		t.Fatalf("exit code = %d, want %d", code, exitBelowThreshold)
	}
	if !strings.Contains(stdout.String(), "Tailwind Doctor:") {
		t.Fatalf("expected a report on stdout even when the gate fails, got %q", stdout.String())
	}
}

func TestRunRefusesTwoOutputFormats(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	if code := run([]string{"--json", "--sarif", root}, &stdout, &stderr); code != exitOperationalError {
		t.Errorf("exit code = %d, want %d", code, exitOperationalError)
	}
	if !strings.Contains(stderr.String(), "--json") || !strings.Contains(stderr.String(), "--sarif") {
		t.Errorf("stderr should name both flags: %q", stderr.String())
	}
}

func TestRunFixesBeforeReportingAndGating(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"tailwindcss":"^4.1.0"}}`), 0o644); err != nil {
		t.Fatalf("write package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.css"), []byte(`@import "tailwindcss"; @theme { --color-brand: #abcdef; }`), 0o644); err != nil {
		t.Fatalf("write theme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "card.tsx"), []byte(`<div className="text-[#abcdef]" />`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--fix", "--json", "--fail-under", "100", root}, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "fixed 1 arbitrary value(s) in 1 file(s)") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "text-[#abcdef]") || !strings.Contains(stdout.String(), `"score": 100`) {
		t.Fatalf("stdout is not the post-fix JSON report:\n%s", stdout.String())
	}
}

func TestRunRejectsAmbiguousWriteOperationsAndExtraPaths(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"--fix", "--write-baseline", root},
		{root, root},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitOperationalError {
			t.Fatalf("run(%q) = %d, want %d", args, code, exitOperationalError)
		}
		if stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("run(%q): stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

func TestRunWritesSARIF(t *testing.T) {
	root := writeProject(t, debtSource)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--sarif", root}, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version": "2.1.0"`) {
		t.Errorf("stdout is not a SARIF log:\n%s", stdout.String())
	}
}
