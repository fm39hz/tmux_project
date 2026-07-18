package template

import (
	"testing"

	"github.com/fm39hz/gotomux/internal/store"
)

func TestJSONAllowsLayoutDump(t *testing.T) {
	dump := "7efd,158x35,0,0[158x17,0,0,63,158x17,0,18,64]"
	raw := `{"name":"x","windows":[{"name":"w","layout":"` + dump + `","panes":[{"cwd":"/a"},{"cwd":"/b"}]}]}`
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Windows[0].Layout != dump {
		t.Fatalf("layout %q", p.Windows[0].Layout)
	}
}

func TestJSONPresetRoundtrip(t *testing.T) {
	raw := `{
  "name": "demo",
  "cwd": "/tmp",
  "windows": [
    {
      "name": "editor",
      "layout": "even-horizontal",
      "panes": [
        {"cwd": "/tmp", "cmd": "nvim"},
        {"cwd": "/tmp/test"}
      ]
    }
  ]
}`
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "demo" || len(p.Windows) != 1 || len(p.Windows[0].Panes) != 2 {
		t.Fatalf("%+v", p)
	}
	if p.Windows[0].Panes[0].Cmd != "nvim" {
		t.Fatal("cmd")
	}
	out := Format(p)
	p2, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Windows[0].Layout != "even-horizontal" {
		t.Fatalf("layout lost: %q", p2.Windows[0].Layout)
	}
}

func TestApplyTemplate(t *testing.T) {
	tmpl := &store.Preset{
		Name: "default",
		Windows: []store.PresetWindow{
			{Name: "editor", Panes: []store.PresetPane{{Cmd: "nvim"}}},
			{Name: "test", Panes: []store.PresetPane{{Cwd: "test"}, {Cwd: ""}}},
		},
	}
	p := Apply(tmpl, "myproj", "/work/myproj")
	if p.Name != "myproj" || p.Cwd != "/work/myproj" {
		t.Fatalf("root: %+v", p)
	}
	if p.Windows[0].Panes[0].Cwd != "/work/myproj" || p.Windows[0].Panes[0].Cmd != "nvim" {
		t.Fatalf("editor: %+v", p.Windows[0].Panes[0])
	}
	if p.Windows[1].Panes[0].Cwd != "/work/myproj/test" {
		t.Fatalf("rel: %q", p.Windows[1].Panes[0].Cwd)
	}
	if p.Windows[1].Panes[1].Cwd != "/work/myproj" {
		t.Fatalf("empty: %q", p.Windows[1].Panes[1].Cwd)
	}
}

func TestResolveCwd(t *testing.T) {
	if resolveCwd("/a", "") != "/a" {
		t.Fatal("empty")
	}
	if resolveCwd("/a", "b") != "/a/b" {
		t.Fatal("rel")
	}
	if resolveCwd("/a", "/abs") != "/abs" {
		t.Fatal("abs")
	}
}

func TestPresetToTemplate(t *testing.T) {
	p := &store.Preset{
		Name: "fantasia",
		Cwd:  "/work/Fantasia",
		Windows: []store.PresetWindow{
			{Name: "editor", Cwd: "/work/Fantasia", Panes: []store.PresetPane{{Cwd: "/work/Fantasia", Cmd: "nvim"}}},
			{Name: "test", Panes: []store.PresetPane{{Cwd: "/work/Fantasia/test"}, {Cwd: "/work/Fantasia"}}},
		},
	}
	tmpl := presetToTemplate(p)
	if tmpl.Name != "fantasia" {
		t.Fatal(tmpl.Name)
	}
	if tmpl.Windows[0].Panes[0].Cwd != "" || tmpl.Windows[0].Panes[0].Cmd != "nvim" {
		t.Fatalf("editor pane: %+v", tmpl.Windows[0].Panes[0])
	}
	if tmpl.Windows[1].Panes[0].Cwd != "test" {
		t.Fatalf("rel: %q", tmpl.Windows[1].Panes[0].Cwd)
	}
	if tmpl.Windows[1].Panes[1].Cwd != "" {
		t.Fatalf("root pane: %q", tmpl.Windows[1].Panes[1].Cwd)
	}
	// bake
	got := Apply(tmpl, "other", "/proj/other")
	if got.Windows[0].Panes[0].Cwd != "/proj/other" || got.Windows[0].Panes[0].Cmd != "nvim" {
		t.Fatalf("bake editor: %+v", got.Windows[0].Panes[0])
	}
	if got.Windows[1].Panes[0].Cwd != "/proj/other/test" {
		t.Fatalf("bake test: %q", got.Windows[1].Panes[0].Cwd)
	}
}

func TestRelativizeCwd(t *testing.T) {
	if relativizeCwd("/a", "/a/b") != "b" {
		t.Fatal("rel")
	}
	if relativizeCwd("/a", "/a") != "" {
		t.Fatal("root")
	}
	if relativizeCwd("/a", "/other") != "" {
		t.Fatal("outside")
	}
}

// --- ranking (lexicographic rankKey) ---


