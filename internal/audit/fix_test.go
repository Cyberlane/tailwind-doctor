package audit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFixReplacesOnlyExactTokenMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
	writeFile(t, root, "app.css", `@import "tailwindcss"; @theme { --color-brand: #abcdef; --spacing: 0.25rem; }`)
	writeFile(t, root, "page.html", `<div class="text-[#abcdef] mt-[1rem] text-[#fedcba]"></div>`)

	result, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if result != (FixResult{Files: 1, Replacements: 2}) {
		t.Fatalf("result = %+v", result)
	}
	content, err := os.ReadFile(filepath.Join(root, "page.html"))
	if err != nil {
		t.Fatalf("read fixed source: %v", err)
	}
	want := `<div class="text-brand mt-4 text-[#fedcba]"></div>`
	if string(content) != want {
		t.Fatalf("fixed source = %q, want %q", content, want)
	}

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run after fix: %v", err)
	}
	remainingArbitrary := 0
	for _, finding := range report.Findings {
		if finding.Rule == "no-arbitrary-value" && finding.Class == "text-[#fedcba]" {
			remainingArbitrary++
		}
	}
	if remainingArbitrary != 1 {
		t.Fatalf("findings after fix = %+v", report.Findings)
	}
}

func TestFixHandlesEveryVerbatimExtractionShape(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
	writeFile(t, root, "app.css", "@import \"tailwindcss\"; @theme { --color-brand: #abcdef; }\n.card { @apply text-[#abcdef]; }\n")
	writeFile(t, root, "page.html", `<div class="text-[#abcdef]"></div>`)
	writeFile(t, root, "card.tsx", `import clsx from "clsx"; import { cva } from "class-variance-authority"; const card = cva("text-[#abcdef]", { variants: { tone: { brand: "text-[#abcdef]" } } }); const value = clsx("text-[#abcdef]")`)
	writeFile(t, root, "nav.vue", `<div :class="['text-[#abcdef]']"></div>`)
	writeFile(t, root, "hero.astro", `<div class:list={['text-[#abcdef]']}></div>`)

	result, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if result.Replacements != 7 || result.Files != 5 {
		t.Fatalf("result = %+v, want 7 replacements in 5 files", result)
	}
	for _, name := range []string{"app.css", "page.html", "card.tsx", "nav.vue", "hero.astro"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(content), "[#abcdef]") {
			t.Errorf("%s still contains arbitrary value: %s", name, content)
		}
	}
}

func TestFixLeavesUntrustedAndNonVerbatimValuesUntouched(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
	writeFile(t, root, "app.css", `@import "tailwindcss"; @theme { --color-brand: #abcdef; }`)
	writeFile(t, root, ConfigFileName, "[arbitrary-values]\nallow = [\"bg-[#abcdef]\"]\n")
	source := `<div class="text-[#abcdef] {runtime} bg-[#abcdef]"></div>`
	writeFile(t, root, "page.svelte", source)

	result, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if result != (FixResult{}) {
		t.Fatalf("result = %+v, want no changes", result)
	}
	content, _ := os.ReadFile(filepath.Join(root, "page.svelte"))
	if string(content) != source {
		t.Fatalf("source changed to %q", content)
	}
}

func TestFixHonorsBaselineAndRuleSeverity(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		config string
		base   string
	}{
		{name: "rule off", config: "[rules]\nno-arbitrary-value = \"off\"\n"},
		{name: "baselined", base: `{"version":1,"suppressed":[{"rule":"no-arbitrary-value","file":"page.html","class":"text-[#abcdef]"}]}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
			writeFile(t, root, "app.css", `@import "tailwindcss"; @theme { --color-brand: #abcdef; }`)
			writeFile(t, root, "page.html", `<div class="text-[#abcdef]"></div>`)
			if testCase.config != "" {
				writeFile(t, root, ConfigFileName, testCase.config)
			}
			if testCase.base != "" {
				writeFile(t, root, BaselineFileName, testCase.base)
			}

			result, err := Fix(root)
			if err != nil {
				t.Fatalf("Fix: %v", err)
			}
			if result != (FixResult{}) {
				t.Fatalf("result = %+v, want no changes", result)
			}
		})
	}
}

func TestFixPreservesSourcePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
	writeFile(t, root, "app.css", `@import "tailwindcss"; @theme { --color-brand: #abcdef; }`)
	path := filepath.Join(root, "page.html")
	writeFile(t, root, "page.html", `<div class="text-[#abcdef]"></div>`)
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("chmod fixture: %v", err)
	}

	if _, err := Fix(root); err != nil {
		t.Fatalf("Fix: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixed source: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions = %o, want 640", info.Mode().Perm())
	}
}

func TestRunSkipsSymbolicLinkSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated permissions on Windows")
	}
	root := t.TempDir()
	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "outside.html")
	if err := os.WriteFile(target, []byte(`<div class="p-4 p-2"></div>`), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked.html")); err != nil {
		t.Fatalf("symlink fixture: %v", err)
	}

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Files != 0 || len(report.Findings) != 0 {
		t.Fatalf("symlink was analyzed: scanned=%+v findings=%+v", report.Scanned, report.Findings)
	}
}

func TestRunRefusesTailwindEvidenceOutsideTheRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated permissions on Windows")
	}
	root := t.TempDir()
	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "app.css")
	if err := os.WriteFile(target, []byte(`@import "tailwindcss"; @theme { --color-private: #abcdef; }`), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, "app.css")); err != nil {
		t.Fatalf("symlink fixture: %v", err)
	}

	if _, err := Run(root); err == nil {
		t.Fatal("Run followed Tailwind evidence outside the analysis root")
	}
}

func TestApplyFixesRejectsStaleSourceWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html", `<div class="text-[#abcdef]"></div>`)
	finding := Finding{
		File: "page.html", Class: "text-[#000000]", Line: 1, Column: 13,
		replacement: "text-black", fixable: true,
	}

	if _, err := applyFixes(root, []Finding{finding}); err == nil || !strings.Contains(err.Error(), "stale fix") {
		t.Fatalf("error = %v, want stale fix refusal", err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "page.html"))
	if string(content) != `<div class="text-[#abcdef]"></div>` {
		t.Fatalf("source changed to %q", content)
	}
}
