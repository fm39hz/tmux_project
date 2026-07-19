package project

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Project root markers - nearest walk-up wins (innermost).
var projectMarkers = []string{
	".git", ".jj", ".hg",
	"package.json", "pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json",
	"deno.json", "deno.jsonc", "bun.lock", "bun.lockb",
	"go.mod", "Cargo.toml", "CMakeLists.txt", "meson.build", "Makefile", "makefile",
	"global.json", "Directory.Build.props",
	"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", "poetry.lock",
	"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
	"Gemfile", "composer.json", "mix.exs",
	"project.godot", "ProjectSettings",
	"flake.nix", "shell.nix", ".envrc",
}

var genericBases = map[string]bool{
	"app": true, "apps": true, "web": true, "www": true, "api": true,
	"src": true, "lib": true, "core": true, "server": true, "client": true,
	"frontend": true, "backend": true, "mobile": true, "docs": true,
	"test": true, "tests": true, "cmd": true, "internal": true,
	"site": true, "admin": true, "service": true, "services": true,
	"game": true, "godot": true, "project": true, "src-tauri": true,
}

var projectRootMemo sync.Map

// FindProjectRoot walks up from start to nearest project-looking directory.
func FindProjectRoot(start string) string {
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
		if FileExists(p) || DirExists(p) {
			return true
		}
	}
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

// Session returns (session name, project root) for path.
func Session(path string) (name, root string) {
	root = FindProjectRoot(path)
	name = SessionName(root)
	return name, root
}

// SessionName: tmux-safe name from project root basename.
func SessionName(root string) string {
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
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// ValidSessionName: tmux targets use "sess:win" - colon/control break them.
func ValidSessionName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch r {
		case ':', '\n', '\r', '\t':
			return false
		}
	}
	return true
}

func FileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func DirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
