package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Default directory name segments that should never be watched or trigger rebuilds.
var defaultIgnoreDirNames = map[string]struct{}{
	".git":          {},
	"node_modules":  {},
	"__pycache__":   {},
	".venv":         {},
	"venv":          {},
	"target":        {},
	"dist":          {},
	"build":         {},
	".next":         {},
	".nuxt":         {},
	".output":       {},
	".tox":          {},
	".pytest_cache": {},
	".mypy_cache":   {},
	".idea":         {},
	".vscode":       {},
}

// File basenames / suffixes that should not trigger rebuilds.
var defaultIgnoreFileNames = map[string]struct{}{
	".ds_store": {},
}

var defaultIgnoreSuffixes = []string{".pyc", ".pyo"}

// ShouldRebuild reports whether a path relative to the project root should
// schedule a rebuild. Rel may use either / or OS separators; empty means root.
func ShouldRebuild(rel string, extra []string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return false
	}
	if strings.HasPrefix(rel, "../") {
		return false
	}
	base := filepath.Base(rel)
	lowerBase := strings.ToLower(base)
	if _, ok := defaultIgnoreFileNames[lowerBase]; ok {
		return false
	}
	for _, suf := range defaultIgnoreSuffixes {
		if strings.HasSuffix(lowerBase, suf) {
			return false
		}
	}
	parts := strings.Split(rel, "/")
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if _, ok := defaultIgnoreDirNames[p]; ok {
			return false
		}
	}
	for _, ig := range extra {
		ig = strings.TrimSpace(ig)
		if ig == "" {
			continue
		}
		igSlash := filepath.ToSlash(ig)
		for _, p := range parts {
			if p == igSlash {
				return false
			}
		}
		if matched, _ := filepath.Match(igSlash, rel); matched {
			return false
		}
		if matched, _ := filepath.Match(igSlash, base); matched {
			return false
		}
		// Also match glob against any path suffix segment path.
		for i := range parts {
			sub := strings.Join(parts[i:], "/")
			if matched, _ := filepath.Match(igSlash, sub); matched {
				return false
			}
		}
	}
	return true
}

// isIgnoredDirName reports whether a single directory basename should be skipped
// when installing watches (defaults + --ignore exact segment).
func isIgnoredDirName(name string, extra []string) bool {
	if _, ok := defaultIgnoreDirNames[name]; ok {
		return true
	}
	for _, ig := range extra {
		ig = strings.TrimSpace(ig)
		if ig == "" {
			continue
		}
		if name == ig {
			return true
		}
		if matched, _ := filepath.Match(filepath.ToSlash(ig), name); matched {
			return true
		}
	}
	return false
}

// collectWatchDirs walks root and returns absolute directory paths to watch.
// Ignored directories are not entered (no children added).
func collectWatchDirs(root string, extra []string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable entries; keep walking.
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if path != root {
			name := info.Name()
			if isIgnoredDirName(name, extra) {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && !ShouldRebuild(rel, extra) {
				return filepath.SkipDir
			}
		}
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}
