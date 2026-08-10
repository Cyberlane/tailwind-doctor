package audit

import (
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScannedExposurePerUnit(t *testing.T) {
	scanned := Scanned{Files: 2, ClassLists: 7, Utilities: 41, Tokens: 9, HighConfidenceTokens: 6, MediumConfidenceTokens: 3, ColorPairs: 5}

	if got := scanned.exposure(ExposureUtility, ConfidenceHigh); got != 41 {
		t.Errorf("utility exposure = %d, want 41", got)
	}
	if got := scanned.exposure(ExposureClassList, ConfidenceHigh); got != 7 {
		t.Errorf("class-list exposure = %d, want 7", got)
	}
	if got := scanned.exposure(ExposureToken, ConfidenceHigh); got != 6 {
		t.Errorf("high-confidence token exposure = %d, want 6", got)
	}
	if got := scanned.exposure(ExposureToken, ConfidenceMedium); got != 9 {
		t.Errorf("medium-confidence token exposure = %d, want 9", got)
	}
	if got := scanned.exposure(ExposureColorPair, ConfidenceHigh); got != 5 {
		t.Errorf("color-pair exposure = %d, want 5", got)
	}
	if got := scanned.exposure(Exposure("nonsense"), ConfidenceHigh); got != 0 {
		t.Errorf("an unknown unit exposes nothing, got %d", got)
	}
}

// The published table in docs/scoring.md. If any row moves, the document and the
// implementation have diverged and one of them is lying.
func TestTransferMatchesThePublishedTable(t *testing.T) {
	cases := []struct {
		density *big.Rat
		want    int
	}{
		{big.NewRat(0, 1), 100},
		{big.NewRat(1, 100), 95},
		{big.NewRat(2, 100), 91},
		{big.NewRat(4, 100), 83},
		{big.NewRat(10, 100), 67},
		{big.NewRat(20, 100), 50},
		{big.NewRat(40, 100), 33},
		{big.NewRat(1, 1), 17},
	}
	for _, testCase := range cases {
		if got := transfer(testCase.density); got != testCase.want {
			t.Errorf("transfer(%s) = %d, want %d", testCase.density.RatString(), got, testCase.want)
		}
	}
}

// maximumReachableDensity is the largest D the current rule set can produce.
// Each rule's rate is bounded by one finding per unit of its own exposure, so
// the sum is bounded by the sum of the category weights involved: 2 for
// Summing every current rule's category weight gives the conservative bound:
// 2 arbitrary + 3 conflicts + 3 overlaps + 1 variant density + 2 unused token
// + 4 contrast. Retired rules contribute nothing.
const maximumReachableDensity = 15

// The score must keep falling across the whole range a real codebase can reach,
// and must not bottom out inside it. That is the entire reason this replaced
// 100 - 2 x findings, which flattened to zero at fifty findings.
func TestTransferFallsAcrossEveryReachableDensity(t *testing.T) {
	previous := MaximumScore + 1
	for numerator := int64(0); numerator <= maximumReachableDensity*10; numerator++ {
		score := transfer(big.NewRat(numerator, 10))
		if score > previous {
			t.Fatalf("density %s scored %d, above the previous %d",
				big.NewRat(numerator, 10).RatString(), score, previous)
		}
		if score <= 0 {
			t.Fatalf("density %s scored %d: the scale must not bottom out at a reachable density",
				big.NewRat(numerator, 10).RatString(), score)
		}
		previous = score
	}
}

// An integer scale has to reach zero somewhere. The claim worth pinning is where:
// 100 x H / (H + D) rounds to zero only above D = 39.8, which is more than six
// times the worst the rule set can produce, so no codebase can reach the floor.
func TestTransferOnlyReachesZeroBeyondAnyPossibleDebt(t *testing.T) {
	if score := transfer(big.NewRat(maximumReachableDensity, 1)); score < 1 {
		t.Errorf("the worst reachable density scored %d, want at least 1", score)
	}
	if score := transfer(big.NewRat(398, 10)); score == 0 {
		t.Errorf("D = 39.8 scored 0, want the last non-zero score")
	}
	if score := transfer(big.NewRat(400, 10)); score != 0 {
		t.Errorf("D = 40 scored %d; the scale is expected to have run out by there", score)
	}
}

func TestScoresRequiresErrorSeverityAndConfidence(t *testing.T) {
	config := defaultConfig()

	cases := []struct {
		name    string
		finding Finding
		want    bool
	}{
		{"high confidence error", Finding{Severity: SeverityError, Confidence: ConfidenceHigh}, true},
		{"medium confidence error", Finding{Severity: SeverityError, Confidence: ConfidenceMedium}, false},
		{"high confidence warning", Finding{Severity: SeverityWarn, Confidence: ConfidenceHigh}, false},
		{"unknown confidence", Finding{Severity: SeverityError, Confidence: Confidence("guess")}, false},
	}
	for _, testCase := range cases {
		if got := config.scores(testCase.finding); got != testCase.want {
			t.Errorf("%s: scores = %v, want %v", testCase.name, got, testCase.want)
		}
	}

	config.MinConfidence = ConfidenceMedium
	if !config.scores(Finding{Severity: SeverityError, Confidence: ConfidenceMedium}) {
		t.Error("lowering min-confidence should let medium findings score")
	}
}

// The mixed example worked through in docs/scoring.md, end to end.
func TestCategoryScoresMatchTheWorkedExample(t *testing.T) {
	scanned := Scanned{Files: 40, ClassLists: 400, Utilities: 2000}
	findings := make([]Finding, 0, 38)
	for index := 0; index < 20; index++ {
		findings = append(findings, Finding{
			Rule: "no-arbitrary-value", Category: CategoryConsistency,
			Severity: SeverityError, Confidence: ConfidenceHigh,
		})
	}
	for index := 0; index < 10; index++ {
		findings = append(findings, Finding{
			Rule: "no-conflicting-utilities", Category: CategoryCorrectness,
			Severity: SeverityError, Confidence: ConfidenceHigh,
		})
	}
	for index := 0; index < 8; index++ {
		findings = append(findings, Finding{
			Rule: "responsive-bloat", Category: CategoryMaintainability,
			Severity: SeverityError, Confidence: ConfidenceMedium,
		})
	}

	config := defaultConfig()
	if score := transfer(weightedDensity(findings, scanned, config, nil)); score != 85 {
		t.Errorf("headline score = %d, want 85", score)
	}

	byName := map[Category]CategoryScore{}
	for _, category := range categoryScores(findings, scanned, config) {
		byName[category.Name] = category
	}

	// Accessibility's introductory rule is disabled by default. Reporting 100
	// would read as "accessible" where it means "not measured".
	if accessibility := byName[CategoryAccessibility]; accessibility.Score != nil {
		t.Errorf("accessibility score = %d, want null", *accessibility.Score)
	}
	for _, expected := range []struct {
		category Category
		score    int
	}{
		{CategoryCorrectness, 93},
		{CategoryConsistency, 91},
	} {
		actual := byName[expected.category]
		if actual.Score == nil {
			t.Errorf("%s score = null, want %d", expected.category, expected.score)
			continue
		}
		if *actual.Score != expected.score {
			t.Errorf("%s score = %d, want %d", expected.category, *actual.Score, expected.score)
		}
	}

	maintainability := byName[CategoryMaintainability]
	if maintainability.Score != nil {
		t.Errorf("maintainability score = %d, want null while its replacement rule is opt-in", *maintainability.Score)
	}
	if maintainability.ScoredFindings != 0 || maintainability.UnscoredFindings != 8 {
		t.Errorf("maintainability counted %d scored and %d unscored, want 0 and 8",
			maintainability.ScoredFindings, maintainability.UnscoredFindings)
	}
}

// The categories in every report appear in a fixed order, because byte-identical
// output is a product boundary.
func TestCategoryScoresAreOrderedDeterministically(t *testing.T) {
	first := categoryScores(nil, Scanned{}, defaultConfig())
	second := categoryScores(nil, Scanned{}, defaultConfig())
	if len(first) != len(categoryOrder) {
		t.Fatalf("reported %d categories, want %d", len(first), len(categoryOrder))
	}
	for index := range first {
		if first[index].Name != categoryOrder[index] || first[index].Name != second[index].Name {
			t.Fatalf("category order is not fixed: %#v then %#v", first, second)
		}
	}
}

func TestCategoryScoresAreUnmeasuredWithoutExposure(t *testing.T) {
	for _, category := range categoryScores(nil, Scanned{}, defaultConfig()) {
		if category.Score != nil {
			t.Errorf("%s score = %d, want null without exposure", category.Name, *category.Score)
		}
	}
}

func TestCategoryScoresPublishEveryEnabledExposure(t *testing.T) {
	config := defaultConfig()
	config.Severities["unused-token"] = SeverityError
	scores := categoryScores(nil, Scanned{Utilities: 20, Tokens: 3, HighConfidenceTokens: 3}, config)

	var consistency CategoryScore
	for _, score := range scores {
		if score.Name == CategoryConsistency {
			consistency = score
		}
	}
	want := []ExposureCount{{Unit: ExposureUtility, Count: 20}, {Unit: ExposureToken, Count: 3}}
	if len(consistency.Exposures) != len(want) {
		t.Fatalf("exposures = %+v", consistency.Exposures)
	}
	for index := range want {
		if consistency.Exposures[index] != want[index] {
			t.Errorf("exposure %d = %+v, want %+v", index, consistency.Exposures[index], want[index])
		}
	}
}

func TestMinConfidenceIsConfigurable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFileName),
		[]byte("[score]\nmin-confidence = \"medium\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	config, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.MinConfidence != ConfidenceMedium {
		t.Errorf("min-confidence = %q, want medium", config.MinConfidence)
	}

	if err := os.WriteFile(filepath.Join(root, ConfigFileName),
		[]byte("[score]\nmin-confidence = \"probably\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(root); err == nil || !strings.Contains(err.Error(), "min-confidence") {
		t.Errorf("an invalid tier should be refused, got %v", err)
	}
}
