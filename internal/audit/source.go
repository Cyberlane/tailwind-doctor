package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/plugins"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

// SourceAnalyzer holds the immutable project context needed to analyze unsaved
// editor buffers. AnalyzeSource touches no source file and executes no project
// code; callers can reuse one instance across document changes.
type SourceAnalyzer struct {
	config   Config
	baseline *Baseline
	layout   tailwind.Layout
	themes   []resolvedTheme
	ignores  *ignoreRules
}

func NewSourceAnalyzer(root string) (*SourceAnalyzer, error) {
	root = filepath.Clean(root)
	config, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}
	baseline, err := LoadBaseline(root, config.BaselinePath)
	if err != nil {
		return nil, err
	}
	ignores := newIgnoreRules(config.IgnorePaths)
	if config.RespectGitignore {
		if err := loadGitignoreFiles(root, ignores); err != nil {
			return nil, err
		}
	}
	rootDirectory, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open analysis root: %w", err)
	}
	defer rootDirectory.Close()
	layout, err := tailwind.DiscoverFiltered(rootDirectory.FS(), ignores.ignores)
	if err != nil {
		return nil, fmt.Errorf("discover Tailwind packages: %w", err)
	}
	themes, _, err := loadThemes(rootDirectory.FS(), layout)
	if err != nil {
		return nil, err
	}
	return &SourceAnalyzer{
		config: config, baseline: baseline, layout: layout, themes: themes, ignores: ignores,
	}, nil
}

// AnalyzeSource returns diagnostics for one current buffer. The file path is
// relative to the workspace root and is used only for package scope and report
// locations; content is supplied by the editor.
func (analyzer *SourceAnalyzer) AnalyzeSource(file, content string) ([]Finding, error) {
	file = filepath.ToSlash(filepath.Clean(filepath.FromSlash(file)))
	if file == "." || filepath.IsAbs(file) || file == ".." || strings.HasPrefix(file, "../") {
		return nil, fmt.Errorf("source path must stay inside analysis root: %s", file)
	}
	if analyzer.ignores.ignores(file, false) || !sourceExtensions[filepath.Ext(file)] {
		return []Finding{}, nil
	}

	syntax, resolvedTheme := analysisContextForFile(file, analyzer.layout, analyzer.themes, analyzer.config)
	var inventory *tokens.Inventory
	allowSuggestions := false
	trustedContrastTheme := false
	if resolvedTheme != nil {
		inventory = resolvedTheme.theme.Inventory
		allowSuggestions = !resolvedTheme.theme.Degraded
		trustedContrastTheme = !resolvedTheme.theme.Degraded && plugins.Complete(resolvedTheme.theme.PluginCoverage)
	}

	report := Report{Findings: []Finding{}}
	suppressed := []Finding{}
	for _, list := range Extract(file, content) {
		if !list.Resolved {
			continue
		}
		findings, _ := inspectWithInventory(file, list, syntax, inventory, allowSuggestions)
		findings = append(findings, inspectContrast(file, list, syntax, inventory, trustedContrastTheme).findings...)
		for _, finding := range findings {
			recordFinding(&report, &suppressed, finding, analyzer.config, analyzer.baseline)
		}
	}
	sortFindings(report.Findings)
	for index := range report.Findings {
		report.Findings[index].Scored = analyzer.config.scores(report.Findings[index])
	}
	return report.Findings, nil
}
