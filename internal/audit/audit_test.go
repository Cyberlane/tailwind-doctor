package audit

import "testing"

func TestInspectFindsHighConfidenceProblems(t *testing.T) {
	findings := inspect("src/card.tsx", "p-4 p-2 text-[#123456] sm:p-2 md:p-4 lg:p-6 xl:p-8 2xl:p-10")

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %#v", len(findings), findings)
	}
}

func TestInspectKeepsVariantsSeparate(t *testing.T) {
	findings := inspect("src/card.tsx", "p-4 md:p-6")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}
