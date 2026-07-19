package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChildrenGitmodulesAndMarkers(t *testing.T) {
	root := t.TempDir()
	// gitmodules style children
	_ = os.WriteFile(filepath.Join(root, ".gitmodules"), []byte(`
[submodule "cong"]
	path = cong-dlqg
	url = x
[submodule "kho"]
	path = kho-dl-mo
	url = y
`), 0o644)
	for _, d := range []string{"cong-dlqg", "kho-dl-mo", "node_modules", "docs"} {
		_ = os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	// marker-only child
	_ = os.MkdirAll(filepath.Join(root, "apps-fe"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "apps-fe", "package.json"), []byte(`{}`), 0o644)
	// junk
	_ = os.MkdirAll(filepath.Join(root, "node_modules", "x"), 0o755)

	ch := Children(root)
	// gitmodules first in file order
	if len(ch) < 2 {
		t.Fatalf("children %v", ch)
	}
	if filepath.Base(ch[0]) != "cong-dlqg" || filepath.Base(ch[1]) != "kho-dl-mo" {
		t.Fatalf("gitmodules order: %v", ch)
	}
	// apps-fe included, node_modules not
	bases := map[string]bool{}
	for _, c := range ch {
		bases[filepath.Base(c)] = true
	}
	if !bases["apps-fe"] {
		t.Fatalf("marker child missing: %v", ch)
	}
	if bases["node_modules"] || bases["docs"] {
		// docs may not be project root without markers — ok if absent
	}
	if bases["node_modules"] {
		t.Fatal("node_modules must skip")
	}
}

func TestChildrenFlatEmpty(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644)
	if ch := Children(root); len(ch) != 0 {
		t.Fatalf("flat: %v", ch)
	}
}
