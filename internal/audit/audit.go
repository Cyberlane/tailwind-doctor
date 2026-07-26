package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var classAttribute = regexp.MustCompile(`(?s)(?:class|className)\s*=\s*["']([^"']+)["']`)

var sourceExtensions = map[string]bool{
	".astro": true, ".html": true, ".jsx": true, ".tsx": true,
	".vue": true, ".svelte": true,
}

type Finding struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	File    string `json:"file"`
	Class   string `json:"class"`
}

type Report struct {
	Score    int       `json:"score"`
	Files    int       `json:"files"`
	Findings []Finding `json:"findings"`
}

// MaximumScore is the score a project with no findings receives, and therefore
// the highest threshold a caller can meaningfully gate on.
const MaximumScore = 100

func Run(root string) (Report, error) {
	// Findings starts non-nil so a clean project serializes as [] rather than
	// null, which every JSON consumer would otherwise have to special-case.
	report := Report{Score: MaximumScore, Findings: []Finding{}}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist", "build", ".next", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExtensions[filepath.Ext(path)] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		report.Files++
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, match := range classAttribute.FindAllStringSubmatch(string(content), -1) {
			report.Findings = append(report.Findings, inspect(relative, match[1])...)
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].File == report.Findings[j].File {
			return report.Findings[i].Rule < report.Findings[j].Rule
		}
		return report.Findings[i].File < report.Findings[j].File
	})
	report.Score -= len(report.Findings) * 2
	if report.Score < 0 {
		report.Score = 0
	}
	return report, nil
}

func inspect(file, classes string) []Finding {
	utilities := strings.Fields(classes)
	findings := make([]Finding, 0)
	seen := make(map[string]string)
	responsive := 0

	for _, utility := range utilities {
		if strings.Contains(utility, "[") {
			findings = append(findings, Finding{
				Rule: "no-arbitrary-value", File: file, Class: utility,
				Message: "Avoid arbitrary values; prefer a named design token.",
			})
		}

		base := utility[strings.LastIndex(utility, ":")+1:]
		if strings.Contains(utility, ":") {
			responsive++
		}
		group := utilityGroup(base)
		if group == "" {
			continue
		}
		key := utility[:len(utility)-len(base)] + group
		if previous, ok := seen[key]; ok {
			findings = append(findings, Finding{
				Rule: "no-conflicting-utilities", File: file, Class: classes,
				Message: fmt.Sprintf("%s conflicts with %s in the same variant.", previous, utility),
			})
			continue
		}
		seen[key] = utility
	}

	if responsive >= 5 {
		findings = append(findings, Finding{
			Rule: "responsive-bloat", File: file, Class: classes,
			Message: "Five or more variant utilities make this class list difficult to maintain.",
		})
	}
	return findings
}

func utilityGroup(utility string) string {
	for _, prefix := range []string{"p-", "px-", "py-", "pt-", "pr-", "pb-", "pl-", "m-", "mx-", "my-", "mt-", "mr-", "mb-", "ml-", "text-", "bg-", "border-"} {
		if strings.HasPrefix(utility, prefix) {
			return prefix
		}
	}
	return ""
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
		fmt.Fprintf(writer, "- [%s] %s: %s\n", finding.Rule, finding.File, finding.Message)
	}
}
