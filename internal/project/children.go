package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Children returns stable-ordered project-like subdirs of root for placement slots C0,C1,...
// Order: gitmodules paths first (go-git / fallback), then other nested projects by path.
// Empty if root missing or flat.
//
// Signals (git is not sole source): submodules > nested git/markers.
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
		if seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}

	// 1) .gitmodules paths (stable order) via go-git config + fallback
	for _, sm := range ListSubmodules(root) {
		add(filepath.Join(root, sm.Path))
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
	if IsGitRepo(dir) {
		return true
	}
	if DirExists(filepath.Join(dir, ".git")) || FileExists(filepath.Join(dir, ".git")) {
		return true
	}
	return isProjectRoot(dir)
}
