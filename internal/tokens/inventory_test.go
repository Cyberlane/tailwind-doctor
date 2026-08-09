package tokens

import "testing"

func TestTokensAreReturnedInFamilyThenNameOrder(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilySpacing, Name: "4", Value: "1rem", Origin: OriginProject})
	inventory.Put(Token{Family: FamilyColor, Name: "brand-500", Value: "#3b82f6", Origin: OriginProject})
	inventory.Put(Token{Family: FamilyColor, Name: "brand-100", Value: "#dbeafe", Origin: OriginProject})

	got := inventory.Tokens()
	want := []struct {
		family Family
		name   string
	}{
		{FamilyColor, "brand-100"},
		{FamilyColor, "brand-500"},
		{FamilySpacing, "4"},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(got), len(want))
	}
	for index, expected := range want {
		if got[index].Family != expected.family || got[index].Name != expected.name {
			t.Errorf("token %d = %s/%s, want %s/%s",
				index, got[index].Family, got[index].Name, expected.family, expected.name)
		}
	}
}

func TestPutReplacesTheSameFamilyAndName(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "brand", Value: "#000000", Origin: OriginDefault})
	inventory.Put(Token{Family: FamilyColor, Name: "brand", Value: "#3b82f6", Origin: OriginProject})

	if inventory.Count(FamilyColor) != 1 {
		t.Fatalf("Count = %d, want 1", inventory.Count(FamilyColor))
	}
	token, found := inventory.ByName(FamilyColor, "brand")
	if !found {
		t.Fatal("brand not found")
	}
	if token.Value != "#3b82f6" || token.Origin != OriginProject {
		t.Errorf("got %s/%s, want #3b82f6/project", token.Value, token.Origin)
	}
}

func TestClearRemovesOneFamilyOnly(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "red-500", Value: "#ef4444", Origin: OriginDefault})
	inventory.Put(Token{Family: FamilySpacing, Name: "4", Value: "1rem", Origin: OriginDefault})

	inventory.Clear(FamilyColor)

	if inventory.Count(FamilyColor) != 0 {
		t.Errorf("colour count = %d, want 0", inventory.Count(FamilyColor))
	}
	if inventory.Count(FamilySpacing) != 1 {
		t.Errorf("spacing count = %d, want 1", inventory.Count(FamilySpacing))
	}
}

func TestClearAllEmptiesEveryFamily(t *testing.T) {
	inventory := NewInventory()
	inventory.Put(Token{Family: FamilyColor, Name: "red-500", Value: "#ef4444", Origin: OriginDefault})
	inventory.Put(Token{Family: FamilySpacing, Name: "4", Value: "1rem", Origin: OriginDefault})

	inventory.ClearAll()

	if len(inventory.Tokens()) != 0 {
		t.Errorf("got %d tokens, want 0", len(inventory.Tokens()))
	}
}

func TestFamiliesReturnsACopy(t *testing.T) {
	first := Families()
	first[0] = "mutated"
	if Families()[0] == "mutated" {
		t.Error("Families() exposed its backing array")
	}
}
