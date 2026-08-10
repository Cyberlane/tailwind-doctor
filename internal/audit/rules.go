package audit

import "math/big"

// Category groups rules by what it costs a user to ignore them. The weight a
// category carries is what makes one kind of debt move the score more than
// another; the values are argued for in docs/scoring.md and frozen per
// ScoreModelVersion.
type Category string

const (
	CategoryAccessibility   Category = "accessibility"
	CategoryCorrectness     Category = "correctness"
	CategoryConsistency     Category = "consistency"
	CategoryMaintainability Category = "maintainability"
)

// categoryOrder fixes the order categories appear in every report. Ranging over
// a map would order them differently between runs, and byte-identical output is
// a product boundary.
var categoryOrder = []Category{
	CategoryAccessibility,
	CategoryCorrectness,
	CategoryConsistency,
	CategoryMaintainability,
}

var categoryWeights = map[Category]int64{
	CategoryAccessibility:   4,
	CategoryCorrectness:     3,
	CategoryConsistency:     2,
	CategoryMaintainability: 1,
}

// categoryWeight returns nil for a category that carries no weight, which can
// only happen if a rule names one that does not exist.
func categoryWeight(category Category) *big.Rat {
	weight, known := categoryWeights[category]
	if !known {
		return nil
	}
	return new(big.Rat).SetInt64(weight)
}

// Exposure names the unit a rule is measured against, and therefore the
// denominator its rate is computed over. There is deliberately no single global
// denominator: an arbitrary value is a property of one utility while responsive
// bloat is a property of a whole class list, so one denominator would misprice
// one of them. See docs/scoring.md.
type Exposure string

const (
	ExposureUtility   Exposure = "utility"
	ExposureClassList Exposure = "class-list"
	ExposureToken     Exposure = "token"
	ExposureColorPair Exposure = "color-pair"
)

// Confidence records how sure the tool is that a finding is real. Only high
// confidence moves the score by default; anything less is reported and tagged,
// because a visibly uncertain finding costs less trust than a silent miss.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// confidenceRank orders the tiers so a minimum can be compared numerically. An
// unrecognised tier ranks 0, below every real one, so a typo stops findings
// scoring rather than letting everything score.
func confidenceRank(confidence Confidence) int {
	switch confidence {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	}
	return 0
}

// RuleDefinition is the frozen public description of one rule. Identifiers are
// never renamed and never repurposed; a retired rule keeps its identifier and
// reports nothing. See docs/rule-stability.md.
type RuleDefinition struct {
	ID                string
	Category          Category
	Exposure          Exposure
	DefaultSeverity   Severity
	DefaultConfidence Confidence
	// Since is the release that introduced the rule. DefaultOn is false for one
	// minor release after that, because adding a rule changes every user's score
	// and can break a passing CI gate without anyone touching their code.
	Since     string
	DefaultOn bool
}

var ruleRegistry = []RuleDefinition{
	{
		ID: "no-arbitrary-value", Category: CategoryConsistency, Exposure: ExposureUtility,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceHigh,
		Since: "0.1.0", DefaultOn: true,
	},
	{
		ID: "no-conflicting-utilities", Category: CategoryCorrectness, Exposure: ExposureUtility,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceHigh,
		Since: "0.1.0", DefaultOn: true,
	},
	{
		// Medium by default: a five-variant threshold is a heuristic with no
		// defect behind it, so it is reported and score-neutral.
		ID: "responsive-bloat", Category: CategoryMaintainability, Exposure: ExposureClassList,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceMedium,
		Since: "0.1.0", DefaultOn: false,
	},
	{
		ID: "variant-density", Category: CategoryMaintainability, Exposure: ExposureClassList,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceMedium,
		Since: "0.2.0", DefaultOn: false,
	},
	{
		ID: "no-overlapping-utilities", Category: CategoryCorrectness, Exposure: ExposureUtility,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceMedium,
		Since: "0.2.0", DefaultOn: false,
	},
	{
		ID: "unused-token", Category: CategoryConsistency, Exposure: ExposureToken,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceHigh,
		Since: "0.2.0", DefaultOn: false,
	},
	{
		ID: "color-contrast", Category: CategoryAccessibility, Exposure: ExposureColorPair,
		DefaultSeverity: SeverityError, DefaultConfidence: ConfidenceHigh,
		Since: "0.3.0", DefaultOn: false,
	},
}

func lookupRule(id string) (RuleDefinition, bool) {
	for _, rule := range ruleRegistry {
		if rule.ID == id {
			return rule, true
		}
	}
	return RuleDefinition{}, false
}
