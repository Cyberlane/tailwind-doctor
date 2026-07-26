package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// A project adopting this tool on an existing codebase starts with debt it did
// not just write. Without a way to record that, the only usable threshold is one
// low enough to pass today, which gates nothing. A baseline records the findings
// already present so that a build fails on new debt and not on old.
//
// The format is deliberately positional-free. A finding is keyed by rule, file,
// and class list, never by line and column, because reformatting a file or
// adding an import above a component would otherwise resurrect every suppressed
// finding in it as new debt.

// BaselineFileName is the default suppression file, read from the analysis root.
const BaselineFileName = "twdoctor-baseline.json"

// BaselineVersion is the format version. A reader that does not recognise a
// version refuses the file rather than guessing at its meaning.
const BaselineVersion = 1

// SuppressedFinding identifies debt that has been accepted. Reason is for the
// humans reading the file; nothing in the tool interprets it.
type SuppressedFinding struct {
	Rule   string `json:"rule"`
	File   string `json:"file"`
	Class  string `json:"class"`
	Reason string `json:"reason,omitempty"`
}

type Baseline struct {
	Version    int                 `json:"version"`
	Note       string              `json:"note,omitempty"`
	Suppressed []SuppressedFinding `json:"suppressed"`

	index map[string]bool
}

func (suppressed SuppressedFinding) key() string {
	return suppressed.Rule + "\x00" + suppressed.File + "\x00" + suppressed.Class
}

func (baseline *Baseline) suppresses(finding Finding) bool {
	if baseline == nil || baseline.index == nil {
		return false
	}
	return baseline.index[SuppressedFinding{
		Rule: finding.Rule, File: finding.File, Class: finding.Class,
	}.key()]
}

// LoadBaseline reads the suppression file from root. A missing file is not an
// error: most projects do not need one.
func LoadBaseline(root, name string) (*Baseline, error) {
	if name == "" {
		name = BaselineFileName
	}
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", name, err)
	}

	var baseline Baseline
	if err := json.Unmarshal(content, &baseline); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if baseline.Version != BaselineVersion {
		return nil, fmt.Errorf("%s: version %d is not supported; this build reads version %d",
			name, baseline.Version, BaselineVersion)
	}

	baseline.index = make(map[string]bool, len(baseline.Suppressed))
	for _, entry := range baseline.Suppressed {
		baseline.index[entry.key()] = true
	}
	return &baseline, nil
}

// NewBaseline builds a suppression file covering every finding in a report, so
// that a project can record its current debt and gate on anything added after.
func NewBaseline(report Report) Baseline {
	entries := make([]SuppressedFinding, 0, len(report.Findings))
	seen := map[string]bool{}
	for _, finding := range report.Findings {
		entry := SuppressedFinding{Rule: finding.Rule, File: finding.File, Class: finding.Class}
		if seen[entry.key()] {
			continue
		}
		seen[entry.key()] = true
		entries = append(entries, entry)
	}

	// Sorted so that regenerating the file produces a reviewable diff rather
	// than a reshuffle.
	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		switch {
		case left.File != right.File:
			return left.File < right.File
		case left.Rule != right.Rule:
			return left.Rule < right.Rule
		}
		return left.Class < right.Class
	})

	return Baseline{
		Version: BaselineVersion,
		Note: "Debt accepted at the time this file was written. Findings are keyed by " +
			"rule, file, and class list, never by position, so moving code does not " +
			"resurrect them. Remove an entry to start failing on it again.",
		Suppressed: entries,
	}
}

func WriteBaseline(path string, baseline Baseline) error {
	encoded, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
