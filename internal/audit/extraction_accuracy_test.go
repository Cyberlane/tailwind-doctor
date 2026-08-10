package audit

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file calls the real extractor through the same extension filter Run
// applies, so the accuracy figure describes shipped behaviour rather than a
// convenient subset of it. Nothing here may be imported by non-test code.

// The -update flag is declared once for the whole package, in golden_test.go.
var resolveColumns = flag.Bool("resolve", false, "fill in ground-truth records whose column is written as ?")

const (
	corpusDirectory = "../../testdata/corpus"
	baselinePath    = corpusDirectory + "/baseline.json"
)

// statusExtract marks a class list the extractor must find. statusUnresolved
// marks a site the extractor must report as unknowable rather than invent
// classes for. statusIgnore marks a decoy that must never be extracted.
// statusSkip marks a record whose correct answer is disputed; skips are counted
// and printed but never folded into the rate. See docs/false-positive-policy.md.
const (
	statusExtract    = "extract"
	statusUnresolved = "unresolved"
	statusIgnore     = "ignore"
	statusSkip       = "skip"
)

// A record is written by hand as `<line>:<column> <status> <shape> "<value>"`.
// The column may be authored as `?` and filled in by -resolve, which only locates
// a string that a human has already decided belongs on that line. It refuses any
// line where the string appears more than once, so it can never pick for you.
var recordPattern = regexp.MustCompile(`^(\d+):(\?|\d+)\s+(\S+)\s+(\S+)\s+"(.*)"$`)

type record struct {
	line   int
	column int
	status string
	shape  string
	value  string
}

type fixture struct {
	slug      string
	inputPath string
	content   string
	records   []record
}

// shapeResult is the per-shape breakdown. Expected counts every class list of
// that shape the corpus says exists; Found counts how many the extractor
// produced. A shape whose Found is zero is a shape the extractor is blind to.
type shapeResult struct {
	Shape    string  `json:"shape"`
	Expected int     `json:"expected"`
	Found    int     `json:"found"`
	Recall   float64 `json:"recall"`
}

type falsePositive struct {
	Fixture string `json:"fixture"`
	Value   string `json:"value"`
	Reason  string `json:"reason,omitempty"`
}

type measurement struct {
	Note                      string          `json:"note"`
	Fixtures                  int             `json:"fixtures"`
	ExpectedExtractions       int             `json:"expected_extractions"`
	ActualExtractions         int             `json:"actual_extractions"`
	TruePositives             int             `json:"true_positives"`
	FalsePositives            int             `json:"false_positives"`
	FalseNegatives            int             `json:"false_negatives"`
	Skipped                   int             `json:"skipped"`
	ExpectedUnresolvedSites   int             `json:"expected_unresolved_sites"`
	UnresolvedSitesReported   int             `json:"unresolved_sites_reported"`
	Precision                 float64         `json:"precision"`
	Recall                    float64         `json:"recall"`
	PrecisionTarget           float64         `json:"precision_target"`
	EnforcePrecisionTarget    bool            `json:"enforce_precision_target"`
	PerShape                  []shapeResult   `json:"per_shape"`
	UnexplainedFalsePositives int             `json:"unexplained_false_positives"`
	AcceptedFalsePositives    []falsePositive `json:"accepted_false_positives"`
	ObservedFalsePositives    []falsePositive `json:"observed_false_positives"`
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return round(float64(numerator) / float64(denominator))
}

func unescape(value string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(value)
}

func loadCorpus(t *testing.T) []fixture {
	t.Helper()

	entries, err := os.ReadDir(corpusDirectory)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	fixtures := make([]fixture, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(corpusDirectory, entry.Name())
		inputPath, err := findInput(directory)
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		content, err := os.ReadFile(inputPath)
		if err != nil {
			t.Fatalf("read %s: %v", inputPath, err)
		}
		records, err := parseExpected(filepath.Join(directory, "expected.txt"))
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		fixtures = append(fixtures, fixture{
			slug:      entry.Name(),
			inputPath: inputPath,
			content:   string(content),
			records:   records,
		})
	}

	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].slug < fixtures[j].slug })
	if len(fixtures) == 0 {
		t.Fatal("corpus is empty")
	}
	return fixtures
}

