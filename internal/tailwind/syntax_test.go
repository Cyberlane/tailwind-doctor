package tailwind

import "testing"

func TestParseUtilityUnderstandsSyntax(t *testing.T) {
	testCases := []struct {
		name      string
		raw       string
		syntax    UtilitySyntax
		base      string
		variants  []string
		negative  bool
		important bool
	}{
		{name: "plain utility", raw: "p-4", syntax: DefaultUtilitySyntax(), base: "p-4"},
		{name: "stacked variants", raw: "hover:md:p-4", syntax: DefaultUtilitySyntax(), base: "p-4", variants: []string{"hover", "md"}},
		{name: "arbitrary value containing a colon", raw: "text-[color:red]", syntax: DefaultUtilitySyntax(), base: "text-[color:red]"},
		{name: "v3 important marker", raw: "!p-4", syntax: DefaultUtilitySyntax(), base: "p-4", important: true},
		{name: "v4 important marker", raw: "p-4!", syntax: DefaultUtilitySyntax(), base: "p-4", important: true},
		{name: "negative before the prefix", raw: "-tw-mt-2", syntax: UtilitySyntax{Prefix: "tw-", Separator: ":"}, base: "mt-2", negative: true},
		{name: "custom separator", raw: "hover_p-4", syntax: UtilitySyntax{Separator: "_"}, base: "p-4", variants: []string{"hover"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed := ParseUtility(testCase.raw, testCase.syntax)
			if parsed.Base != testCase.base {
				t.Errorf("Base = %q, want %q", parsed.Base, testCase.base)
			}
			if len(parsed.Variants) != len(testCase.variants) {
				t.Fatalf("Variants = %v, want %v", parsed.Variants, testCase.variants)
			}
			for index, variant := range testCase.variants {
				if parsed.Variants[index] != variant {
					t.Errorf("Variants[%d] = %q, want %q", index, parsed.Variants[index], variant)
				}
			}
			if parsed.Negative != testCase.negative {
				t.Errorf("Negative = %v, want %v", parsed.Negative, testCase.negative)
			}
			if parsed.Important != testCase.important {
				t.Errorf("Important = %v, want %v", parsed.Important, testCase.important)
			}
		})
	}
}

func TestVariantKeyIsOrderIndependent(t *testing.T) {
	first := ParseUtility("hover:md:p-4", DefaultUtilitySyntax())
	second := ParseUtility("md:hover:p-4", DefaultUtilitySyntax())
	if first.VariantKey() != second.VariantKey() {
		t.Errorf("VariantKey differs: %q vs %q", first.VariantKey(), second.VariantKey())
	}
}

func TestHasArbitraryValueIgnoresArbitraryVariants(t *testing.T) {
	if ParseUtility("[&_svg]:size-4", DefaultUtilitySyntax()).HasArbitraryValue() {
		t.Error("an arbitrary variant is not an arbitrary value")
	}
	if !ParseUtility("text-[#123456]", DefaultUtilitySyntax()).HasArbitraryValue() {
		t.Error("text-[#123456] has an arbitrary value")
	}
}

func TestParseUtilityStripsAVariantPrefix(t *testing.T) {
	syntax := UtilitySyntax{Prefix: "tw", PrefixIsVariant: true, Separator: ":"}
	parsed := ParseUtility("tw:p-4", syntax)
	if parsed.Base != "p-4" || len(parsed.Variants) != 0 {
		t.Errorf("parsed = %+v, want unvaried p-4", parsed)
	}
	stacked := ParseUtility("tw:hover:p-4", syntax)
	if stacked.Base != "p-4" || len(stacked.Variants) != 1 || stacked.Variants[0] != "hover" {
		t.Errorf("stacked = %+v", stacked)
	}
	negative := ParseUtility("-tw:mt-2", syntax)
	if negative.Base != "mt-2" || !negative.Negative {
		t.Errorf("negative = %+v", negative)
	}
}
