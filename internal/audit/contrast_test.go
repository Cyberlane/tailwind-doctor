package audit

import (
	"math"
	"strings"
	"testing"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

func contrastInventory() *tokens.Inventory {
	inventory := tokens.NewInventory()
	for _, token := range []tokens.Token{
		{Family: tokens.FamilyColor, Name: "black", Raw: "#000", Value: "#000000"},
		{Family: tokens.FamilyColor, Name: "white", Raw: "#fff", Value: "#ffffff"},
		{Family: tokens.FamilyColor, Name: "gray", Raw: "#777", Value: "#777777"},
		{Family: tokens.FamilyColor, Name: "light", Raw: "#888", Value: "#888888"},
		{Family: tokens.FamilyColor, Name: "current", Raw: "currentColor", Value: "currentcolor"},
		{Family: tokens.FamilyFontSize, Name: "body", Raw: "16px", Value: "16px"},
		{Family: tokens.FamilyFontSize, Name: "large", Raw: "24px", Value: "24px"},
		{Family: tokens.FamilyFontSize, Name: "bold-large", Raw: "19px", Value: "19px"},
		{Family: tokens.FamilyFontWeight, Name: "bold", Raw: "700", Value: "700"},
	} {
		inventory.Put(token)
	}
	return inventory
}

func TestInspectContrast(t *testing.T) {
	t.Parallel()
	inventory := contrastInventory()
	tests := []struct {
		name          string
		classes       string
		findings      int
		resolved      int
		unknownReason string
		message       string
	}{
		{name: "normal failure", classes: "text-gray bg-white text-body", findings: 1, resolved: 1, message: "requires at least 4.5:1"},
		{name: "universal failure", classes: "text-gray bg-light", findings: 1, resolved: 1, message: "requires at least 3.0:1"},
		{name: "pass", classes: "text-black bg-white", resolved: 1},
		{name: "large pass", classes: "text-gray bg-white text-large", resolved: 1},
		{name: "bold large pass", classes: "text-gray bg-white text-bold-large font-bold", resolved: 1},
		{name: "unknown inherited size", classes: "text-gray bg-white", unknownReason: unknownTextThreshold},
		{name: "missing background", classes: "text-gray", unknownReason: unknownMissingBackground},
		{name: "missing foreground", classes: "bg-white", unknownReason: unknownMissingForeground},
		{name: "unresolved foreground", classes: "text-current bg-white", unknownReason: unknownForegroundColor},
		{name: "multiple foregrounds", classes: "text-black text-gray bg-white", unknownReason: unknownMultipleForegrounds},
		{name: "foreground alpha", classes: "text-black/50 bg-white text-body", findings: 1, resolved: 1, message: "3.98:1"},
		{name: "background alpha", classes: "text-black bg-white/50", unknownReason: unknownUnsupportedCompositing},
		{name: "whole opacity", classes: "text-black bg-white opacity-50", unknownReason: unknownOpacityContext},
		{name: "dark pair", classes: "dark:text-gray dark:bg-light", findings: 1, resolved: 1},
		{name: "stacked variants", classes: "hover:dark:text-gray dark:hover:bg-light", findings: 1, resolved: 1},
		{name: "responsive pair", classes: "md:text-gray md:bg-light", findings: 1, resolved: 1},
		{name: "variants do not cross pair", classes: "text-black dark:bg-white", unknownReason: unknownMissingBackground},
		{name: "arbitrary named colors", classes: "text-[gray] bg-[white] text-body", findings: 1, resolved: 1},
		{name: "arbitrary rgb and hsl", classes: "text-[rgb(119_119_119)] bg-[hsl(0_0%_53.33%)]", findings: 1, resolved: 1},
		{name: "multiple backgrounds", classes: "text-black bg-white bg-light", unknownReason: unknownMultipleBackgrounds},
		{name: "legacy text opacity", classes: "text-black bg-white text-opacity-50", unknownReason: unknownOpacityContext},
		{name: "legacy background opacity", classes: "text-black bg-white bg-opacity-50", unknownReason: unknownOpacityContext},
		{name: "untrusted theme", classes: "text-black bg-white", unknownReason: unknownUntrustedTheme},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			trusted := testCase.name != "untrusted theme"
			inspection := inspectContrast("page.html", classList(testCase.classes), tailwind.DefaultUtilitySyntax(), inventory, trusted)
			if len(inspection.findings) != testCase.findings {
				t.Fatalf("findings = %d (%v), want %d", len(inspection.findings), inspection.findings, testCase.findings)
			}
			if inspection.resolvedPairs != testCase.resolved {
				t.Errorf("resolved pairs = %d, want %d", inspection.resolvedPairs, testCase.resolved)
			}
			if testCase.unknownReason != "" && inspection.unknownReasons[testCase.unknownReason] == 0 {
				t.Errorf("unknown reasons = %v, want %s", inspection.unknownReasons, testCase.unknownReason)
			}
			if testCase.message != "" && !strings.Contains(inspection.findings[0].Message, testCase.message) {
				t.Errorf("message = %q, want substring %q", inspection.findings[0].Message, testCase.message)
			}
		})
	}
}

func TestContextContrastThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		context    contrastContext
		threshold  float64
		resolvable bool
	}{
		{name: "normal", context: contrastContext{fontSizes: []string{"16px"}}, threshold: 4.5, resolvable: true},
		{name: "large", context: contrastContext{fontSizes: []string{"24px"}}, threshold: 3, resolvable: true},
		{name: "bold large", context: contrastContext{fontSizes: []string{"19px"}, fontWeights: []string{"700"}}, threshold: 3, resolvable: true},
		{name: "not bold large", context: contrastContext{fontSizes: []string{"19px"}, fontWeights: []string{"600"}}, threshold: 4.5, resolvable: true},
		{name: "relative size", context: contrastContext{fontSizes: []string{"1rem"}}},
		{name: "inherited", context: contrastContext{}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			threshold, resolved := contextContrastThreshold(testCase.context)
			if resolved != testCase.resolvable || math.Abs(threshold-testCase.threshold) > 1e-9 {
				t.Errorf("threshold = %v, resolved %v; want %v, %v", threshold, resolved, testCase.threshold, testCase.resolvable)
			}
		})
	}
}

func TestContrastSuggestionIsDeterministicAndPreservesSyntax(t *testing.T) {
	t.Parallel()
	inventory := contrastInventory()
	inspection := inspectContrast("page.html", classList("dark:!text-gray/50 dark:bg-white dark:text-large"), tailwind.DefaultUtilitySyntax(), inventory, true)
	if len(inspection.findings) != 1 {
		t.Fatalf("findings = %v, want one", inspection.findings)
	}
	if !strings.Contains(inspection.findings[0].Message, "dark:!text-black/50") {
		t.Errorf("message = %q, want syntax-preserving token suggestion", inspection.findings[0].Message)
	}
}
