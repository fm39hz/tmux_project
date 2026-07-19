package template

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/tmux"
)

// Fork = one window essence unit (reusable unit).
// Learned silently when user freezes or sticks after working with sticky.
// Same/different from sticky windows both count as signal: hits rise on match,
// new keys appear on divergence (fork of habit).

// WindowForkKey fingerprints one window: pane count + split class + tools.
// Labels/cwd ignored - "first pane nvim" is one key everywhere.
func WindowForkKey(w store.PresetWindow) string {
	n := len(w.Panes)
	if n == 0 {
		n = 1
	}
	split := tmux.LayoutForShape(w.Layout, n)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d", n))
	b.WriteByte('|')
	b.WriteString(split)
	b.WriteByte('|')
	for j := 0; j < n; j++ {
		if j > 0 {
			b.WriteByte(',')
		}
		if j < len(w.Panes) {
			b.WriteString(tmux.ToolIntent(w.Panes[j].Cmd))
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

// windowForkBody: product JSON fragment for one window (readable, no cwd).
func windowForkBody(w store.PresetWindow) string {
	n := len(w.Panes)
	if n == 0 {
		n = 1
	}
	type pane struct {
		Cmd string `json:"cmd,omitempty"`
	}
	type win struct {
		Split string `json:"split,omitempty"`
		Panes []pane `json:"panes"`
	}
	ww := win{Split: tmux.LayoutForShape(w.Layout, n)}
	for j := 0; j < n; j++ {
		var c string
		if j < len(w.Panes) {
			c = tmux.ToolIntent(w.Panes[j].Cmd)
		}
		ww.Panes = append(ww.Panes, pane{Cmd: c})
	}
	b, err := json.Marshal(ww)
	if err != nil {
		return ""
	}
	return string(b)
}

// ObserveForks records every window of preset as a fork unit (best-effort, silent).
// Call after freeze/stick when shape essence is taken from p.
func ObserveForks(st *store.Store, p *store.Preset) {
	if st == nil || p == nil {
		return
	}
	// learn from pure shape windows (same essence as sticky body)
	sh := ToShape(p, "fork")
	for _, w := range sh.Windows {
		key := WindowForkKey(w)
		if key == "" {
			continue
		}
		_ = st.RecordFork(key, windowForkBody(w))
	}
}
