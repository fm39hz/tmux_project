package project

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/config"
)

// GitSubmodule is one .gitmodules entry.
type GitSubmodule struct {
	Path string // relative to repo root
	URL  string
}

// ListSubmodules reads submodule path/url via go-git config decoder when possible,
// with a small line parser fallback. No git binary required.
func ListSubmodules(root string) []GitSubmodule {
	root = filepath.Clean(root)
	if root == "" {
		return nil
	}
	// Warm go-git open (validates repo / worktree .git); submodule data from file.
	_, _ = git.PlainOpenWithOptions(root, &git.PlainOpenOptions{
		DetectDotGit: true,
	})

	if sms := listSubmodulesGoGit(root); len(sms) > 0 {
		return sms
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

func listSubmodulesGoGit(root string) []GitSubmodule {
	fs := osfs.New(root)
	f, err := fs.Open(".gitmodules")
	if err != nil {
		return nil
	}
	defer f.Close()
	cfg := config.New()
	if err := config.NewDecoder(f).Decode(cfg); err != nil {
		return nil
	}
	var out []GitSubmodule
	for _, sec := range cfg.Sections {
		if sec.Name != "submodule" {
			continue
		}
		for _, sub := range sec.Subsections {
			sm := GitSubmodule{}
			for _, o := range sub.Options {
				switch strings.ToLower(o.Key) {
				case "path":
					sm.Path = filepath.Clean(o.Value)
				case "url":
					sm.URL = strings.TrimSpace(o.Value)
				}
			}
			if sm.Path != "" && sm.Path != "." {
				out = append(out, sm)
			}
		}
	}
	return out
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

// IsGitRepo reports whether root is a git work tree / repo (go-git).
func IsGitRepo(root string) bool {
	_, err := git.PlainOpenWithOptions(filepath.Clean(root), &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	return err == nil
}
