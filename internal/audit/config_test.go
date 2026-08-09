package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLoadConfigReadsEverySetting(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ConfigFileName, `
# Settings for this project.
[rules]
no-arbitrary-value = "off"
responsive-bloat = "warn"

[paths]
ignore = [
  "generated/**",   # machine-written
  "vendor/**",
]
respect-gitignore = false

[arbitrary-values]
allow = ["text-[#123456]"]

[tailwind]
prefix = "tw-"
separator = "_"

[baseline]
path = "debt.json"
`)

	config, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.severityFor("no-arbitrary-value") != SeverityOff {
		t.Errorf("no-arbitrary-value severity = %q", config.severityFor("no-arbitrary-value"))
	}
	if config.severityFor("responsive-bloat") != SeverityWarn {
		t.Errorf("responsive-bloat severity = %q", config.severityFor("responsive-bloat"))
	}
	if config.severityFor("no-conflicting-utilities") != SeverityError {
		t.Errorf("an unconfigured rule should default to error")
	}
	if len(config.IgnorePaths) != 2 || config.IgnorePaths[0] != "generated/**" {
		t.Errorf("ignore paths = %#v", config.IgnorePaths)
	}
	if config.RespectGitignore {
		t.Errorf("respect-gitignore should be false")
	}
	if !config.AllowedArbitrary["text-[#123456]"] {
		t.Errorf("allowed arbitrary values = %#v", config.AllowedArbitrary)
	}
	if config.Syntax.Prefix != "tw-" || config.Syntax.Separator != "_" {
		t.Errorf("syntax = %#v", config.Syntax)
	}
	if !config.prefixConfigured || !config.separatorConfigured {
		t.Errorf("explicit syntax settings were not recorded: %#v", config)
	}
	if config.BaselinePath != "debt.json" {
		t.Errorf("baseline path = %q", config.BaselinePath)
	}
}

// A configuration file that cannot be understood must stop the run. Analysing
// with settings that were silently half-applied produces a number nobody can
// trust and nobody can debug.
func TestLoadConfigRejectsWhatItCannotUnderstand(t *testing.T) {
	cases := map[string]string{
		"unknown severity":  "[rules]\nno-arbitrary-value = \"loud\"\n",
		"wrong type":        "[paths]\nignore = \"generated\"\n",
		"unsupported value": "[tailwind]\nprefix = 12\n",
		"missing value":     "[tailwind]\nprefix\n",
		"empty separator":   "[tailwind]\nseparator = \"\"\n",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, ConfigFileName, content)
			if _, err := LoadConfig(root); err == nil {
				t.Fatalf("expected an error")
			}
		})
	}
}

func TestLoadConfigTreatsAMissingFileAsDefaults(t *testing.T) {
	config, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !config.RespectGitignore || config.Syntax.Separator != ":" {
		t.Fatalf("defaults = %#v", config)
	}
}

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern string
		target  string
		want    bool
	}{
		{"dist/**", "dist/app.html", true},
		{"dist/**", "dist/nested/deep/app.html", true},
		{"dist/**", "dist", true},
		{"dist/**", "src/app.html", false},
		{"**/*.min.html", "public/build/app.min.html", true},
		{"*.html", "app.html", true},
		{"*.html", "nested/app.html", false},
	}

	for _, test := range cases {
		if got := matchPath(test.pattern, test.target); got != test.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", test.pattern, test.target, got, test.want)
		}
	}
}

func TestRunHonoursIgnoredPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ConfigFileName, "[paths]\nignore = [\"generated/**\"]\n")
	writeFile(t, root, "generated/page.html", `<div class="p-4 p-2"></div>`)
	writeFile(t, root, "src/page.html", `<div class="m-4 m-2"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Files != 1 {
		t.Fatalf("scanned %d files, want 1", report.Scanned.Files)
	}
	if len(report.Findings) != 1 || report.Findings[0].File != "src/page.html" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

// Git applies a .gitignore to its own directory and below, and a nested file can
// re-include what an outer one excluded.
func TestRunHonoursGitignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "out/\n*.generated.html\n")
	writeFile(t, root, "out/page.html", `<div class="p-4 p-2"></div>`)
	writeFile(t, root, "src/thing.generated.html", `<div class="p-4 p-2"></div>`)
	writeFile(t, root, "src/page.html", `<div class="m-4 m-2"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Files != 1 {
		t.Fatalf("scanned %d files, want 1", report.Scanned.Files)
	}

	writeFile(t, root, ConfigFileName, "[paths]\nrespect-gitignore = false\n")
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Files != 3 {
		t.Fatalf("scanned %d files with gitignore off, want 3", report.Scanned.Files)
	}
}

