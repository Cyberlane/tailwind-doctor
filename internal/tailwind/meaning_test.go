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
		{"border-r", "border-width", "", ""},
		{"border-brand", "border-color", tokens.FamilyColor, "brand"},
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

func TestClassifyUtilityPreservesArbitrarySuggestionShape(t *testing.T) {
	meaning := ClassifyUtility("text-[#abc]/50", tokens.NewInventory())
	if meaning.Family != tokens.FamilyColor || meaning.ArbitraryValue != "#abc" ||
		meaning.SuggestionPrefix != "text-" || meaning.SuggestionSuffix != "/50" {
		t.Fatalf("meaning = %+v", meaning)
	}
}
