package scene_test

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// frac has a fractional line box: 8.3 + 9.1 = 17.4 rounds to 17, as gfx/pen does, and ceils to 18.
type frac struct{}

func (frac) MeasureText(s string, f gfx.Font) gfx.TextMetrics {
	return gfx.TextMetrics{Width: 10 * float64(len([]rune(s))), Ascent: 8.3, Descent: 9.1}
}

func buildSrc(t *testing.T, src string) (*scene.Screen, string) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}}) })
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

func boxH(s *scene.Screen, path string) float64 {
	for _, b := range s.Boxes() {
		if b.Path == path {
			return b.H
		}
	}
	return -1
}

func TestLineBoxIsRounded(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "A" { title "x" }
    item "B" { title "x"; line-height 30 }
    item "C" { title "x"; line-height 30; line-spacing 2 }
}`)
	for path, want := range map[string]float64{"A": 17 + 2*6, "B": 30 + 2*6, "C": 32 + 2*6} {
		if got := boxH(s, path); got != want {
			t.Errorf("%s height %v, want %v", path, got, want)
		}
	}
}

var textX = regexp.MustCompile(`<text[^>]* x="([-0-9.]+)"`)

func TestLeafTextLayout(t *testing.T) {
	_, svg := buildSrc(t, `element "E" {
    item "L" { pos 0 0; width 100; height 30; title "ab"; layout left }
    item "M" { pos 0 40; width 100; height 30; title "ab" }
    item "R" { pos 0 80; width 100; height 30; title "ab"; layout right }
}`)
	m := textX.FindAllStringSubmatch(svg, -1)
	if len(m) != 3 {
		t.Fatalf("want 3 texts in %s", svg)
	}
	var got []float64
	for _, x := range m {
		v, _ := strconv.ParseFloat(x[1], 64)
		got = append(got, v)
	}
	// text width 20; left is at the padding, right ends at the padding from the edge
	if got[0] >= got[1] || got[1] >= got[2] || got[1] != 40 {
		t.Errorf("x of left, middle, right: %v", got)
	}
}