func findInput(directory string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "input.") {
			return filepath.Join(directory, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no input file in %s", directory)
}

func parseExpected(path string) ([]record, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	records := make([]record, 0)
	for number, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		match := recordPattern.FindStringSubmatch(trimmed)
		if match == nil {
			return nil, fmt.Errorf("%s:%d: malformed record: %s", path, number+1, trimmed)
		}
		lineNumber, _ := strconv.Atoi(match[1])
		if match[2] == "?" {
			return nil, fmt.Errorf("%s:%d: column is still ?; run: go test ./internal/audit -run TestCorpusGroundTruthIsWellFormed -resolve", path, number+1)
		}
		column, _ := strconv.Atoi(match[2])
		records = append(records, record{
			line:   lineNumber,
			column: column,
			status: match[3],
			shape:  match[4],
			value:  unescape(match[5]),
		})
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].line == records[j].line {
			return records[i].column < records[j].column
		}
		return records[i].line < records[j].line
	})
	return records, nil
}

// extractedClassLists applies the same extension filter Run applies and then
// calls the real extractor, so the measurement describes shipped behaviour. Only
// resolved entries are class lists; an unresolved entry names an expression and
// is counted separately.
func extractedClassLists(inputPath, content string) ([]ClassList, int) {
	if !sourceExtensions[filepath.Ext(inputPath)] {
		return nil, 0
	}
	values := make([]ClassList, 0)
	unresolved := 0
	for _, list := range Extract(inputPath, content) {
		if list.Resolved {
			values = append(values, list)
			continue
		}
		unresolved++
	}
	return values, unresolved
}

// take consumes the extracted class list answering a ground-truth record. Value,
// line, and column must all match: findings carry positions end to end, so a
// right string attributed to the wrong source site is no longer accepted as a
// true positive.
func take(extracted []ClassList, used []bool, entry record) int {
	for index, candidate := range extracted {
		if used[index] || candidate.Value != entry.value ||
			candidate.Line != entry.line || candidate.Column != entry.column {
			continue
		}
		return index
	}
	return -1
}

// TestCorpusGroundTruthIsWellFormed checks the corpus against itself before any
// accuracy claim is made. Ground truth is hand-written, so the position and the
// string in a record can drift apart; if they have, every number below is
// meaningless and the failure should say so plainly.
func TestCorpusGroundTruthIsWellFormed(t *testing.T) {
	if *resolveColumns {
		resolveCorpusColumns(t)
		return
	}

	knownStatuses := map[string]bool{
		statusExtract: true, statusUnresolved: true, statusIgnore: true, statusSkip: true,
	}

	for _, current := range loadCorpus(t) {
		lines := strings.Split(current.content, "\n")
		for _, entry := range current.records {
			if !knownStatuses[entry.status] {
				t.Errorf("%s: unknown status %q at %d:%d", current.slug, entry.status, entry.line, entry.column)
				continue
			}
			if entry.line < 1 || entry.line > len(lines) {
				t.Errorf("%s: line %d is outside the fixture", current.slug, entry.line)
				continue
			}
			text := lines[entry.line-1]
			start := entry.column - 1
			if start < 0 || start+len(entry.value) > len(text) {
				t.Errorf("%s:%d:%d: column is outside the line", current.slug, entry.line, entry.column)
				continue
			}
			if text[start:start+len(entry.value)] != entry.value {
				t.Errorf("%s:%d:%d: expected %q, line holds %q",
					current.slug, entry.line, entry.column, entry.value, text[start:start+len(entry.value)])
			}
		}
	}
}

