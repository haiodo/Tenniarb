package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// buildMarkup lays out a classic diagram with the estimating measurer: its metrics grow with the font
// size, so a run of &(size) makes its line taller.
func buildMarkup(t *testing.T, src string) (*scene.Screen, string) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: gfx.EstimateText{}})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return screen, out.String()
}

func boxW(s *scene.Screen, path string) float64 {
	for _, b := range s.Boxes() {
		if b.Path == path {
			return b.W
		}
	}
	return -1
}

// A title of a classic diagram draws its !(...) runs in their colours, the markup itself not at all, and
// the runs add up to the width of the same text written plain.
func TestClassicTitleColorMarkup(t *testing.T) {
	s, svg := buildMarkup(t, `element "E" {
  item A { title "!(red|IN)!(green|tegrations)" }
  item B { title "INtegrations" }
  item C { title "!(#ff0000|X)" }
}`)
	if w, want := boxW(s, "A"), boxW(s, "B"); w != want {
		t.Errorf("coloured title width %v, plain width %v", w, want)
	}
	for _, part := range []string{
		`fill="rgb(255,0,0)">IN<`,
		`fill="rgb(0,128,0)">tegrations<`,
		`fill="rgb(255,0,0)">X<`,
	} {
		if !strings.Contains(svg, part) {
			t.Errorf("missing %q in:\n%s", part, svg)
		}
	}
	if strings.Contains(svg, "!(") || strings.Contains(svg, "|") {
		t.Errorf("the markup is drawn:\n%s", svg)
	}
}

// A run of &(size) sets its line height and the box of the item above the one of a plain title.
func TestClassicTitleFontMarkupGrowsLine(t *testing.T) {
	s, svg := buildMarkup(t, `element "E" {
  item A { title "x" }
  item B { title "&(30|x)" }
}`)
	if hb, ha := boxH(s, "B"), boxH(s, "A"); hb <= ha {
		t.Errorf("sized title height %v, plain height %v: the line did not grow", hb, ha)
	}
	if !strings.Contains(svg, `font-size="30"`) {
		t.Errorf("the run keeps no size of its own:\n%s", svg)
	}
}

// An unknown colour of the markup falls back to the text colour of the item, is reported once and does
// not fail the file.
func TestClassicTitleUnknownColor(t *testing.T) {
	s, svg := buildMarkup(t, `element "E" {
  item A { title "!(nosuch|x)" }
  item B { title "!(nosuch|y)" }
}`)
	n := 0
	for _, w := range s.Warnings {
		if strings.Contains(w.Msg, "no such color") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d warnings about the unknown colour, want 1: %v", n, s.Warnings)
	}
	if strings.Contains(svg, "nosuch") {
		t.Errorf("the markup is drawn:\n%s", svg)
	}
	if !strings.Contains(svg, `fill="rgb(0,0,0)">x<`) {
		t.Errorf("the text is not in the default colour:\n%s", svg)
	}
}
