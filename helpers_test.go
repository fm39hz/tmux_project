package main

import "testing"

func TestFuzzyUTF8(t *testing.T) {
	if !fuzzyMatch("thư", "thư mục") {
		t.Fatal("utf8 miss")
	}
	if fuzzyMatch("xyz", "abc") {
		t.Fatal("false positive")
	}
}
func TestTruncateRunes(t *testing.T) {
	s := truncateRunes("nhà cửa đẹp", 5)
	if s != "nhà …" && len([]rune(s)) > 5 {
		t.Fatalf("got %q", s)
	}
}
func TestValidSessionName(t *testing.T) {
	if validSessionName("") || validSessionName("a:b") || !validSessionName("foo-bar") {
		t.Fatal("validSessionName")
	}
}
func TestCmdArgs(t *testing.T) {
	got := cmdArgs("nvim foo")
	if len(got) != 2 || got[0] != "nvim" {
		t.Fatalf("%v", got)
	}
}