// TestExtractionAccuracy is the milestone's scale. It measures the extractor
// against the corpus and fails when the result is worse than the committed
// baseline. Run with -update to rewrite the baseline, and review the diff.
func TestExtractionAccuracy(t *testing.T) {
	fixtures := loadCorpus(t)
	baseline := readBaseline(t)

	measured := measure(fixtures, baseline.AcceptedFalsePositives)
	t.Log("\n" + render(measured))

	if *updateFixtures {
		writeBaseline(t, measured)
		t.Log("baseline rewritten; review testdata/corpus/baseline.json in the diff")
		return
	}

	if measured.FalsePositives > baseline.FalsePositives {
		t.Errorf("false positives rose from %d to %d", baseline.FalsePositives, measured.FalsePositives)
	}
	if measured.FalseNegatives > baseline.FalseNegatives {
		t.Errorf("false negatives rose from %d to %d", baseline.FalseNegatives, measured.FalseNegatives)
	}
	if measured.Precision < baseline.Precision {
		t.Errorf("precision fell from %.4f to %.4f", baseline.Precision, measured.Precision)
	}
	if measured.Recall < baseline.Recall {
		t.Errorf("recall fell from %.4f to %.4f", baseline.Recall, measured.Recall)
	}
	if measured.UnexplainedFalsePositives > baseline.UnexplainedFalsePositives {
		t.Errorf("unexplained false positives rose from %d to %d",
			baseline.UnexplainedFalsePositives, measured.UnexplainedFalsePositives)
	}

	// The absolute gate agreed for this project is precision >= 0.995 with no
	// unexplained false positives. It is not enforceable while the regex is in
	// place, so the baseline carries a flag; it is turned on when structural
	// parsing lands and must never be turned off again.
	if baseline.EnforcePrecisionTarget {
		if measured.Precision < baseline.PrecisionTarget {
			t.Errorf("precision %.4f is below the target %.4f", measured.Precision, baseline.PrecisionTarget)
		}
		if measured.UnexplainedFalsePositives > 0 {
			t.Errorf("%d unexplained false positive(s); each one needs a reason in baseline.json",
				measured.UnexplainedFalsePositives)
		}
	}
}

func measure(fixtures []fixture, accepted []falsePositive) measurement {
	acceptedIndex := make(map[string]string, len(accepted))
	for _, entry := range accepted {
		acceptedIndex[entry.Fixture+"\x00"+entry.Value] = entry.Reason
	}

	result := measurement{
		Note: "Extraction accuracy against testdata/corpus. Regenerate with: " +
			"go test ./internal/audit -run TestExtractionAccuracy -update",
		Fixtures:        len(fixtures),
		PrecisionTarget: 0.995,
		// Turned on when structural parsing reached the target, and never turned
		// off again. Every later extraction change is held to it.
		EnforcePrecisionTarget: true,
		AcceptedFalsePositives: accepted,
	}

	shapes := make(map[string]*shapeResult)
	observed := make([]falsePositive, 0)

	for _, current := range fixtures {
		extracted, unresolved := extractedClassLists(current.inputPath, current.content)
		result.UnresolvedSitesReported += unresolved
		result.ActualExtractions += len(extracted)
		used := make([]bool, len(extracted))

		wanted := make([]record, 0, len(current.records))
		for _, entry := range current.records {
			switch entry.status {
			case statusSkip:
				result.Skipped++
			case statusUnresolved:
				result.ExpectedUnresolvedSites++
			case statusExtract:
				wanted = append(wanted, entry)
			}
		}

		matched := make([]bool, len(wanted))
		for position, entry := range wanted {
			if index := take(extracted, used, entry); index >= 0 {
				used[index] = true
				matched[position] = true
			}
		}

		for position, entry := range wanted {
			result.ExpectedExtractions++
			shape, ok := shapes[entry.shape]
			if !ok {
				shape = &shapeResult{Shape: entry.shape}
				shapes[entry.shape] = shape
			}
			shape.Expected++
			if matched[position] {
				result.TruePositives++
				shape.Found++
			}
		}

		leftovers := make([]string, 0)
		for index, candidate := range extracted {
			if !used[index] {
				leftovers = append(leftovers, candidate.Value)
			}
		}
		sort.Strings(leftovers)
		for _, value := range leftovers {
			reason, isAccepted := acceptedIndex[current.slug+"\x00"+value]
			observed = append(observed, falsePositive{Fixture: current.slug, Value: value, Reason: reason})
			if !isAccepted {
				result.UnexplainedFalsePositives++
			}
		}
	}

	result.FalsePositives = len(observed)
	result.FalseNegatives = result.ExpectedExtractions - result.TruePositives
	result.Precision = ratio(result.TruePositives, result.TruePositives+result.FalsePositives)
	result.Recall = ratio(result.TruePositives, result.ExpectedExtractions)
	result.ObservedFalsePositives = observed

	result.PerShape = make([]shapeResult, 0, len(shapes))
	for _, shape := range shapes {
		shape.Recall = ratio(shape.Found, shape.Expected)
		result.PerShape = append(result.PerShape, *shape)
	}
	sort.Slice(result.PerShape, func(i, j int) bool { return result.PerShape[i].Shape < result.PerShape[j].Shape })

	return result
}

