package project

// Project root markers - nearest walk-up wins (innermost).
// Product policy list (no mature shared Go lib for this).
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

// genericBases: session name gets parent- prefix (web -> parent-web).
var genericBases = map[string]bool{
	"app": true, "apps": true, "web": true, "www": true, "api": true,
	"src": true, "lib": true, "core": true, "server": true, "client": true,
	"frontend": true, "backend": true, "mobile": true, "docs": true,
	"test": true, "tests": true, "cmd": true, "internal": true,
	"site": true, "admin": true, "service": true, "services": true,
	"game": true, "godot": true, "project": true, "src-tauri": true,
}

// skipDir: never umbrella children.
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
