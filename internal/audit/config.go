package audit

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
)

// ConfigFileName is the optional per-project configuration file, read from the
// root being analysed. A project without one is analysed with the defaults.
const ConfigFileName = "twdoctor.toml"

// Severity decides what a rule does when it matches. A warning is reported but
// does not move the score, which is how a rule can be useful before it is
// trusted enough to gate a build on.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityOff   Severity = "off"
)

// Config is the resolved project configuration.
type Config struct {
	Severities          map[string]Severity
	IgnorePaths         []string
	RespectGitignore    bool
	AllowedArbitrary    map[string]bool
	Syntax              tailwind.UtilitySyntax
	BaselinePath        string
	prefixConfigured    bool
	separatorConfigured bool
	// MinConfidence is the lowest confidence a finding may carry and still move
	// the score. Findings below it are reported and tagged, never hidden.
	MinConfidence Confidence
}

func defaultConfig() Config {
	return Config{
		Severities:       map[string]Severity{},
		RespectGitignore: true,
		AllowedArbitrary: map[string]bool{},
		Syntax:           tailwind.DefaultUtilitySyntax(),
		BaselinePath:     BaselineFileName,
		MinConfidence:    ConfidenceHigh,
	}
}

// severityFor resolves what a rule does in this project. Configuration wins;
// otherwise a rule that has not yet had its release of warning is off, and every
// other rule takes its registered default. See docs/rule-stability.md.
func (config Config) severityFor(rule string) Severity {
	if severity, ok := config.Severities[rule]; ok {
		return severity
	}
	if definition, found := lookupRule(rule); found {
		if !definition.DefaultOn {
			return SeverityOff
		}
		return definition.DefaultSeverity
	}
	return SeverityError
}

// LoadConfig reads twdoctor.toml from root. A missing file is not an error; a
// malformed one is, because silently analysing with the wrong settings is worse
// than refusing to start.
func LoadConfig(root string) (Config, error) {
	config := defaultConfig()

	content, err := os.ReadFile(filepath.Join(root, ConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, fmt.Errorf("read %s: %w", ConfigFileName, err)
	}

	document, err := parseTOML(string(content))
	if err != nil {
		return config, fmt.Errorf("%s: %w", ConfigFileName, err)
	}
	if err := validateConfigDocument(document); err != nil {
		return config, fmt.Errorf("%s: %w", ConfigFileName, err)
	}

	ruleNames := make([]string, 0, len(document["rules"]))
	for rule := range document["rules"] {
		ruleNames = append(ruleNames, rule)
	}
	sort.Strings(ruleNames)
	for _, rule := range ruleNames {
		raw := document["rules"][rule]
		if _, found := lookupRule(rule); !found {
			return config, fmt.Errorf("%s: rules.%s is not a known rule", ConfigFileName, rule)
		}
		text, ok := raw.(string)
		if !ok {
			return config, fmt.Errorf("%s: rules.%s must be a string", ConfigFileName, rule)
		}
		severity := Severity(text)
		switch severity {
		case SeverityError, SeverityWarn, SeverityOff:
			config.Severities[rule] = severity
		default:
			return config, fmt.Errorf("%s: rules.%s is %q; expected error, warn, or off", ConfigFileName, rule, text)
		}
	}

	paths := document["paths"]
	ignore, err := paths.listValue("ignore")
	if err != nil {
		return config, fmt.Errorf("%s: paths.%w", ConfigFileName, err)
	}
	config.IgnorePaths = ignore
	for _, pattern := range config.IgnorePaths {
		if err := validatePathPattern(pattern); err != nil {
			return config, fmt.Errorf("%s: paths.ignore contains %q: %w", ConfigFileName, pattern, err)
		}
	}
	if respect, present, err := paths.boolValue("respect-gitignore"); err != nil {
		return config, fmt.Errorf("%s: paths.%w", ConfigFileName, err)
	} else if present {
		config.RespectGitignore = respect
	}

	allowed, err := document["arbitrary-values"].listValue("allow")
	if err != nil {
		return config, fmt.Errorf("%s: arbitrary-values.%w", ConfigFileName, err)
	}
	for _, class := range allowed {
		config.AllowedArbitrary[class] = true
	}

	tailwind := document["tailwind"]
	if prefix, present, err := tailwind.stringValue("prefix"); err != nil {
		return config, fmt.Errorf("%s: tailwind.%w", ConfigFileName, err)
	} else if present {
		config.Syntax.Prefix = prefix
		config.Syntax.PrefixIsVariant = false
		config.prefixConfigured = true
	}
	if separator, present, err := tailwind.stringValue("separator"); err != nil {
		return config, fmt.Errorf("%s: tailwind.%w", ConfigFileName, err)
	} else if present {
		if separator == "" {
			return config, fmt.Errorf("%s: tailwind.separator must not be empty", ConfigFileName)
		}
		config.Syntax.Separator = separator
		config.separatorConfigured = true
	}

	if minimum, present, err := document["score"].stringValue("min-confidence"); err != nil {
		return config, fmt.Errorf("%s: score.%w", ConfigFileName, err)
	} else if present {
		confidence := Confidence(minimum)
		if confidenceRank(confidence) == 0 {
			return config, fmt.Errorf("%s: score.min-confidence is %q; expected high, medium, or low",
				ConfigFileName, minimum)
		}
		config.MinConfidence = confidence
	}

	if baseline, present, err := document["baseline"].stringValue("path"); err != nil {
		return config, fmt.Errorf("%s: baseline.%w", ConfigFileName, err)
	} else if present {
		if err := validateRelativeConfigPath(baseline); err != nil {
			return config, fmt.Errorf("%s: baseline.path %q: %w", ConfigFileName, baseline, err)
		}
		config.BaselinePath = baseline
	}

	return config, nil
}

func validateConfigDocument(document map[string]tomlTable) error {
	allowed := map[string]map[string]bool{
		"":                 {},
		"rules":            nil,
		"paths":            {"ignore": true, "respect-gitignore": true},
		"arbitrary-values": {"allow": true},
		"tailwind":         {"prefix": true, "separator": true},
		"score":            {"min-confidence": true},
		"baseline":         {"path": true},
	}
	tables := make([]string, 0, len(document))
	for table := range document {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		keys, known := allowed[table]
		if !known {
			return fmt.Errorf("unknown table [%s]", table)
		}
		if table == "rules" {
			continue
		}
		tableKeys := make([]string, 0, len(document[table]))
		for key := range document[table] {
			tableKeys = append(tableKeys, key)
		}
		sort.Strings(tableKeys)
		for _, key := range tableKeys {
			if !keys[key] {
				qualified := key
				if table != "" {
					qualified = table + "." + key
				}
				return fmt.Errorf("unknown setting %s", qualified)
			}
		}
	}
	return nil
}

func validatePathPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern must not be empty")
	}
	if strings.Contains(pattern, "\\") {
		return fmt.Errorf("use slash-separated paths")
	}
	if path.IsAbs(pattern) {
		return fmt.Errorf("pattern must be relative")
	}
	if pattern == ".." || strings.HasPrefix(pattern, "../") {
		return fmt.Errorf("pattern must stay inside the analysis root")
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return fmt.Errorf("invalid glob: %w", err)
		}
	}
	return nil
}

