package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectTypesGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ms := DetectTypes(dir)
	ok := false
	for _, m := range ms {
		if m.Type == "go" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("want go, got %+v", ms)
	}
	if !IsDetectedProject(dir) {
		t.Fatal("IsDetectedProject")
	}
}

func TestDetectTypesGodot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.godot"), []byte("; godot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsDetectedProject(dir) {
		t.Fatalf("godot: %+v", DetectTypes(dir))
	}
}

func TestListSubmodules(t *testing.T) {
	dir := t.TempDir()
	mod := `[submodule "a"]
	path = libs/a
	url = https://example.com/a.git
[submodule "b"]
	path = libs/b
	url = https://example.com/a.git
`
	if err := os.WriteFile(filepath.Join(dir, ".gitmodules"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	sms := ListSubmodules(dir)
	if len(sms) != 2 {
		t.Fatalf("got %+v", sms)
	}
	shared := SharedSubmoduleURLs(dir)
	if len(shared["https://example.com/a.git"]) != 2 {
		t.Fatalf("shared %+v", shared)
	}
}
