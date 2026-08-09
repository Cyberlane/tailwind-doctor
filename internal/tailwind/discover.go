package tailwind

import (
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/cssdecl"
)

// Package is one independently scoped Tailwind theme.
type Package struct {
	Dir        string
	Version    Version
	ConfigFile string
	Entry      string
	Evidence   []Evidence
}

// Layout is the deterministic package layout of a project.
type Layout struct {
	Packages []Package
}

type packageCandidate struct {
	configs []string
	entries []string
}

var skippedDirectoryNames = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	".next": true, "vendor": true,
}

// Discover finds direct v3 configs and v4 CSS entries, then establishes one
// package root for each independent theme.
func Discover(fsys fs.FS) (Layout, error) {
	candidates := map[string]*packageCandidate{}
	err := fs.WalkDir(fsys, ".", func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if file != "." && skippedDirectoryNames[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}

		if isTailwindConfig(entry.Name()) {
			dir := cleanDir(path.Dir(file))
			candidate := ensureCandidate(candidates, dir)
			candidate.configs = append(candidate.configs, cleanPath(file))
			return nil
		}
		if path.Ext(entry.Name()) != ".css" {
			return nil
		}
		content, readErr := fs.ReadFile(fsys, file)
		if readErr != nil {
			return nil
		}
		sheet, parseErr := cssdecl.Parse(string(content))
		if parseErr != nil || !hasV4EntrySignal(sheet.Nodes) {
			return nil
		}
		dir := nearestManifestDir(fsys, path.Dir(file))
		candidate := ensureCandidate(candidates, cleanDir(dir))
		candidate.entries = append(candidate.entries, cleanPath(file))
		return nil
	})
	if err != nil {
		return Layout{}, err
	}

	directories := make([]string, 0, len(candidates))
	for dir := range candidates {
		directories = append(directories, dir)
	}
	sort.Strings(directories)

	packages := make([]Package, 0, len(directories))
	for _, dir := range directories {
		candidate := candidates[dir]
		sort.Strings(candidate.configs)
		sort.Strings(candidate.entries)
		detection, detectErr := Detect(fsys, dir)
		if detectErr != nil {
			return Layout{}, detectErr
		}
		pkg := Package{Dir: dir, Version: detection.Version, Evidence: detection.Evidence}
		if len(candidate.configs) > 0 {
			pkg.ConfigFile = preferredConfig(candidate.configs)
		}
		if len(candidate.entries) > 0 {
			pkg.Entry = candidate.entries[0]
			if pkg.Version == VersionUnknown {
				pkg.Version = Version4
			}
		}
		packages = append(packages, pkg)
	}
	return Layout{Packages: packages}, nil
}

func ensureCandidate(candidates map[string]*packageCandidate, dir string) *packageCandidate {
	candidate, found := candidates[dir]
	if !found {
		candidate = &packageCandidate{}
		candidates[dir] = candidate
	}
	return candidate
}

func cleanDir(dir string) string {
	cleaned := cleanPath(path.Clean(dir))
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func isTailwindConfig(name string) bool {
	if !strings.HasPrefix(name, "tailwind.config.") {
		return false
	}
	switch path.Ext(name) {
	case ".js", ".cjs", ".mjs", ".ts", ".mts", ".cts":
		return true
	}
	return false
}

func hasV4EntrySignal(nodes []cssdecl.Node) bool {
	for _, node := range nodes {
		if node.Kind == cssdecl.NodeAtRule &&
			((node.Name == "import" && strings.Contains(node.Prelude, "tailwindcss")) || node.Name == "theme") {
			return true
		}
		if hasV4EntrySignal(node.Children) {
			return true
		}
	}
	return false
}

func nearestManifestDir(fsys fs.FS, start string) string {
	dir := cleanDir(start)
	for {
		if info, err := fs.Stat(fsys, path.Join(dir, "package.json")); err == nil && !info.IsDir() {
			return dir
		}
		if dir == "." {
			return cleanDir(start)
		}
		dir = cleanDir(path.Dir(dir))
	}
}

func preferredConfig(configs []string) string {
	preference := map[string]int{".ts": 0, ".mts": 1, ".cts": 2, ".js": 3, ".cjs": 4, ".mjs": 5}
	sorted := append([]string(nil), configs...)
	sort.SliceStable(sorted, func(first, second int) bool {
		leftRank, rightRank := preference[path.Ext(sorted[first])], preference[path.Ext(sorted[second])]
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return sorted[first] < sorted[second]
	})
	return sorted[0]
}

// PackageFor returns the nearest ancestor package for file.
func (layout Layout) PackageFor(file string) (Package, bool) {
	file = cleanPath(path.Clean(file))
	bestIndex := -1
	bestLength := -1
	for index, pkg := range layout.Packages {
		ancestor := pkg.Dir == "." || file == pkg.Dir || strings.HasPrefix(file, pkg.Dir+"/")
		if ancestor && len(pkg.Dir) > bestLength {
			bestIndex = index
			bestLength = len(pkg.Dir)
		}
	}
	if bestIndex < 0 {
		return Package{}, false
	}
	return layout.Packages[bestIndex], true
}
