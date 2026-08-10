package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FixResult describes the source changes made by Fix.
type FixResult struct {
	Files        int
	Replacements int
}

type sourceEdit struct {
	start       int
	end         int
	replacement string
}

type rewrittenFile struct {
	path    string
	mode    os.FileMode
	content []byte
}

// Fix applies only replacements proven by the same static analysis that
// produced the report. Configuration, allowlists, and baselines are honored, so
// --fix never edits a finding the project chose not to report.
func Fix(root string) (FixResult, error) {
	config, err := LoadConfig(root)
	if err != nil {
		return FixResult{}, err
	}
	baseline, err := LoadBaseline(root, config.BaselinePath)
	if err != nil {
		return FixResult{}, err
	}
	report, err := RunWithConfig(root, config, baseline)
	if err != nil {
		return FixResult{}, err
	}
	return applyFixes(root, report.Findings)
}

func applyFixes(root string, findings []Finding) (FixResult, error) {
	byFile := map[string][]Finding{}
	for _, finding := range findings {
		if finding.fixable {
			byFile[finding.File] = append(byFile[finding.File], finding)
		}
	}
	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)

	prepared := make([]rewrittenFile, 0, len(files))
	result := FixResult{}
	for _, file := range files {
		path, err := sourcePath(root, file)
		if err != nil {
			return FixResult{}, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return FixResult{}, fmt.Errorf("inspect %s before fixing: %w", file, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return FixResult{}, fmt.Errorf("refuse to fix symbolic link %s", file)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return FixResult{}, fmt.Errorf("read %s before fixing: %w", file, err)
		}

		edits := make([]sourceEdit, 0, len(byFile[file]))
		for _, finding := range byFile[file] {
			start, found := offsetAt(content, finding.Line, finding.Column)
			if !found || start+len(finding.Class) > len(content) ||
				string(content[start:start+len(finding.Class)]) != finding.Class {
				return FixResult{}, fmt.Errorf("refuse stale fix for %s:%d:%d: source no longer contains %q",
					file, finding.Line, finding.Column, finding.Class)
			}
			edits = append(edits, sourceEdit{
				start: start, end: start + len(finding.Class),
				replacement: finding.replacement,
			})
		}
		sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
		for index := 1; index < len(edits); index++ {
			if edits[index].start < edits[index-1].end {
				return FixResult{}, fmt.Errorf("refuse overlapping fixes in %s", file)
			}
		}

		var rewritten strings.Builder
		rewritten.Grow(len(content))
		cursor := 0
		for _, edit := range edits {
			rewritten.Write(content[cursor:edit.start])
			rewritten.WriteString(edit.replacement)
			cursor = edit.end
		}
		rewritten.Write(content[cursor:])
		prepared = append(prepared, rewrittenFile{
			path: path, mode: info.Mode().Perm(), content: []byte(rewritten.String()),
		})
		result.Replacements += len(edits)
	}

	// Validate every edit before the first write, then replace each source through
	// a temporary file in the same directory. A failed validation therefore leaves
	// the entire project untouched, and a successful rename is atomic per file.
	for _, file := range prepared {
		if err := replaceFile(file); err != nil {
			return FixResult{}, err
		}
		result.Files++
	}
	return result, nil
}

func sourcePath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("refuse source path outside analysis root: %s", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refuse source path outside analysis root: %s", relative)
	}
	return filepath.Join(root, clean), nil
}

func offsetAt(content []byte, line, column int) (int, bool) {
	if line < 1 || column < 1 {
		return 0, false
	}
	currentLine, currentColumn := 1, 1
	for offset, character := range content {
		if currentLine == line && currentColumn == column {
			return offset, true
		}
		if character == '\n' {
			currentLine, currentColumn = currentLine+1, 1
		} else {
			currentColumn++
		}
	}
	if currentLine == line && currentColumn == column {
		return len(content), true
	}
	return 0, false
}

func replaceFile(file rewrittenFile) error {
	temporary, err := os.CreateTemp(filepath.Dir(file.path), ".tw-doctor-fix-*")
	if err != nil {
		return fmt.Errorf("create temporary fix for %s: %w", file.path, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = temporary.Close() }()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(file.mode); err != nil {
		return fmt.Errorf("preserve permissions for %s: %w", file.path, err)
	}
	if _, err := temporary.Write(file.content); err != nil {
		return fmt.Errorf("write temporary fix for %s: %w", file.path, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary fix for %s: %w", file.path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary fix for %s: %w", file.path, err)
	}
	if err := os.Rename(temporaryPath, file.path); err != nil {
		return fmt.Errorf("replace %s with fixed source: %w", file.path, err)
	}
	return nil
}
