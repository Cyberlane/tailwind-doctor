package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

var sourceExtensions = map[string]bool{
	".astro": true, ".html": true, ".jsx": true, ".tsx": true,
	".vue": true, ".svelte": true, ".css": true,
}

type Finding struct {
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
	File     string   `json:"file"`
	Class    string   `json:"class"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Severity Severity `json:"severity"`
}

type Report struct {
	Score    int       `json:"score"`
	Files    int       `json:"files"`
	Findings []Finding `json:"findings"`
	// Suppressed counts findings that matched the baseline. Reporting the count
	// keeps accepted debt visible rather than letting it vanish.
	Suppressed int `json:"suppressed"`
}

// MaximumScore is the score a project with no findings receives, and therefore
// the highest threshold a caller can meaningfully gate on.
const MaximumScore = 100

// Run analyses a directory tree. Configuration and the baseline are read from
// the root, so a caller needs nothing but a path.
func Run(root string) (Report, error) {
	config, err := LoadConfig(root)
	if err != nil {
		return Report{}, err
	}
	baseline, err := LoadBaseline(root, config.BaselinePath)
	if err != nil {
		return Report{}, err
	}
	return RunWithConfig(root, config, baseline)
}

// RunWithConfig analyses a directory tree with configuration supplied by the
// caller, which is what lets the baseline be regenerated from a run that ignores
// the existing one.
func RunWithConfig(root string, config Config, baseline *Baseline) (Report, error) {
	// Findings starts non-nil so a clean project serializes as [] rather than
	// null, which every JSON consumer would otherwise have to special-case.
	report := Report{Score: MaximumScore, Findings: []Finding{}}
	ignores := newIgnoreRules(config.IgnorePaths)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist", "build", ".next", "vendor":
				return filepath.SkipDir
			}
			if config.RespectGitignore {
				// WalkDir visits a directory before its contents, so patterns are
				// recorded in time to apply to everything they govern.
				if content, err := os.ReadFile(filepath.Join(path, ".gitignore")); err == nil {
					ignores.addGitignore(relative, string(content))
				}
			}
			if relative != "." && ignores.ignores(relative, true) {
				return filepath.SkipDir
			}
			return nil
		}

		if !sourceExtensions[filepath.Ext(path)] || ignores.ignores(relative, false) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		report.Files++
		for _, list := range Extract(path, string(content)) {
			// An unresolved site names an expression, not a set of utilities.
			// Linting it would report classes the source never contained.
			if !list.Resolved {
				continue
			}
			for _, finding := range inspect(relative, list, config.Syntax) {
				finding.Severity = config.severityFor(finding.Rule)
				if finding.Severity == SeverityOff {
					continue
				}
				if finding.Rule == "no-arbitrary-value" && config.AllowedArbitrary[finding.Class] {
					continue
				}
				if baseline.suppresses(finding) {
					report.Suppressed++
					continue
				}
				report.Findings = append(report.Findings, finding)
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	sortFindings(report.Findings)

	// Only findings the project treats as errors move the score. A warning is
	// reported so it can be acted on, without changing what a build gates on.
	scored := 0
	for _, finding := range report.Findings {
		if finding.Severity == SeverityError {
			scored++
		}
	}
	report.Score -= scored * 2
	if report.Score < 0 {
		report.Score = 0
	}
	return report, nil
}

// utilityToken is one utility and where it sits in the source. Positions are
// only meaningful when the class list was taken verbatim from the file; a list
// assembled from the literal parts of an interpolated value has no single span,
// so every utility in it reports the position of the list itself.
type utilityToken struct {
	text   string
	line   int
	column int
}

func splitUtilities(list ClassList) []utilityToken {
	tokens := make([]utilityToken, 0, 8)
	position := &positionTracker{line: list.Line, column: list.Column}
	start := -1

	for index := 0; index <= len(list.Value); index++ {
		atEnd := index == len(list.Value)
		if !atEnd && !isSpace(list.Value[index]) {
			if start < 0 {
				start = index
				if list.Verbatim {
					tokens = append(tokens, utilityToken{line: position.line, column: position.column})
				} else {
					tokens = append(tokens, utilityToken{line: list.Line, column: list.Column})
				}
			}
		} else if start >= 0 {
			tokens[len(tokens)-1].text = list.Value[start:index]
			start = -1
		}
		if !atEnd {
			position.advance(list.Value[index])
		}
	}
	return tokens
}

func inspect(file string, list ClassList, syntax UtilitySyntax) []Finding {
	findings := make([]Finding, 0)
	seen := make(map[string]string)
	variants := 0

	for _, token := range splitUtilities(list) {
		parsed := parseUtility(token.text, syntax)

		if parsed.hasArbitraryValue() {
			findings = append(findings, Finding{
				Rule: "no-arbitrary-value", File: file, Class: token.text,
				Line: token.line, Column: token.column,
				Message: "Avoid arbitrary values; prefer a named design token.",
			})
		}

		if len(parsed.Variants) > 0 {
			variants++
		}

		group := utilityGroup(parsed.Base)
		if group == "" {
			continue
		}
		key := parsed.variantKey() + "|" + group
		if previous, ok := seen[key]; ok {
			findings = append(findings, Finding{
				Rule: "no-conflicting-utilities", File: file, Class: list.Value,
				Line: token.line, Column: token.column,
				Message: fmt.Sprintf("%s conflicts with %s in the same variant.", previous, token.text),
			})
			continue
		}
		seen[key] = token.text
	}

	if variants >= 5 {
		findings = append(findings, Finding{
			Rule: "responsive-bloat", File: file, Class: list.Value,
			Line: list.Line, Column: list.Column,
			Message: "Five or more variant utilities make this class list difficult to maintain.",
		})
	}
	return findings
}

// sortFindings gives the report a total order, so that identical input produces
// byte-identical output whatever order the filesystem hands files over in.
func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		switch {
		case left.File != right.File:
			return left.File < right.File
		case left.Line != right.Line:
			return left.Line < right.Line
		case left.Column != right.Column:
			return left.Column < right.Column
		case left.Rule != right.Rule:
			return left.Rule < right.Rule
		}
		return left.Message < right.Message
	})
}

func WriteJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteHuman(writer io.Writer, report Report) {
	fmt.Fprintf(writer, "Tailwind Doctor: %d/100\n", report.Score)
	fmt.Fprintf(writer, "Scanned %d source files\n", report.Files)
	if len(report.Findings) == 0 {
		fmt.Fprintln(writer, "No findings. Your class lists look healthy.")
		return
	}
	fmt.Fprintf(writer, "\n%d finding(s):\n", len(report.Findings))
	for _, finding := range report.Findings {
		fmt.Fprintf(writer, "- [%s] %s:%d:%d: %s\n", finding.Rule, finding.File, finding.Line, finding.Column, finding.Message)
	}
}