func TestSeverityDecidesWhatMovesTheScore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html", `<div class="p-4 p-2"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// One conflict over two utilities: D = 3 x 1/2, so 100 x 0.2/1.7 rounds to 12.
	if report.Score != 12 {
		t.Fatalf("score = %d, want 12", report.Score)
	}

	writeFile(t, root, ConfigFileName, "[rules]\nno-conflicting-utilities = \"warn\"\n")
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("a warning should still be reported, got %#v", report.Findings)
	}
	if report.Score != MaximumScore {
		t.Fatalf("a warning should not move the score, got %d", report.Score)
	}

	writeFile(t, root, ConfigFileName, "[rules]\nno-conflicting-utilities = \"off\"\n")
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("a disabled rule should report nothing, got %#v", report.Findings)
	}
}

func TestArbitraryValueAllowlist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html", `<div class="text-[#123456] shadow-[0_0_0]"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("findings = %#v", report.Findings)
	}

	writeFile(t, root, ConfigFileName, "[arbitrary-values]\nallow = [\"text-[#123456]\"]\n")
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Class != "shadow-[0_0_0]" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

// A baseline records debt by rule, file, and class so that reformatting a file
// does not resurrect every suppressed finding in it.
func TestBaselineSuppressesRecordedDebtAndSurvivesMovement(t *testing.T) {
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
	if err := WriteBaseline(filepath.Join(root, BaselineFileName), NewBaseline(first)); err != nil {
		t.Fatalf("WriteBaseline: %v", err)
	}

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 0 || report.Suppressed != 1 {
		t.Fatalf("findings = %#v, suppressed = %d", report.Findings, report.Suppressed)
	}
	if report.Score != MaximumScore {
		t.Fatalf("suppressed debt should not move the score, got %d", report.Score)
	}

	// The same class list, pushed down the file. Position is not part of the key.
	writeFile(t, root, "page.html", "\n\n\n<div class=\"p-4 p-2\"></div>")
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("moving code resurrected suppressed debt: %#v", report.Findings)
	}

	// New debt is not covered by the baseline.
	writeFile(t, root, "other.html", `<div class="m-4 m-2"></div>`)
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].File != "other.html" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestBaselineRejectsAnUnknownVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, BaselineFileName, `{"version": 99, "suppressed": []}`)
	if _, err := Run(root); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected a version error, got %v", err)
	}
}

// A rule that has not yet had its release of warning is off unless the project
// asks for it, and asking for it must still work.
func TestSeverityForRespectsDefaultOn(t *testing.T) {
	config := defaultConfig()
	if severity := config.severityFor("unused-token"); severity != SeverityOff {
		t.Errorf("unused-token defaults to %q, want off", severity)
	}
	original := ruleRegistry
	ruleRegistry = append(append([]RuleDefinition(nil), original...), RuleDefinition{
		ID: "test-only-new-rule", Category: CategoryConsistency, Exposure: ExposureUtility,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceHigh,
		Since: "0.2.0", DefaultOn: false,
	})
	t.Cleanup(func() { ruleRegistry = original })

	if severity := config.severityFor("test-only-new-rule"); severity != SeverityOff {
		t.Errorf("a rule shipping disabled defaults to %q, want off", severity)
	}

	config.Severities["test-only-new-rule"] = SeverityError
	if severity := config.severityFor("test-only-new-rule"); severity != SeverityError {
		t.Errorf("configuration should be able to enable it, got %q", severity)
	}
}
