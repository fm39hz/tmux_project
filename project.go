package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Project root markers — nearest walk-up wins (innermost).
// Ordered only for readability; any hit stops the walk.
var projectMarkers = []string{
	// VCS
	".git",
	".jj",
	".hg",
	// JS / TS
	"package.json",
	"pnpm-workspace.yaml",
	"turbo.json",
	"nx.json",
	"lerna.json",
	"deno.json",
	"deno.jsonc",
	"bun.lock",
	"bun.lockb",
	// Go / Rust / C / C++
	"go.mod",
	"Cargo.toml",
	"CMakeLists.txt",
	"meson.build",
	"Makefile",
	"makefile",
	// .NET
	// (sln/csproj handled via ext walk below)
	"global.json",
	"Directory.Build.props",
	// Python
	"pyproject.toml",
	"setup.py",
	"requirements.txt",
	"Pipfile",
	"poetry.lock",
	// JVM
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
	"settings.gradle",
	"settings.gradle.kts",
	// Ruby / PHP / Elixir
	"Gemfile",
	"composer.json",
	"mix.exs",
	// Godot / Unity-ish
	"project.godot",
	"ProjectSettings", // Unity (dir)
	// Nix / misc
	"flake.nix",
	"shell.nix",
	".envrc", // direnv often at project root
}

// genericBases: basename alone collides often → prefix with parent.
var genericBases = map[string]bool{
	"app": true, "apps": true, "web": true, "www": true, "api": true,
	"src": true, "lib": true, "core": true, "server": true, "client": true,
	"frontend": true, "backend": true, "mobile": true, "docs": true,
	"test": true, "tests": true, "cmd": true, "internal": true,
	"site": true, "admin": true, "service": true, "services": true,
	"game": true, "godot": true, "project": true, "src-tauri": true,
}

var projectRootMemo sync.Map // cleaned path → root

// findProjectRoot walks up from start to the nearest directory that looks
// like a project root. If nothing matches, returns cleaned start (not /).
func findProjectRoot(start string) string {
	start = filepath.Clean(start)
	if start == "" {
		if cwd, err := os.Getwd(); err == nil {
			start = cwd
		} else {
			return start
		}
	}
	if v, ok := projectRootMemo.Load(start); ok {
		return v.(string)
	}
	path := start
	var chain []string
	root := start
	for path != "/" && path != "." && path != "" {
		chain = append(chain, path)
		if isProjectRoot(path) {
			root = path
			break
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	for _, p := range chain {
		projectRootMemo.Store(p, root)
	}
	projectRootMemo.Store(start, root)
	return root
}

func isProjectRoot(dir string) bool {
	for _, m := range projectMarkers {
		p := filepath.Join(dir, m)
		if fileExists(p) || dirExists(p) {
			return true
		}
	}
	// .NET: any *.sln or *.csproj in this directory only (not recursive)
	if hasExtInDir(dir, ".sln") || hasExtInDir(dir, ".csproj") {
		return true
	}
	return false
}

func hasExtInDir(dir, ext string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	ext = strings.ToLower(ext)
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ext) {
			return true
		}
	}
	return false
}

// projectSession resolves path → (session name, project root cwd).
// Subdirs inside a repo map to the repo session, not a bogus "src" session.
func projectSession(path string) (name, root string) {
	root = findProjectRoot(path)
	name = sessionName(root)
	return name, root
}

// sessionName: tmux-safe name from project root basename.
// Generic basenames get parent prefix to cut collisions (web → org-web).
func sessionName(root string) string {
	root = filepath.Clean(root)
	base := sanitizeSessionPart(filepath.Base(root))
	if base == "" {
		base = "session"
	}
	if genericBases[base] {
		parent := sanitizeSessionPart(filepath.Base(filepath.Dir(root)))
		if parent != "" && parent != base {
			return parent + "-" + base
		}
	}
	return base
}

func sanitizeSessionPart(s string) string {
	s = strings.TrimPrefix(s, ".")
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r == ' ' || r == '.' || r == '_':
			return '-'
		default:
			return -1
		}
	}, s)
	// collapse --
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
