package template

import (
	"path/filepath"
	"testing"

	"github.com/fm39hz/gotomux/internal/model"
	"github.com/fm39hz/gotomux/internal/store"
)

func TestWindowForkKeyStableAcrossProjects(t *testing.T) {
	a := model.Window{
		Name: "editor", Cwd: "/work/a",
		Panes: []model.Pane{{Cwd: "/work/a", Cmd: "nvim"}},
	}
	b := model.Window{
		Name: "code", Cwd: "/other/b",
		Panes: []model.Pane{{Cwd: "/other/b/src", Cmd: "nvim"}},
	}
	if WindowForkKey(a) != WindowForkKey(b) {
		t.Fatalf("nvimx1 must be one fork: %s vs %s", WindowForkKey(a), WindowForkKey(b))
	}
	shell := model.Window{
		Layout: "even-vertical",
		Panes:  []model.Pane{{}, {}},
	}
	if WindowForkKey(a) == WindowForkKey(shell) {
		t.Fatal("editor != shell-v2")
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

	p := &model.Session{
		Name: "proj", Cwd: "/work/x",
		Windows: []model.Window{
			{Name: "editor", Panes: []model.Pane{{Cmd: "nvim"}}},
			{Name: "shell", Layout: "4080,158x35,0,0[1,2]", Panes: []model.Pane{{}, {}}},
			{Name: "yazi", Panes: []model.Pane{{Cmd: "yazi"}}},
		},
	}
	// two freezes -> hit counts
	if _, _, err := FreezeSave(st, p, false); err != nil {
		t.Fatal(err)
	}
	p2 := &model.Session{
		Name: "other", Cwd: "/work/y",
		Windows: []model.Window{
			{Name: "ed", Panes: []model.Pane{{Cmd: "nvim"}}},
			{Name: "sh", Layout: "even-vertical", Panes: []model.Pane{{}, {}}},
			{Name: "files", Panes: []model.Pane{{Cmd: "yazi"}}},
		},
	}
	if _, _, err := FreezeSave(st, p2, true); err != nil {
		t.Fatal(err)
	}

	ek := WindowForkKey(ToShape(p, "t").Windows[0])
	sk := WindowForkKey(ToShape(p, "t").Windows[1])
	yk := WindowForkKey(ToShape(p, "t").Windows[2])
	if st.ForkHits(ek) < 2 {
		t.Fatalf("editor-nvim hits %d want >=2", st.ForkHits(ek))
	}
	if st.ForkHits(sk) < 2 {
		t.Fatalf("shell-v2 hits %d", st.ForkHits(sk))
	}
	if st.ForkHits(yk) < 2 {
		t.Fatalf("yazi hits %d", st.ForkHits(yk))
	}
	// divergence: new agent window -> new fork key
	p3 := &model.Session{
		Name: "z", Cwd: "/z",
		Windows: []model.Window{
			{Panes: []model.Pane{{Cmd: "nvim"}}},
			{Panes: []model.Pane{{Cmd: "opencode"}}},
		},
	}
	ObserveForks(st, p3)
	ak := WindowForkKey(ToShape(p3, "t").Windows[1])
	if st.ForkHits(ak) < 1 {
		t.Fatal("opencode fork not learned")
	}
	if ak == ek {
		t.Fatal("agent != editor")
	}
}
