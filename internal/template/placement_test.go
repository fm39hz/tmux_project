package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fm39hz/gotomux/internal/store"
)

func TestPatternFromPresetUmbrella(t *testing.T) {
	root := t.TempDir()
	c0 := filepath.Join(root, "cong-dlqg")
	c1 := filepath.Join(root, "kho-dl-mo")
	for _, d := range []string{c0, c1} {
		_ = os.MkdirAll(d, 0o755)
		_ = os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	}
	p := &store.Preset{
		Name: "kho-cong", Cwd: root,
		Windows: []store.PresetWindow{
			{Panes: []store.PresetPane{{Cwd: root}}},
			{Panes: []store.PresetPane{
				{Cwd: c0}, {Cwd: c0}, {Cwd: c1}, {Cwd: c1},
			}},
			{Panes: []store.PresetPane{{Cwd: root}}},
		},
	}
	pat := PatternFromPreset(p)
	want := "R|C0,C0,C1,C1|R"
	if pat != want {
		t.Fatalf("got %q want %q", pat, want)
	}
	// all root → empty
	flat := &store.Preset{
		Cwd: root,
		Windows: []store.PresetWindow{
			{Panes: []store.PresetPane{{Cwd: root}}},
		},
	}
	if PatternFromPreset(flat) != "" {
		t.Fatal("trivial must be empty")
	}
}

func TestObserveAndBakePlacement(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "cfg"))
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rootA := filepath.Join(dir, "umbrella-a")
	c0a := filepath.Join(rootA, "fe")
	c1a := filepath.Join(rootA, "be")
	for _, d := range []string{c0a, c1a} {
		_ = os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	}
	inst := &store.Preset{
		Name: "ua", Cwd: rootA,
		Windows: []store.PresetWindow{
			{Name: "ed", Panes: []store.PresetPane{{Cwd: rootA, Cmd: "nvim"}}},
			{Name: "sh", Layout: "tiled", Panes: []store.PresetPane{
				{Cwd: c0a}, {Cwd: c1a},
			}},
		},
	}
	sid, _, err := FreezeSave(st, inst, true)
	if err != nil {
		t.Fatal(err)
	}
	pat, ok := st.BestPlacement(sid)
	if !ok {
		t.Fatal("expected learned placement")
	}
	// children of rootA sorted: be, fe → freeze panes fe,be → C1,C0
	if pat != "R|C1,C0" {
		t.Fatalf("learned %q want R|C1,C0 (alpha children be,fe)", pat)
	}

	rootB := filepath.Join(dir, "umbrella-b")
	be := filepath.Join(rootB, "be")
	fe := filepath.Join(rootB, "fe")
	for _, d := range []string{be, fe} {
		_ = os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	}
	tmpl, _, err := LoadActive(st)
	if err != nil {
		t.Fatal(err)
	}
	baked := bakeShape(st, tmpl, "ub", rootB, sid)
	if len(baked.Windows) != 2 || len(baked.Windows[1].Panes) != 2 {
		t.Fatalf("%+v", baked.Windows)
	}
	if baked.Windows[0].Panes[0].Cwd != rootB {
		t.Fatalf("w0 %s", baked.Windows[0].Panes[0].Cwd)
	}
	// C1,C0 → fe, be
	if baked.Windows[1].Panes[0].Cwd != fe || baked.Windows[1].Panes[1].Cwd != be {
		t.Fatalf("w1 panes %s %s want fe,be",
			baked.Windows[1].Panes[0].Cwd, baked.Windows[1].Panes[1].Cwd)
	}
	// tool intent bakes when present on shape
	_ = baked.Windows[0].Panes[0].Cmd
	// split materialised at bake (not left empty for Load to invent)
	if len(baked.Windows[1].Panes) > 1 && baked.Windows[1].Layout != "even-horizontal" && baked.Windows[1].Layout != "tiled" {
		// default inference is even-horizontal when shape had no named split
		if baked.Windows[1].Layout != "even-horizontal" {
			t.Fatalf("bake must set split, got %q", baked.Windows[1].Layout)
		}
	}
}

func TestBakeMissingChildFallsBackRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.RecordPlacement("shape-x", "R|C0,C1")
	// shape body minimal via Upsert
	body := `{"name":"shape-x","windows":[{"name":"a","panes":[{}]},{"name":"b","panes":[{},{}]}]}`
	_ = st.UpsertShapeByID("shape-x", "keyx", body)
	root := filepath.Join(dir, "only-one")
	_ = os.MkdirAll(filepath.Join(root, "fe", ".git"), 0o755)
	tmpl, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	baked := bakeShape(st, tmpl, "s", root, "shape-x")
	// C0=fe, C1 missing → root
	if baked.Windows[1].Panes[0].Cwd != filepath.Join(root, "fe") {
		t.Fatalf("c0 %s", baked.Windows[1].Panes[0].Cwd)
	}
	if baked.Windows[1].Panes[1].Cwd != root {
		t.Fatalf("c1 fallback %s", baked.Windows[1].Panes[1].Cwd)
	}
	_ = strings.Contains
}
