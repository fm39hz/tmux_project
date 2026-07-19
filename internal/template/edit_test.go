package template

import (
	"strings"
	"testing"
)

func TestParseSplitAndLegacyLayout(t *testing.T) {
	a, err := Parse(`{"name":"s","windows":[{"name":"w","split":"even-horizontal","panes":[{},{}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if a.Windows[0].Layout != "even-horizontal" {
		t.Fatalf("%q", a.Windows[0].Layout)
	}
	b, err := Parse(`{"name":"s","windows":[{"name":"w","layout":"tiled","panes":[{},{}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if b.Windows[0].Layout != "tiled" {
		t.Fatalf("legacy %q", b.Windows[0].Layout)
	}
	out := Format(a)
	if !strings.Contains(out, `"split"`) {
		t.Fatalf("want split key: %s", out)
	}
	if strings.Contains(out, `"layout"`) {
		t.Fatalf("must not emit layout key: %s", out)
	}
}

func TestParseClassifiesDump(t *testing.T) {
	raw := `{"name":"s","windows":[{"name":"shell","layout":"4080,158x35,0,0[158x17,0,0,1,158x17,0,18,2]","panes":[{},{}]}]}`
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Windows[0].Layout != "even-vertical" {
		t.Fatalf("got %q", p.Windows[0].Layout)
	}
	out := Format(p)
	if strings.Contains(out, "4080") || strings.Contains(out, `"layout"`) {
		t.Fatalf("must not emit dump/layout: %s", out)
	}
	if !strings.Contains(out, `"split": "even-vertical"`) {
		t.Fatalf("want split even-vertical: %s", out)
	}
}
