package project

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GitSubmodule is one .gitmodules entry.
type GitSubmodule struct {
	Path string // relative to repo root
	URL  string
}

// ListSubmodules reads .gitmodules entries via a simple line parser.
func ListSubmodules(root string) []GitSubmodule {
	root = filepath.Clean(root)
	if root == "" {
		return nil
	}
	return listSubmodulesFallback(root)
}

// SharedSubmoduleURLs maps remote URL -> relative paths (homology: same util).
func SharedSubmoduleURLs(root string) map[string][]string {
	byURL := map[string][]string{}
	for _, sm := range ListSubmodules(root) {
		if sm.URL == "" || sm.Path == "" {
			continue
		}
		byURL[sm.URL] = append(byURL[sm.URL], sm.Path)
	}
	for u, paths := range byURL {
		sort.Strings(paths)
		byURL[u] = paths
	}
	return byURL
}

// listSubmodulesFallback: line parser (always works on plain .gitmodules).
func listSubmodulesFallback(root string) []GitSubmodule {
	f, err := os.Open(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []GitSubmodule
	var cur *GitSubmodule
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[submodule") {
			if cur != nil && cur.Path != "" {
				out = append(out, *cur)
			}
			cur = &GitSubmodule{}
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		val = strings.TrimSpace(val)
		switch key {
		case "path":
			cur.Path = filepath.Clean(val)
		case "url":
			cur.URL = val
		}
	}
	if cur != nil && cur.Path != "" {
		out = append(out, *cur)
	}
	return out
}

// IsGitRepo reports whether root is a git work tree or linked worktree.
func IsGitRepo(root string) bool {
	_, err := os.Stat(filepath.Join(filepath.Clean(root), ".git"))
	return err == nil
}