func validateRelativeConfigPath(value string) error {
	if value == "" {
		return fmt.Errorf("path must not be empty")
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("path must be relative to the analysis root")
	}
	clean := filepath.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must stay inside the analysis root")
	}
	return nil
}

// matchPath reports whether a slash-separated path matches a glob. A ** matches
// any number of path segments, including none; a * matches within one segment.
// filepath.Match has no ** at all, and a tool whose main job is walking a
// repository needs it: "dist/**" is how everyone writes "that whole directory".
func matchPath(pattern, target string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(target, "/"))
}

func matchSegments(pattern, target []string) bool {
	switch {
	case len(pattern) == 0:
		return len(target) == 0
	case pattern[0] == "**":
		for index := 0; index <= len(target); index++ {
			if matchSegments(pattern[1:], target[index:]) {
				return true
			}
		}
		return false
	case len(target) == 0:
		return false
	}
	matched, err := path.Match(pattern[0], target[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(pattern[1:], target[1:])
}

// ignoreRules holds the patterns that exclude a path from analysis: those
// configured in twdoctor.toml and those found in .gitignore files. Git applies a
// .gitignore to its own directory and below, so patterns are kept per directory
// and a path is tested against every one above it.
type ignoreRules struct {
	configured []string
	perDir     map[string][]gitignorePattern
	dirs       []string
}

type gitignorePattern struct {
	glob          string
	negated       bool
	directoryOnly bool
	anchored      bool
	segmentAny    bool
}

func newIgnoreRules(configured []string) *ignoreRules {
	return &ignoreRules{configured: configured, perDir: map[string][]gitignorePattern{}}
}

// addGitignore records the patterns of a .gitignore found in directory, which is
// a path relative to the analysis root.
func (rules *ignoreRules) addGitignore(directory, content string) {
	patterns := make([]gitignorePattern, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern := gitignorePattern{}
		if trimmed, found := strings.CutPrefix(line, "!"); found {
			pattern.negated, line = true, trimmed
		}
		if trimmed, found := strings.CutSuffix(line, "/"); found {
			pattern.directoryOnly, line = true, trimmed
		}
		if trimmed, found := strings.CutPrefix(line, "/"); found {
			pattern.anchored, line = true, trimmed
		}
		if strings.Contains(line, "/") {
			pattern.anchored = true
		}
		pattern.glob = line
		patterns = append(patterns, pattern)
	}
	if len(patterns) == 0 {
		return
	}
	if _, seen := rules.perDir[directory]; !seen {
		rules.dirs = append(rules.dirs, directory)
	}
	rules.perDir[directory] = patterns
}

// ignores reports whether a path relative to the analysis root is excluded.
func (rules *ignoreRules) ignores(relative string, isDir bool) bool {
	for _, pattern := range rules.configured {
		if matchPath(pattern, relative) {
			return true
		}
	}

	ignored := false
	for _, directory := range rules.dirs {
		scoped, inside := scopeTo(directory, relative)
		if !inside {
			continue
		}
		for _, pattern := range rules.perDir[directory] {
			if pattern.directoryOnly && !isDir {
				continue
			}
			if !matchGitignore(pattern, scoped) {
				continue
			}
			ignored = !pattern.negated
		}
	}
	return ignored
}

func scopeTo(directory, relative string) (string, bool) {
	if directory == "." || directory == "" {
		return relative, true
	}
	if !strings.HasPrefix(relative, directory+"/") {
		return "", false
	}
	return strings.TrimPrefix(relative, directory+"/"), true
}

// matchGitignore applies one pattern. An unanchored pattern matches at any
// depth, which is what makes "node_modules" in a root .gitignore exclude a
// nested one too.
func matchGitignore(pattern gitignorePattern, relative string) bool {
	if pattern.anchored {
		return matchPath(pattern.glob, relative)
	}
	segments := strings.Split(relative, "/")
	for index := range segments {
		if matchPath(pattern.glob, strings.Join(segments[index:], "/")) {
			return true
		}
		if matched, err := path.Match(pattern.glob, segments[index]); err == nil && matched {
			return true
		}
	}
	return false
}
