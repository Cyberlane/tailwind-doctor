package audit

import (
	"strings"
	"testing"
)

func TestRunReportsTokenUsageSuggestionsAndApply(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
	writeFile(t, root, ConfigFileName, "[rules]\nunused-token = \"error\"\n")
	writeFile(t, root, "app.css", `@import "tailwindcss";
@theme {
  --color-brand: #abcdef;
  --color-orphan: #123456;
  --spacing: 0.25rem;
}
.button { @apply bg-brand p-4 text-[#abcdef] mt-[1rem]; }
`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Tokens != 3 || len(report.Tokens) != 1 {
		t.Fatalf("token exposure = %d, packages = %+v", report.Scanned.Tokens, report.Tokens)
	}
	if len(report.Tokens[0].Unused) != 1 || report.Tokens[0].Unused[0].Name != "orphan" {
		t.Fatalf("unused = %+v", report.Tokens[0].Unused)
	}

	var unused *Finding
	suggestions := map[string]bool{}
	for index := range report.Findings {
		switch report.Findings[index].Rule {
		case "unused-token":
			unused = &report.Findings[index]
		case "no-arbitrary-value":
			suggestions[report.Findings[index].Message] = true
		}
	}
	if unused == nil || unused.Class != "--color-orphan" || unused.Confidence != ConfidenceHigh {
		t.Fatalf("unused finding = %+v", unused)
	}
	var textSuggestion, spacingSuggestion bool
	for message := range suggestions {
		textSuggestion = textSuggestion || (strings.Contains(message, "text-brand") && strings.Contains(message, "--color-brand"))
		spacingSuggestion = spacingSuggestion || (strings.Contains(message, "mt-4") && strings.Contains(message, "--spacing"))
	}
	if !textSuggestion || !spacingSuggestion {
		t.Fatalf("suggestions = %+v", suggestions)
	}
}

func TestVersion3TokenUsageAndSuggestion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0"}}`)
	writeFile(t, root, ConfigFileName, "[rules]\nunused-token = \"error\"\n")
	writeFile(t, root, "tailwind.config.js", `module.exports = { theme: { extend: { colors: {
  brand: "#abcdef", orphan: "#123456"
} } } }`)
	writeFile(t, root, "page.html", `<div class="bg-brand text-[#abcdef]"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Tokens != 2 || len(report.Tokens) != 1 ||
		len(report.Tokens[0].Unused) != 1 || report.Tokens[0].Unused[0].Name != "orphan" {
		t.Fatalf("token report = %+v", report.Tokens)
	}
	var suggested bool
	for _, finding := range report.Findings {
		suggested = suggested || (finding.Rule == "no-arbitrary-value" && strings.Contains(finding.Message, "text-brand"))
	}
	if !suggested {
		t.Fatalf("findings = %+v", report.Findings)
	}
}

func TestUnusedTokenConfidenceFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^3.4.0","@acme/tailwind":"1.0.0"}}`)
	writeFile(t, root, ConfigFileName, "[rules]\nunused-token = \"error\"\n")
	writeFile(t, root, "tailwind.config.js", `module.exports = {
  theme: { extend: { colors: { orphan: "#123456" } } },
  plugins: [require("@acme/tailwind")]
}`)
	writeFile(t, root, "card.tsx", `export const Card = ({ classes }) => <div className={classes} />`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Tokens) != 1 || report.Tokens[0].Confidence != ConfidenceMedium ||
		len(report.Tokens[0].ConfidenceReasons) != 2 {
		t.Fatalf("token coverage = %+v", report.Tokens)
	}
	if report.Scanned.Tokens != 1 || report.Scanned.HighConfidenceTokens != 0 ||
		report.Scanned.MediumConfidenceTokens != 1 {
		t.Fatalf("token confidence exposure = %+v", report.Scanned)
	}
	for _, finding := range report.Findings {
		if finding.Rule == "unused-token" && (finding.Confidence != ConfidenceMedium || finding.Scored) {
			t.Fatalf("unused-token = %+v", finding)
		}
	}

	writeFile(t, root, ConfigFileName, "[rules]\nunused-token = \"error\"\n[score]\nmin-confidence = \"medium\"\n")
	report, err = Run(root)
	if err != nil {
		t.Fatalf("Run with medium confidence: %v", err)
	}
	var scored bool
	for _, finding := range report.Findings {
		scored = scored || (finding.Rule == "unused-token" && finding.Scored)
	}
	if !scored {
		t.Fatalf("medium-confidence token finding was not scored: %+v", report.Findings)
	}
	for _, category := range report.Categories {
		if category.Name != CategoryConsistency {
			continue
		}
		if len(category.Exposures) != 2 || category.Exposures[1] != (ExposureCount{Unit: ExposureToken, Count: 1}) {
			t.Fatalf("consistency exposures = %+v", category.Exposures)
		}
	}
}

func TestSharedDeclarationIsUsedAcrossPackages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "shared.css", `:root { --color-shared: #123456; }`)
	for _, packageName := range []string{"a", "b"} {
		writeFile(t, root, "packages/"+packageName+"/package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
		writeFile(t, root, "packages/"+packageName+"/app.css", `@import "tailwindcss"; @import "../../shared.css";`)
	}
	writeFile(t, root, "packages/a/page.html", `<div class="bg-shared"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Scanned.Tokens != 1 || len(report.Tokens) != 2 {
		t.Fatalf("token exposure = %d, packages = %+v", report.Scanned.Tokens, report.Tokens)
	}
	for _, packageReport := range report.Tokens {
		if len(packageReport.Unused) != 0 {
			t.Errorf("%s unused = %+v", packageReport.Package, packageReport.Unused)
		}
	}
}

func TestTokenTaxonomySeparatesTextColourAndSize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"tailwindcss":"^4.1.0"}}`)
	writeFile(t, root, "app.css", `@import "tailwindcss"; @theme { --color-brand: #123456; --text-display: 3rem; }`)
	writeFile(t, root, "page.html", `<div class="text-brand text-display"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, finding := range report.Findings {
		if finding.Rule == "no-conflicting-utilities" {
			t.Fatalf("different CSS properties reported as conflicting: %+v", finding)
		}
	}
}
