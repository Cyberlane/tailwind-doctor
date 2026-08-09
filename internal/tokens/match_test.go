package tokens

import "testing"

func TestNormalize(t *testing.T) {
	testCases := []struct {
		name       string
		raw        string
		value      string
		resolvable bool
	}{
		{name: "lowercases hex", raw: "#3B82F6", value: "#3b82f6", resolvable: true},
		{name: "expands short hex", raw: "#ABC", value: "#aabbcc", resolvable: true},
		{name: "expands short hex with alpha", raw: "#abcd", value: "#aabbccdd", resolvable: true},
		{name: "adds a leading zero", raw: ".25rem", value: "0.25rem", resolvable: true},
		{name: "strips trailing zeros", raw: "0.250rem", value: "0.25rem", resolvable: true},
		{name: "strips a trailing point", raw: "1.0rem", value: "1rem", resolvable: true},
		{name: "collapses inner whitespace", raw: "0 1px  2px  #000", value: "0 1px 2px #000000", resolvable: true},
		{name: "trims", raw: "  1rem  ", value: "1rem", resolvable: true},
		{name: "keeps units distinct", raw: "16px", value: "16px", resolvable: true},
		{name: "refuses var()", raw: "var(--brand)", resolvable: false},
		{name: "refuses calc()", raw: "calc(100% - 1rem)", resolvable: false},
		{name: "refuses an empty value", raw: "   ", resolvable: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			value, resolvable := Normalize(testCase.raw)
			if resolvable != testCase.resolvable {
				t.Fatalf("resolvable = %v, want %v", resolvable, testCase.resolvable)
			}
			if resolvable && value != testCase.value {
				t.Errorf("value = %q, want %q", value, testCase.value)
			}
		})
	}
}

func TestNormalizeDoesNotConvertUnits(t *testing.T) {
	pixels, _ := Normalize("16px")
	rems, _ := Normalize("1rem")
	if pixels == rems {
		t.Error("16px must not normalize to the same value as 1rem: the conversion needs a root font size nobody declared")
	}
}

func TestLookupFindsATokenByValue(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "brand-500", Value: "#3b82f6", Raw: "#3B82F6", Origin: OriginProject})

	token, found := inventory.Lookup(FamilyColor, "#3b82f6")
	if !found {
		t.Fatal("exact value not found")
	}
	if token.Name != "brand-500" {
		t.Errorf("Name = %q, want brand-500", token.Name)
	}

	if _, found := inventory.Lookup(FamilySpacing, "#3b82f6"); found {
		t.Error("a colour value must not match in the spacing family")
	}
}

func TestLookupNormalizesTheQuery(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "brand-500", Value: "#3b82f6", Origin: OriginProject})

	if _, found := inventory.Lookup(FamilyColor, "#3B82F6"); !found {
		t.Error("lookup should normalize the query before comparing")
	}
}

func TestLookupPrefersTheFirstNameInOrderOnATie(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "primary", Value: "#3b82f6", Origin: OriginProject})
	inventory.Put(Token{Family: FamilyColor, Name: "brand", Value: "#3b82f6", Origin: OriginProject})

	for attempt := 0; attempt < 20; attempt++ {
		token, found := inventory.Lookup(FamilyColor, "#3b82f6")
		if !found {
			t.Fatal("not found")
		}
		if token.Name != "brand" {
			t.Fatalf("Name = %q, want brand: a tie must resolve by name order, not map order", token.Name)
		}
	}
}

func TestLookupIgnoresUnresolvableTokens(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "brand", Raw: "var(--x)", Unresolvable: true, Origin: OriginProject})

	if _, found := inventory.Lookup(FamilyColor, "var(--x)"); found {
		t.Error("an unresolvable token must never be suggested")
	}
}
