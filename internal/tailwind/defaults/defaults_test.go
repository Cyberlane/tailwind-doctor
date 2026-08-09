package defaults

import (
	"strings"
	"testing"

	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

func TestEveryDefaultTokenIsOriginDefault(t *testing.T) {
	for _, version := range []string{"3", "4"} {
		for _, token := range Theme(version).Tokens() {
			if token.Origin != tokens.OriginDefault {
				t.Errorf("v%s %s/%s has origin %q, want default", version, token.Family, token.Name, token.Origin)
			}
			if token.Decl != (tokens.Site{}) {
				t.Errorf("v%s %s/%s carries a declaration site; a default is not declared in the project", version, token.Family, token.Name)
			}
		}
	}
}

func TestDefaultTablesAreWellFormed(t *testing.T) {
	for _, version := range []string{"3", "4"} {
		for _, token := range Theme(version).Tokens() {
			if strings.TrimSpace(token.Raw) == "" {
				t.Errorf("v%s %s/%s has an empty raw value", version, token.Family, token.Name)
			}
			if token.Name == "" {
				t.Errorf("v%s %s has a token with an empty name", version, token.Family)
			}
			if token.Value == "" && !token.Unresolvable {
				t.Errorf("v%s %s/%s has no comparable value but is not marked unresolvable", version, token.Family, token.Name)
			}
			for _, stem := range []string{"color-", "spacing-", "text-", "font-", "font-weight-", "leading-", "tracking-", "radius-", "shadow-", "breakpoint-", "container-"} {
				if strings.HasPrefix(token.Name, stem) {
					t.Errorf("v%s %s/%s kept namespace stem %q in its name", version, token.Family, token.Name, stem)
				}
			}
			if strings.HasPrefix(token.Name, "-") {
				t.Errorf("v%s %s/%s kept a leading dash", version, token.Family, token.Name)
			}
		}
	}
}

func TestDefaultTablesHaveNoDuplicateRows(t *testing.T) {
	tables := []struct {
		version string
		rows    []entry
	}{{"3", version3}, {"4", version4}}
	for _, table := range tables {
		seen := map[string]bool{}
		for _, row := range table.rows {
			key := string(row.family) + "/" + row.name
			if seen[key] {
				t.Errorf("v%s table declares %s twice", table.version, key)
			}
			seen[key] = true
		}
	}
}

func TestKnownDefaultValues(t *testing.T) {
	testCases := []struct {
		version string
		family  tokens.Family
		name    string
		value   string
	}{
		{"3", tokens.FamilyColor, "red-500", "#ef4444"},
		{"3", tokens.FamilySpacing, "4", "1rem"},
		{"3", tokens.FamilyBreakpoint, "md", "768px"},
		{"3", tokens.FamilyRadius, "DEFAULT", "0.25rem"},
		{"4", tokens.FamilyBreakpoint, "md", "48rem"},
		{"4", tokens.FamilyFontSize, "xl", "1.25rem"},
	}
	for _, testCase := range testCases {
		token, found := Theme(testCase.version).ByName(testCase.family, testCase.name)
		if !found {
			t.Errorf("v%s %s/%s missing", testCase.version, testCase.family, testCase.name)
			continue
		}
		if token.Value != testCase.value {
			t.Errorf("v%s %s/%s = %q, want %q", testCase.version, testCase.family, testCase.name, token.Value, testCase.value)
		}
	}
}

func TestFamilyCountFloors(t *testing.T) {
	testCases := []struct {
		version string
		family  tokens.Family
		atLeast int
	}{
		{"3", tokens.FamilyColor, 240},
		{"3", tokens.FamilySpacing, 30},
		{"3", tokens.FamilyFontSize, 10},
		{"3", tokens.FamilyBreakpoint, 5},
		{"4", tokens.FamilyColor, 240},
		{"4", tokens.FamilyFontSize, 10},
		{"4", tokens.FamilyBreakpoint, 5},
	}
	for _, testCase := range testCases {
		if got := Theme(testCase.version).Count(testCase.family); got < testCase.atLeast {
			t.Errorf("v%s %s count = %d, want at least %d", testCase.version, testCase.family, got, testCase.atLeast)
		}
	}
}

func TestUnknownVersionIsEmptyNotNil(t *testing.T) {
	theme := Theme("5")
	if theme == nil {
		t.Fatal("Theme returned nil")
	}
	if len(theme.Tokens()) != 0 {
		t.Errorf("got %d tokens for an unknown version, want 0", len(theme.Tokens()))
	}
}

func TestThemeReturnsAFreshInventory(t *testing.T) {
	first := Theme("4")
	first.ClearAll()
	if len(Theme("4").Tokens()) == 0 {
		t.Error("Theme handed out shared state")
	}
}
