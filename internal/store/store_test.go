package store

import "testing"

func TestSaveOverwritesAliasAndCwd(t *testing.T) {
	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// isolate: use unique cwd under temp — but store is real user db.
	// Use distinctive names that won't collide with user presets.
	a := &Preset{
		Name: "zztestalias",
		Cwd:  "/tmp/gotomux-save-test-root",
		Windows: []PresetWindow{
			{Name: "w", Panes: []PresetPane{{Cwd: "/tmp/gotomux-save-test-root", Cmd: "true"}}},
		},
	}
	if err := s.Save(a); err != nil {
		t.Fatal(err)
	}
	// legacy-style alias name, same cwd, more panes
	b := &Preset{
		Name: "zz-test-alias", // different spelling, same alias key if we strip -
		Cwd:  "/tmp/gotomux-save-test-root",
		Windows: []PresetWindow{
			{Name: "w1", Panes: []PresetPane{{Cwd: "/tmp/gotomux-save-test-root"}}},
			{Name: "w2", Panes: []PresetPane{{Cwd: "/tmp/gotomux-save-test-root"}}},
		},
	}
	// force alias collision: zztestalias vs zztestalias after strip of zz-test-alias
	// sessionAliasKey("zz-test-alias") == "zztestalias"
	if sessionAliasKey("zz-test-alias") != sessionAliasKey("zztestalias") {
		t.Fatalf("alias keys %q %q", sessionAliasKey("zz-test-alias"), sessionAliasKey("zztestalias"))
	}
	if err := s.Save(b); err != nil {
		t.Fatal(err)
	}
	// old name must be gone
	if _, err := s.Get("zztestalias"); err == nil {
		t.Fatal("legacy alias row should be deleted")
	}
	got, err := s.Get("zz-test-alias")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 2 {
		t.Fatalf("want 2 windows after overwrite, got %d", len(got.Windows))
	}
	_ = s.Delete("zz-test-alias")
}

func TestSessionAliasKey(t *testing.T) {
	if sessionAliasKey("tmux-project") != sessionAliasKey("tmuxproject") {
		t.Fatal("expected alias match")
	}
	if sessionAliasKey("tmux_project") == sessionAliasKey("tmux-project") {
		// _ already removed in key; both become tmuxproject
	}
	if sessionAliasKey("tmux_project") != "tmuxproject" {
		t.Fatalf("got %q", sessionAliasKey("tmux_project"))
	}
}

