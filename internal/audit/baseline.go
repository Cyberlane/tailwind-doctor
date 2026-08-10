package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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
const BaselineVersion = 2

// SuppressedFinding identifies debt that has been accepted. Reason is for the
// humans reading the file; nothing in the tool interprets it.
type SuppressedFinding struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	Rule        string `json:"rule"`
	File        string `json:"file"`
	Class       string `json:"class"`
	Reason      string `json:"reason,omitempty"`
}

type Baseline struct {
	Version    int                 `json:"version"`
	Note       string              `json:"note,omitempty"`
	Suppressed []SuppressedFinding `json:"suppressed"`

	index map[string]bool
}

func (suppressed SuppressedFinding) key() string {
	if suppressed.Fingerprint != "" {
		return "fingerprint\x00" + suppressed.Fingerprint
	}
	return suppressed.Rule + "\x00" + suppressed.File + "\x00" + suppressed.Class
}

func findingFingerprint(rule, file, class string) string {
	digest := sha256.Sum256([]byte(rule + "\x00" + file + "\x00" + class))
	return fmt.Sprintf("sha256:%x", digest)
}

func (baseline *Baseline) suppresses(finding Finding) bool {
	if baseline == nil || baseline.index == nil {
		return false
	}
	fingerprint := findingFingerprint(finding.Rule, finding.File, finding.Class)
	if baseline.index[SuppressedFinding{Fingerprint: fingerprint}.key()] {
		return true
	}
	// Version 1 entries had no explicit fingerprint. Keeping this lookup lets a
	// project upgrade the binary before regenerating its baseline.
	return baseline.index[SuppressedFinding{Rule: finding.Rule, File: finding.File, Class: finding.Class}.key()]
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
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&baseline); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if baseline.Version != 1 && baseline.Version != BaselineVersion {
		return nil, fmt.Errorf("%s: version %d is not supported; this build reads versions 1 and %d",
			name, baseline.Version, BaselineVersion)
	}

	baseline.index = make(map[string]bool, len(baseline.Suppressed))
	for _, entry := range baseline.Suppressed {
		if baseline.Version == BaselineVersion && entry.Fingerprint == "" {
			return nil, fmt.Errorf("%s: version %d entry for %s is missing fingerprint",
				name, BaselineVersion, entry.File)
		}
		if baseline.Version == BaselineVersion {
			expected := findingFingerprint(entry.Rule, entry.File, entry.Class)
			if entry.Fingerprint != expected {
				return nil, fmt.Errorf("%s: fingerprint for %s does not match its rule, file, and class evidence",
					name, entry.File)
			}
		}
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
		entry := SuppressedFinding{
			Fingerprint: findingFingerprint(finding.Rule, finding.File, finding.Class),
			Rule:        finding.Rule, File: finding.File, Class: finding.Class,
		}
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
		Note: "Debt accepted at the time this file was written. Stable fingerprints are " +
			"derived from rule, file, and class evidence rather than source position. Remove " +
			"an entry to start failing on it again.",
		Suppressed: entries,
	}
}

func WriteBaseline(path string, baseline Baseline) error {
	encoded, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".tw-doctor-baseline-*")
	if err != nil {
		return fmt.Errorf("create temporary baseline for %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = temporary.Close() }()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set baseline permissions for %s: %w", path, err)
	}
	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write temporary baseline for %s: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary baseline for %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary baseline for %s: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("unexpected data after baseline document")
}
