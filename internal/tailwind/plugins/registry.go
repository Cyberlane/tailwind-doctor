// Package plugins contains deterministic, reviewed descriptions of Tailwind
// plugin utility surfaces. It never loads plugin code or reads node_modules.
package plugins

import (
	"sort"
	"strconv"
	"strings"
)

// Support is how confidently the registry understands one configured plugin.
type Support string

const (
	SupportComplete       Support = "complete"
	SupportPartial        Support = "partial"
	SupportUnknown        Support = "unknown"
	SupportMissingVersion Support = "missing-version"
	SupportOutOfRange     Support = "out-of-range"
)

// Pattern identifies plugin utilities without evaluating the plugin.
type Pattern struct {
	Exact  string
	Prefix string
}

// Coverage is the resolved registry evidence for one configured plugin.
type Coverage struct {
	Specifier    string
	VersionRange string
	Support      Support
	Complete     bool
	Reason       string
	Patterns     []Pattern
}

type versionBand struct {
	major int
	minor int
}

type registryEntry struct {
	specifier string
	aliases   []string
	bands     []versionBand
	complete  bool
	patterns  []Pattern
}

var registry = []registryEntry{
	{
		specifier: "@tailwindcss/typography",
		bands:     []versionBand{{major: 0, minor: 5}},
		complete:  true,
		patterns: []Pattern{
			{Exact: "prose"}, {Prefix: "prose-"},
		},
	},
	{
		specifier: "@tailwindcss/forms",
		bands:     []versionBand{{major: 0, minor: 5}},
		complete:  true,
		patterns: []Pattern{
			{Exact: "form-checkbox"}, {Exact: "form-input"}, {Exact: "form-multiselect"},
			{Exact: "form-radio"}, {Exact: "form-select"}, {Exact: "form-textarea"},
		},
	},
	{
		specifier: "@tailwindcss/aspect-ratio",
		bands:     []versionBand{{major: 0, minor: 4}},
		complete:  true,
		patterns: []Pattern{
			{Exact: "aspect-none"}, {Prefix: "aspect-h-"}, {Prefix: "aspect-w-"},
		},
	},
	{
		specifier: "daisyui",
		aliases:   []string{"daisyui/plugin"},
		bands:     []versionBand{{major: 4}, {major: 5}},
		complete:  false,
	},
	{
		specifier: "flowbite",
		aliases:   []string{"flowbite/plugin"},
		bands:     []versionBand{{major: 3}, {major: 4}},
		complete:  false,
	},
}

// Resolve returns one sorted coverage record per distinct configured plugin.
func Resolve(specifiers []string, versions map[string]string) []Coverage {
	unique := map[string]bool{}
	for _, specifier := range specifiers {
		if specifier != "" {
			unique[specifier] = true
		}
	}
	ordered := make([]string, 0, len(unique))
	for specifier := range unique {
		ordered = append(ordered, specifier)
	}
	sort.Strings(ordered)

	coverage := make([]Coverage, 0, len(ordered))
	for _, specifier := range ordered {
		entry, found := findEntry(specifier)
		if !found {
			coverage = append(coverage, Coverage{
				Specifier: specifier, Support: SupportUnknown,
				Reason: "plugin is not in the curated registry",
			})
			continue
		}
		versionRange := versions[entry.specifier]
		if versionRange == "" {
			versionRange = versions[specifier]
		}
		resolved := Coverage{
			Specifier: specifier, VersionRange: versionRange,
			Patterns: append([]Pattern(nil), entry.patterns...),
		}
		if versionRange == "" {
			resolved.Support = SupportMissingVersion
			resolved.Reason = "plugin version is not declared in the package manifest"
		} else if !supportsRange(entry.bands, versionRange) {
			resolved.Support = SupportOutOfRange
			resolved.Reason = "plugin version is outside the reviewed registry ranges"
		} else if entry.complete {
			resolved.Support = SupportComplete
			resolved.Complete = true
		} else {
			resolved.Support = SupportPartial
			resolved.Reason = "registry coverage for this plugin is intentionally partial"
		}
		coverage = append(coverage, resolved)
	}
	return coverage
}

// Matches reports whether a utility is inside a reviewed lexical surface.
func (coverage Coverage) Matches(utility string) bool {
	for _, pattern := range coverage.Patterns {
		if pattern.Exact != "" && utility == pattern.Exact {
			return true
		}
		if pattern.Prefix != "" && strings.HasPrefix(utility, pattern.Prefix) {
			return true
		}
	}
	return false
}

// Complete reports whether every configured plugin has complete reviewed
// coverage. An empty plugin list is complete.
func Complete(coverage []Coverage) bool {
	for _, plugin := range coverage {
		if !plugin.Complete {
			return false
		}
	}
	return true
}

func findEntry(specifier string) (registryEntry, bool) {
	for _, entry := range registry {
		if specifier == entry.specifier {
			return entry, true
		}
		for _, alias := range entry.aliases {
			if specifier == alias {
				return entry, true
			}
		}
	}
	return registryEntry{}, false
}

func supportsRange(bands []versionBand, versionRange string) bool {
	major, minor, found := firstVersion(versionRange)
	if !found {
		return false
	}
	for _, band := range bands {
		if major == band.major && (band.major != 0 || minor == band.minor) {
			return true
		}
	}
	return false
}

func firstVersion(versionRange string) (int, int, bool) {
	trimmed := strings.TrimLeft(strings.TrimSpace(versionRange), "^~><=v ")
	parts := strings.SplitN(trimmed, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minorDigits := 0
	for minorDigits < len(parts[1]) && parts[1][minorDigits] >= '0' && parts[1][minorDigits] <= '9' {
		minorDigits++
	}
	if minorDigits == 0 {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1][:minorDigits])
	return major, minor, err == nil
}
