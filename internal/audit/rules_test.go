package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The rule catalog is a public contract. A rule that exists in code but not in
// docs/rules.md is undocumented public API; one documented but unregistered is a
// promise the tool does not keep. Neither is visible in a diff.
func TestRuleCatalogMatchesRegistry(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "docs", "rules.md"))
	if err != nil {
		t.Fatalf("read docs/rules.md: %v", err)
	}

	documented := map[string]bool{}
	for _, line := range strings.Split(string(content), "\n") {
		rest, found := strings.CutPrefix(line, "## `")
		if !found {
			continue
		}
		if id, found := strings.CutSuffix(rest, "`"); found {
			documented[id] = true
		}
	}

	for _, rule := range ruleRegistry {
		if !documented[rule.ID] {
			t.Errorf("rule %q is registered but has no section in docs/rules.md", rule.ID)
		}
		delete(documented, rule.ID)
	}
	for id := range documented {
		t.Errorf("docs/rules.md documents %q, which is not in the registry", id)
	}
}

func TestEveryRegisteredRuleIsFullyDescribed(t *testing.T) {
	seen := map[string]bool{}
	for _, rule := range ruleRegistry {
		if seen[rule.ID] {
			t.Errorf("rule %q is registered twice", rule.ID)
		}
		seen[rule.ID] = true

		if categoryWeight(rule.Category) == nil {
			t.Errorf("rule %q has category %q, which carries no weight", rule.ID, rule.Category)
		}
		switch rule.Exposure {
		case ExposureUtility, ExposureClassList, ExposureToken:
		default:
			t.Errorf("rule %q has unknown exposure %q", rule.ID, rule.Exposure)
		}
		if confidenceRank(rule.DefaultConfidence) == 0 {
			t.Errorf("rule %q has unknown confidence %q", rule.ID, rule.DefaultConfidence)
		}
		if rule.Since == "" {
			t.Errorf("rule %q does not record the release that introduced it", rule.ID)
		}
	}
}

func TestLookupRuleRejectsUnknownIdentifiers(t *testing.T) {
	if _, found := lookupRule("no-arbitrary-value"); !found {
		t.Error("no-arbitrary-value should be registered")
	}
	if _, found := lookupRule("no-such-rule"); found {
		t.Error("an unregistered identifier should not resolve")
	}
}

// Every category carries a weight, and they are ordered so that a report lists
// them the same way on every run.
func TestCategoryWeightsAreCompleteAndOrdered(t *testing.T) {
	if len(categoryOrder) != len(categoryWeights) {
		t.Fatalf("%d categories are ordered but %d carry weights", len(categoryOrder), len(categoryWeights))
	}
	for _, category := range categoryOrder {
		weight := categoryWeight(category)
		if weight == nil {
			t.Errorf("category %q carries no weight", category)
			continue
		}
		if weight.Sign() <= 0 {
			t.Errorf("category %q has weight %s; a weight must be positive", category, weight.RatString())
		}
	}
	if categoryWeight(Category("invented")) != nil {
		t.Error("an unregistered category should carry no weight")
	}
}