func render(result measurement) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "extraction accuracy over %d fixtures\n", result.Fixtures)
	fmt.Fprintf(&builder, "  expected %d, extracted %d\n", result.ExpectedExtractions, result.ActualExtractions)
	fmt.Fprintf(&builder, "  true positives  %d\n", result.TruePositives)
	fmt.Fprintf(&builder, "  false positives %d (%d unexplained)\n", result.FalsePositives, result.UnexplainedFalsePositives)
	fmt.Fprintf(&builder, "  false negatives %d\n", result.FalseNegatives)
	fmt.Fprintf(&builder, "  skipped         %d\n", result.Skipped)
	fmt.Fprintf(&builder, "  unresolved sites %d reported, %d in the corpus\n",
		result.UnresolvedSitesReported, result.ExpectedUnresolvedSites)
	fmt.Fprintf(&builder, "  precision %.4f  recall %.4f\n\n", result.Precision, result.Recall)
	fmt.Fprintf(&builder, "  %-24s %8s %8s %8s\n", "shape", "expected", "found", "recall")
	for _, shape := range result.PerShape {
		fmt.Fprintf(&builder, "  %-24s %8d %8d %8.4f\n", shape.Shape, shape.Expected, shape.Found, shape.Recall)
	}
	return builder.String()
}

// resolveCorpusColumns fills in records authored with a `?` column. It decides
// nothing: the status, shape, value, and line are all chosen by whoever wrote the
// record, and this only locates a string already known to be on that line. Where
// the string appears twice, it says so and leaves the record alone, because only
// a human can say which occurrence was meant.
func resolveCorpusColumns(t *testing.T) {
	t.Helper()

	entries, err := os.ReadDir(corpusDirectory)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(corpusDirectory, entry.Name())
		inputPath, err := findInput(directory)
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		source, err := os.ReadFile(inputPath)
		if err != nil {
			t.Fatalf("read %s: %v", inputPath, err)
		}
		lines := strings.Split(string(source), "\n")

		expectedPath := filepath.Join(directory, "expected.txt")
		expected, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("read %s: %v", expectedPath, err)
		}

		written := strings.Split(string(expected), "\n")
		for index, line := range written {
			match := recordPattern.FindStringSubmatch(strings.TrimSpace(line))
			if match == nil || match[2] != "?" {
				continue
			}
			number, _ := strconv.Atoi(match[1])
			if number < 1 || number > len(lines) {
				t.Errorf("%s:%d: line %d is outside the fixture", expectedPath, index+1, number)
				continue
			}
			value := unescape(match[5])
			if count := strings.Count(lines[number-1], value); count != 1 {
				t.Errorf("%s:%d: %q occurs %d times on line %d; write the column by hand",
					expectedPath, index+1, value, count, number)
				continue
			}
			written[index] = fmt.Sprintf("%d:%d %s %s %q",
				number, strings.Index(lines[number-1], value)+1, match[3], match[4], value)
		}

		if err := os.WriteFile(expectedPath, []byte(strings.Join(written, "\n")), 0o644); err != nil {
			t.Fatalf("write %s: %v", expectedPath, err)
		}
	}
}

func readBaseline(t *testing.T) measurement {
	t.Helper()
	content, err := os.ReadFile(baselinePath)
	if err != nil {
		if os.IsNotExist(err) && *updateFixtures {
			return measurement{}
		}
		t.Fatalf("read baseline: %v", err)
	}
	var baseline measurement
	if err := json.Unmarshal(content, &baseline); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	return baseline
}

func writeBaseline(t *testing.T, result measurement) {
	t.Helper()
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("encode baseline: %v", err)
	}
	if err := os.WriteFile(baselinePath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
}
