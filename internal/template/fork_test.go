package template

import (
	"path/filepath"
	"testing"

	"github.com/fm39hz/gotomux/internal/store"
)

func TestWindowForkKeyStableAcrossProjects(t *testing.T) {
	a := store.PresetWindow{
		Name: "editor", Cwd: "/work/a",
		Panes: []store.PresetPane{{Cwd: "/work/a", Cmd: "nvim"}},
	}
	b := store.PresetWindow{
		Name: "code", Cwd: "/other/b",
		Panes: []store.PresetPane{{Cwd: "/other/b/src", Cmd: "nvim"}},
	}
	if WindowForkKey(a) != WindowForkKey(b) {
		t.Fatalf("nvim×1 must be one fork: %s vs %s", WindowForkKey(a), WindowForkKey(b))
	}
	shell := store.PresetWindow{
		Layout: "even-vertical",
		Panes:  []store.PresetPane{{}, {}},
	}
	if WindowForkKey(a) == WindowForkKey(shell) {
		t.Fatal("editor ≠ shell-v2")
	}
}

func TestObserveForksLearnsFromFreeze(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	p := &store.Preset{
		Name: "proj", Cwd: "/work/x",
		Windows: []store.PresetWindow{
			{Name: "editor", Panes: []store.PresetPane{{Cmd: "nvim"}}},
			{Name: "shell", Layout: "4080,158x35,0,0[1,2]", Panes: []store.PresetPane{{}, {}}},
			{Name: "yazi", Panes: []store.PresetPane{{Cmd: "yazi"}}},
		},
	}
	// two freezes → hit counts
	if _, _, err := FreezeSave(st, p, false); err != nil {
		t.Fatal(err)
	}
	p2 := &store.Preset{
		Name: "other", Cwd: "/work/y",
		Windows: []store.PresetWindow{
			{Name: "ed", Panes: []store.PresetPane{{Cmd: "nvim"}}},
			{Name: "sh", Layout: "even-vertical", Panes: []store.PresetPane{{}, {}}},
			{Name: "files", Panes: []store.PresetPane{{Cmd: "yazi"}}},
		},
	}
	if _, _, err := FreezeSave(st, p2, true); err != nil {
		t.Fatal(err)
	}

	ek := WindowForkKey(ToShape(p, "t").Windows[0])
	sk := WindowForkKey(ToShape(p, "t").Windows[1])
	yk := WindowForkKey(ToShape(p, "t").Windows[2])
	if st.ForkHits(ek) < 2 {
		t.Fatalf("editor-nvim hits %d want ≥2", st.ForkHits(ek))
	}
	if st.ForkHits(sk) < 2 {
		t.Fatalf("shell-v2 hits %d", st.ForkHits(sk))
	}
	if st.ForkHits(yk) < 2 {
		t.Fatalf("yazi hits %d", st.ForkHits(yk))
	}
	// divergence: new agent window → new fork key
	p3 := &store.Preset{
		Name: "z", Cwd: "/z",
		Windows: []store.PresetWindow{
			{Panes: []store.PresetPane{{Cmd: "nvim"}}},
			{Panes: []store.PresetPane{{Cmd: "opencode"}}},
		},
	}
	ObserveForks(st, p3)
	ak := WindowForkKey(ToShape(p3, "t").Windows[1])
	if st.ForkHits(ak) < 1 {
		t.Fatal("opencode fork not learned")
	}
	if ak == ek {
		t.Fatal("agent ≠ editor")
	}
}
