package picker

import (
	"os"
	"strings"
)

// TUI chrome may use Nerd Font glyphs when the terminal likely has them.
// Filenames, shape labels on disk, and logs stay ASCII (template.ShapeLabel).
//
// GOTOMUX_ASCII=1 forces plain ASCII even with a nerd font.

func useNerdIcons() bool {
	if os.Getenv("GOTOMUX_ASCII") == "1" {
		return false
	}
	// common nerd-patched terminals / multiplexers
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "kitty") || strings.Contains(term, "wezterm") ||
		strings.Contains(term, "ghostty") || strings.Contains(term, "alacritty") {
		return true
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("WEZTERM_PANE") != "" ||
		os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}
	// user opt-in
	if os.Getenv("GOTOMUX_NERD") == "1" {
		return true
	}
	// default on for modern local linux/mac TTY (user asked for nerd when useful)
	return true
}

func iconPrompt() string {
	if useNerdIcons() {
		return " " // nf-fa-terminal
	}
	return "> "
}

func iconCursor() string {
	if useNerdIcons() {
		return " " // nf-fa-caret_right
	}
	return "> "
}

func iconSticky() string {
	if useNerdIcons() {
		return " " // nf-fa-thumb_tack
	}
	return "sticky:"
}

// iconForTool maps tool/split tokens in a sticky label to nerd icons.
// Unknown tokens stay as text (v2, t4, sh, ...).
func iconForTool(tok string) string {
	if !useNerdIcons() {
		return tok
	}
	switch tok {
	case "nvim", "vim", "vi", "hx", "helix", "editor":
		return "" // nf-dev-vim / nvim-ish
	case "yazi", "files", "lf", "ranger":
		return "" // folder
	case "opencode", "claude", "codex", "aider", "pi", "agent":
		return "" // robot
	case "git", "lazygit":
		return "" // git branch-ish
	case "shell", "sh":
		return "" // terminal
	default:
		// v2, t4, h2, pN - keep ascii
		return tok
	}
}

// formatStickyMeta: " sticky:nvim+v2+yazi" or " <pin> <icons>+v2+..."
func formatStickyMeta(label string) string {
	if label == "" || label == "default" {
		return ""
	}
	if !useNerdIcons() {
		return "  sticky:" + label
	}
	parts := strings.Split(label, "+")
	for i, p := range parts {
		parts[i] = iconForTool(p)
	}
	return "  " + iconSticky() + strings.Join(parts, "+")
}
