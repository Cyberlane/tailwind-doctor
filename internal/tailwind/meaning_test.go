package tailwind

import (
	"testing"

	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

func TestClassifyUtilitySeparatesAmbiguousPrefixes(t *testing.T) {
	inventory := tokens.NewInventory()
	for _, token := range []tokens.Token{
		{Family: tokens.FamilyColor, Name: "brand"},
		{Family: tokens.FamilyFontSize, Name: "display"},
	} {
		inventory.Put(token)
	}
	testCases := []struct {
		utility  string
		property string
		family   tokens.Family
		name     string
	}{
		{"text-brand", "color", tokens.FamilyColor, "brand"},
		{"text-display", "font-size", tokens.FamilyFontSize, "display"},
		{"text-center", "text-align", "", ""},
		{"bg-brand", "background-color", tokens.FamilyColor, "brand"},
		{"bg-cover", "background-size", "", ""},
		{"border-r", "border-r-width", "", ""},
		{"border-brand", "border-color", tokens.FamilyColor, "brand"},
		{"border-r-brand", "border-r-color", tokens.FamilyColor, "brand"},
		{"font-bold", "font-weight", tokens.FamilyFontWeight, "bold"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.utility, func(t *testing.T) {
			meaning := ClassifyUtility(testCase.utility, inventory)
			if meaning.Property != testCase.property || meaning.Family != testCase.family || meaning.TokenName != testCase.name {
				t.Fatalf("meaning = %+v", meaning)
			}
		})
	}
}

func TestClassifyUtilityFallsBackToDefaultThemeWithoutInventory(t *testing.T) {
	testCases := []struct {
		utility  string
		property string
		family   tokens.Family
	}{
		{"text-4xl", "font-size", tokens.FamilyFontSize},
		{"text-xs", "font-size", tokens.FamilyFontSize},
		{"text-gray-600", "color", tokens.FamilyColor},
		{"text-white", "color", tokens.FamilyColor},
		{"bg-gray-100", "background-color", tokens.FamilyColor},
		{"border-gray-200", "border-color", tokens.FamilyColor},
		{"font-sans", "font-family", tokens.FamilyFontFamily},
	}
	for _, testCase := range testCases {
		t.Run(testCase.utility, func(t *testing.T) {
			meaning := ClassifyUtility(testCase.utility, nil)
			if meaning.Property != testCase.property || meaning.Family != testCase.family {
				t.Fatalf("meaning = %+v, want property %q family %q",
					meaning, testCase.property, testCase.family)
			}
		})
	}
}

// A side suffix does not make a utility a width: border-l-transparent colors
// the left edge, and border-y and border-r touch different edges entirely.
// Collapsing them all into "border-width" reported overrides as conflicts.
func TestClassifyUtilitySeparatesBorderSidesAndKinds(t *testing.T) {
	testCases := []struct {
		utility  string
		property string
	}{
		{"border-y", "border-y-width"},
		{"border-t-2", "border-t-width"},
		{"border-l-transparent", "border-l-color"},
		{"border-transparent", "border-color"},
		{"border-t-[#eee]", "border-t-color"},
		{"bg-transparent", "background-color"},
		{"bg-clip-padding", "background-clip"},
		{"bg-origin-border", "background-origin"},
		{"text-transparent", "color"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.utility, func(t *testing.T) {
			meaning := ClassifyUtility(testCase.utility, nil)
			if meaning.Property != testCase.property {
				t.Fatalf("meaning = %+v, want property %q", meaning, testCase.property)
			}
		})
	}
}

func TestClassifyUtilityRecognizesTableBorderModel(t *testing.T) {
	for _, utility := range []string{"border-collapse", "border-separate"} {
		t.Run(utility, func(t *testing.T) {
			meaning := ClassifyUtility(utility, nil)
			if meaning.Property != "border-collapse" {
				t.Fatalf("meaning = %+v, want property %q", meaning, "border-collapse")
			}
		})
	}
}

func TestClassifyUtilityPreservesArbitrarySuggestionShape(t *testing.T) {
	meaning := ClassifyUtility("text-[#abc]/50", tokens.NewInventory())
	if meaning.Family != tokens.FamilyColor || meaning.ArbitraryValue != "#abc" ||
		meaning.SuggestionPrefix != "text-" || meaning.SuggestionSuffix != "/50" {
		t.Fatalf("meaning = %+v", meaning)
	}
}

func TestClassifyUtilityRecognizesArbitraryNamedColor(t *testing.T) {
	meaning := ClassifyUtility("text-[red]", tokens.NewInventory())
	if meaning.Property != "color" || meaning.Family != tokens.FamilyColor || meaning.ArbitraryValue != "red" {
		t.Errorf("meaning = %#v", meaning)
	}
}

func TestIsKnownUtilitySeparatesTailwindFromApplicationClasses(t *testing.T) {
	for _, utility := range []string{"p-4", "grid-cols-3", "text-sm", "z-[100]"} {
		if !IsKnownUtility(utility, nil) {
			t.Errorf("%q should be recognized", utility)
		}
	}
	for _, class := range []string{"card", "prose-shell", "product-grid"} {
		if IsKnownUtility(class, nil) {
			t.Errorf("application class %q should not be recognized", class)
		}
	}
}
