package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
)

// Several paths of one item: each is drawn, and a command that names its own ink keeps it - the tag is an
// outlined contour with a filled dot, which one path cannot be.
func TestSeveralPathsOfAnItem(t *testing.T) {
	s := multipathSVG(t)
	if !strings.Contains(s, "rgb(255,0,0)") {
		t.Errorf("the fill of the second path is not drawn:\n%s", s)
	}
	if !strings.Contains(s, "rgb(17,34,51)") {
		t.Errorf("the stroke of the item is not drawn:\n%s", s)
	}
	// The item writes one border and one line-width; the dot names only a fill, so it is filled and not
	// stroked: two shapes of the tag, not one.
	if n := strings.Count(s, "<path"); n < 3 {
		t.Errorf("%d paths drawn, want at least three (two of the tag, one of each other icon):\n%s", n, s)
	}
}

// line-cap and line-join reach the stroke of a shape and the border of an item.
func TestStrokeEndsAndCorners(t *testing.T) {
	s := multipathSVG(t)
	if !strings.Contains(s, `stroke-linecap="round"`) {
		t.Errorf("no round cap in:\n%s", s)
	}
	if !strings.Contains(s, `stroke-linejoin="round"`) {
		t.Errorf("no round join in:\n%s", s)
	}
	// The item that asks for nothing keeps the defaults: no attribute, which is butt and miter. The groups
	// of the svg come in the order of the fixture, and Plain is the last of them.
	groups := strings.Split(s, "<g ")
	plain := groups[len(groups)-1]
	if strings.Contains(plain, "stroke-linecap") || strings.Contains(plain, "stroke-linejoin") {
		t.Errorf("a plain item named an end:\n%s", plain)
	}
}

// multipathSVG draws the fixture of the several paths of an item.
func multipathSVG(t *testing.T) string {
	t.Helper()
	screen, sc := build(t, load(t, "multipath.tenn"), "Paths")
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
