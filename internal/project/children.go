package project

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skipDir: never treat as umbrella children.
var skipDir = map[string]bool{
	".git": true, ".jj": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "target": true,
	"dist": true, "build": true, "out": true, "bin": true,
	".cache": true, ".local": true, "__pycache__": true,
	".idea": true, ".vscode": true, ".cursor": true,
	".opencode": true, ".claude": true, ".codex": true,
	".agents": true, ".codegraph": true, ".githooks": true,
	".github": true, ".husky": true, "logs": true, "tmp": true,
	"coverage": true, ".next": true, ".turbo": true,
}

// Children returns stable-ordered project-like subdirs of root for placement slots C0,C1,...
// Order: gitmodules paths first (file order), then other nested projects by path.
// Empty if root missing or flat.
func Children(root string) []string {
	root = filepath.Clean(root)
	if root == "" || !DirExists(root) {
		return nil
	}
	seen := map[string]bool{}
	var out []string

	add := func(abs string) {
		abs = filepath.Clean(abs)
		if abs == root || !DirExists(abs) {
			return
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return
		}
		// only direct or gitmodules path (may be nested one level typically)
		if seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}

	// 1) .gitmodules paths (stable file order)
	for _, rel := range gitmodulePaths(root) {
		add(filepath.Join(root, rel))
	}

	// 2) direct children that look like projects
	ents, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	var extra []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && skipDir[name] {
			continue
		}
		if skipDir[name] {
			continue
		}
		abs := filepath.Join(root, name)
		if seen[abs] {
			continue
		}
		if isChildProject(abs) {
			extra = append(extra, abs)
		}
	}
	sort.Strings(extra)
	for _, abs := range extra {
		add(abs)
	}
	return out
}

func isChildProject(dir string) bool {
	// nested git
	if DirExists(filepath.Join(dir, ".git")) || FileExists(filepath.Join(dir, ".git")) {
		return true
	}
	return isProjectRoot(dir)
}

// gitmodulePaths: relative paths from .gitmodules, file order.
func gitmodulePaths(root string) []string {
	f, err := os.Open(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var paths []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "path") {
			continue
		}
		// path = foo  or path=foo
		_, rest, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		p := strings.TrimSpace(rest)
		if p != "" {
			paths = append(paths, filepath.Clean(p))
		}
	}
	return paths
}
