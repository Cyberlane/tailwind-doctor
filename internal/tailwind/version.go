package tailwind

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/cssdecl"
)

// Version is a supported Tailwind major version.
type Version string

const (
	VersionUnknown Version = ""
	Version3       Version = "3"
	Version4       Version = "4"
)

// Evidence is one inspectable version signal.
type Evidence struct {
	Signal string
	File   string
	Detail string
}

// Detection is a version verdict and every signal that informed it.
type Detection struct {
	Version            Version
	UnsupportedVersion string
	Evidence           []Evidence
}

type packageManifest struct {
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
}

// Detect examines dir and CSS files at most one directory below it. Package
// metadata is authoritative when it resolves a supported major version.
func Detect(fsys fs.FS, dir string) (Detection, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return Detection{}, fmt.Errorf("list %s: %w", dir, err)
	}

	evidence := []Evidence{}
	packageVersion := VersionUnknown
	unsupportedVersion := ""
	manifestPath := path.Join(dir, "package.json")
	if content, err := fs.ReadFile(fsys, manifestPath); err == nil {
		var manifest packageManifest
		if json.Unmarshal(content, &manifest) == nil {
			if versionRange, found := dependencyRange(manifest, "tailwindcss"); found {
				if version, resolved := supportedMajor(versionRange); resolved {
					packageVersion = version
					evidence = append(evidence, Evidence{Signal: "package-json", File: cleanPath(manifestPath), Detail: "tailwindcss " + versionRange})
				} else if major, resolved := dependencyMajor(versionRange); resolved {
					unsupportedVersion = major
					evidence = append(evidence, Evidence{Signal: "package-json", File: cleanPath(manifestPath), Detail: "tailwindcss " + versionRange})
				}
			}
			for _, dependency := range []string{"@tailwindcss/vite", "@tailwindcss/postcss"} {
				if versionRange, found := dependencyRange(manifest, dependency); found {
					evidence = append(evidence, Evidence{Signal: "package-json", File: cleanPath(manifestPath), Detail: dependency + " " + versionRange})
					if major, resolved := dependencyMajor(versionRange); resolved && major != "4" {
						unsupportedVersion = major
					} else if packageVersion == VersionUnknown && unsupportedVersion == "" {
						packageVersion = Version4
					}
				}
			}
		}
	}

	cssFiles := []string{}
	configExtensions := map[string]bool{".js": true, ".cjs": true, ".mjs": true, ".ts": true, ".mts": true, ".cts": true}
	for _, entry := range entries {
		entryPath := path.Join(dir, entry.Name())
		if entry.IsDir() {
			children, readErr := fs.ReadDir(fsys, entryPath)
			if readErr != nil {
				return Detection{}, fmt.Errorf("list version evidence in %s: %w", entryPath, readErr)
			}
			for _, child := range children {
				if !child.IsDir() && path.Ext(child.Name()) == ".css" {
					cssFiles = append(cssFiles, path.Join(entryPath, child.Name()))
				}
			}
			continue
		}
		if strings.HasPrefix(entry.Name(), "tailwind.config.") && configExtensions[path.Ext(entry.Name())] {
			evidence = append(evidence, Evidence{Signal: "config-file", File: cleanPath(entryPath), Detail: entry.Name()})
		}
		if path.Ext(entry.Name()) == ".css" {
			cssFiles = append(cssFiles, entryPath)
		}
	}

	sort.Strings(cssFiles)
	for _, file := range cssFiles {
		content, readErr := fs.ReadFile(fsys, file)
		if readErr != nil {
			return Detection{}, fmt.Errorf("read version evidence %s: %w", file, readErr)
		}
		sheet, parseErr := cssdecl.Parse(string(content))
		if parseErr != nil {
			continue
		}
		collectCSSEvidence(sheet.Nodes, cleanPath(file), &evidence)
	}

	sortEvidence(evidence)
	version := packageVersion
	if unsupportedVersion != "" {
		version = VersionUnknown
	}
	if version == VersionUnknown {
		if unsupportedVersion != "" {
			return Detection{Version: version, UnsupportedVersion: unsupportedVersion, Evidence: evidence}, nil
		}
		for _, item := range evidence {
			if item.Signal == "css-import" || item.Signal == "css-theme" {
				version = Version4
				break
			}
		}
	}
	if version == VersionUnknown {
		for _, item := range evidence {
			if item.Signal == "config-file" || item.Signal == "css-directive" {
				version = Version3
				break
			}
		}
	}
	return Detection{Version: version, Evidence: evidence}, nil
}

func dependencyRange(manifest packageManifest, name string) (string, bool) {
	for _, dependencies := range []map[string]string{manifest.Dependencies, manifest.DevDependencies, manifest.PeerDependencies} {
		if versionRange, found := dependencies[name]; found {
			return versionRange, true
		}
	}
	return "", false
}

func supportedMajor(versionRange string) (Version, bool) {
	major, found := dependencyMajor(versionRange)
	if !found {
		return VersionUnknown, false
	}
	switch major {
	case "3":
		return Version3, true
	case "4":
		return Version4, true
	}
	return VersionUnknown, false
}

func dependencyMajor(versionRange string) (string, bool) {
	trimmed := strings.TrimSpace(versionRange)
	trimmed = strings.TrimLeft(trimmed, "^~><=v ")
	end := 0
	for end < len(trimmed) && trimmed[end] >= '0' && trimmed[end] <= '9' {
		end++
	}
	if end == 0 {
		return "", false
	}
	return trimmed[:end], true
}

func collectCSSEvidence(nodes []cssdecl.Node, file string, evidence *[]Evidence) {
	for _, node := range nodes {
		if node.Kind == cssdecl.NodeAtRule {
			switch {
			case node.Name == "import" && strings.Contains(node.Prelude, "tailwindcss"):
				*evidence = append(*evidence, Evidence{Signal: "css-import", File: file, Detail: node.Prelude})
			case node.Name == "theme":
				*evidence = append(*evidence, Evidence{Signal: "css-theme", File: file, Detail: node.Prelude})
			case node.Name == "tailwind":
				*evidence = append(*evidence, Evidence{Signal: "css-directive", File: file, Detail: node.Prelude})
			}
		}
		collectCSSEvidence(node.Children, file, evidence)
	}
}

func sortEvidence(evidence []Evidence) {
	rank := map[string]int{"package-json": 0, "config-file": 1, "css-import": 2, "css-theme": 3, "css-directive": 4}
	sort.SliceStable(evidence, func(first, second int) bool {
		left, right := evidence[first], evidence[second]
		if rank[left.Signal] != rank[right.Signal] {
			return rank[left.Signal] < rank[right.Signal]
		}
		if left.File != right.File {
			return left.File < right.File
		}
		return left.Detail < right.Detail
	})
}

func cleanPath(file string) string {
	return strings.TrimPrefix(file, "./")
}
