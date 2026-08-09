package audit

import "math/big"

// Scanned records what the analysis was exposed to. These are the score's
// denominators, and they count resolved class lists only: an unresolvable
// className contributes to no denominator, because counting it would dilute
// measured debt in proportion to how much of a codebase cannot be read.
type Scanned struct {
	Files                  int `json:"files"`
	ClassLists             int `json:"classLists"`
	Utilities              int `json:"utilities"`
	Tokens                 int `json:"tokens"`
	HighConfidenceTokens   int `json:"highConfidenceTokens"`
	MediumConfidenceTokens int `json:"mediumConfidenceTokens"`
	ColorPairs             int `json:"colorPairs"`
}

// exposure returns the denominator for one exposure unit. An unrecognised unit
// exposes nothing, which makes its rate zero rather than dividing by a number
// that means something else.
func (scanned Scanned) exposure(unit Exposure, minimum Confidence) int64 {
	switch unit {
	case ExposureUtility:
		return int64(scanned.Utilities)
	case ExposureClassList:
		return int64(scanned.ClassLists)
	case ExposureToken:
		switch minimum {
		case ConfidenceLow:
			return int64(scanned.Tokens)
		case ConfidenceMedium:
			return int64(scanned.HighConfidenceTokens + scanned.MediumConfidenceTokens)
		default:
			return int64(scanned.HighConfidenceTokens)
		}
	case ExposureColorPair:
		return int64(scanned.ColorPairs)
	}
	return 0
}

// ScoreModelVersion identifies the frozen set of weights, exposure units, and
// the transfer function. Changing any of them is a minor release with a
// changelog entry; a consumer compares this before comparing two scores. See
// docs/rule-stability.md.
const ScoreModelVersion = 1

// halfScoreDensity is H: the weighted debt density at which a project scores 50.
// Set from a principle rather than fitted to a repository — one utility in ten
// being a consistency violation scores 50, and consistency carries weight 2, so
// H is 0.10 x 2. Frozen in score model v1; see docs/scoring.md.
var halfScoreDensity = big.NewRat(1, 5)

// scores reports whether a finding moves the score. Severity is what the project
// configured; confidence is what the tool is willing to stand behind. Both have
// to clear the bar, so a rule can be reported long before it is trusted enough
// to gate a build on.
func (config Config) scores(finding Finding) bool {
	if finding.Severity != SeverityError {
		return false
	}
	minimum := config.MinConfidence
	if minimum == "" {
		minimum = ConfidenceHigh
	}
	rank := confidenceRank(finding.Confidence)
	return rank != 0 && rank >= confidenceRank(minimum)
}

// weightedDensity is D: the sum over rules of the category weight times the
// rule's scored findings divided by the exposure of the unit it is measured
// against. Passing a category restricts the sum to that category, which is what
// produces a sub-score.
//
// The arithmetic is rational and therefore exact. A float would make "identical
// input, byte-identical output" true per architecture rather than absolutely,
// and that guarantee is a product boundary.
func weightedDensity(findings []Finding, scanned Scanned, config Config, only *Category) *big.Rat {
	counts := map[string]int64{}
	for _, finding := range findings {
		if !config.scores(finding) {
			continue
		}
		if only != nil && finding.Category != *only {
			continue
		}
		counts[finding.Rule]++
	}

	total := new(big.Rat)
	// Ranging the registry rather than the map keeps the summation order fixed.
	for _, rule := range ruleRegistry {
		count := counts[rule.ID]
		if count == 0 {
			continue
		}
		exposure := scanned.exposure(rule.Exposure, config.MinConfidence)
		if exposure == 0 {
			// A finding implies at least one unit of its own exposure, so this
			// is unreachable; reading it as no debt is the safe direction.
			continue
		}
		weight := categoryWeight(rule.Category)
		if weight == nil {
			continue
		}
		rate := new(big.Rat).SetFrac64(count, exposure)
		total.Add(total, rate.Mul(rate, weight))
	}
	return total
}

// transfer maps a debt density onto 0-100 as 100 x H / (H + D). There is no
// clamp: the previous formula awarded zero to everything past a threshold, so a
// bad codebase and an awful one were indistinguishable and neither could show
// improvement. Ties round up, toward the more favourable score.
func transfer(debt *big.Rat) int {
	denominator := new(big.Rat).Add(halfScoreDensity, debt)
	value := new(big.Rat).Quo(halfScoreDensity, denominator)
	value.Mul(value, new(big.Rat).SetInt64(MaximumScore))
	value.Add(value, big.NewRat(1, 2))

	// The value is never negative, so truncating the quotient floors it, and
	// floor(v + 1/2) is round-half-up.
	rounded := new(big.Int).Quo(value.Num(), value.Denom())
	return int(rounded.Int64())
}

// CategoryScore is one dimension of the report. Score is nil when the category
// has no enabled scoring rule: publishing 100 there would read as "clean" where
// it means "not measured", and that one misreading costs more credibility than
// the sub-scores earn.
type CategoryScore struct {
	Name             Category        `json:"name"`
	Score            *int            `json:"score"`
	Exposures        []ExposureCount `json:"exposures"`
	ScoredFindings   int             `json:"scoredFindings"`
	UnscoredFindings int             `json:"unscoredFindings"`
}

// ExposureCount publishes every denominator an enabled category uses. A
// category may contain rules measured against different units.
type ExposureCount struct {
	Unit  Exposure `json:"unit"`
	Count int      `json:"count"`
}

func categoryScores(findings []Finding, scanned Scanned, config Config) []CategoryScore {
	scores := make([]CategoryScore, 0, len(categoryOrder))
	for _, category := range categoryOrder {
		current := CategoryScore{Name: category, Exposures: []ExposureCount{}}

		measured := false
		seenExposures := map[Exposure]bool{}
		for _, rule := range ruleRegistry {
			if rule.Category != category {
				continue
			}
			if config.severityFor(rule.ID) == SeverityError {
				measured = true
				if !seenExposures[rule.Exposure] {
					seenExposures[rule.Exposure] = true
					current.Exposures = append(current.Exposures, ExposureCount{
						Unit: rule.Exposure, Count: int(scanned.exposure(rule.Exposure, config.MinConfidence)),
					})
				}
			}
		}

		for _, finding := range findings {
			if finding.Category != category {
				continue
			}
			if config.scores(finding) {
				current.ScoredFindings++
			} else {
				current.UnscoredFindings++
			}
		}

		if measured {
			score := transfer(weightedDensity(findings, scanned, config, &category))
			current.Score = &score
		}
		scores = append(scores, current)
	}
	return scores
}
