//go:build darwin || linux || windows

package app

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// The frames of the selection live in the gfx scene: a pan moves them with the content, and a zoom only
// widens the stroke so it stays two points on the screen.
func TestSelectionPictureFollowsTheScene(t *testing.T) {
	sv := newSceneView(nil, nil)
	sv.off = gfx.Point{X: 10, Y: 20}
	sv.byPath["a"] = &scene.Placed{X: 100, Y: 100, W: 50, H: 40}

	var prevBounds gfx.Rect
	for _, z := range []float64{1, 0.5, 0.25} {
		pic := sv.selectionPicture([]string{"a", "gone"}, z)
		if pic.Len() == 0 {
			t.Fatalf("zoom %v: empty picture", z)
		}
		b := pic.Bounds
		// the stroke of 2/zoom points adds under 3/zoom (miter corners) around the rect
		slack := 3 / z
		near := func(a, b float64) bool { return a-b < slack && b-a < slack }
		if !near(b.Min.X, 110) || !near(b.Min.Y, 120) || !near(b.W(), 50) || !near(b.H(), 40) {
			t.Fatalf("zoom %v: bounds %v, frame at 110,120 size 50 x 40", z, b)
		}
		if z != 1 && b == prevBounds {
			t.Fatalf("zoom %v: picture did not change with the stroke", z)
		}
		prevBounds = b
	}
}

// The rubber band and the line of a link being drawn are in the same scene coordinates.
func TestSelectionPicturePreviewGeometry(t *testing.T) {
	sv := newSceneView(nil, nil)
	band := gfx.R(30, 40, 20, 10)
	sv.band = &band
	line := [2]gfx.Point{{X: 1, Y: 2}, {X: 100, Y: 200}}
	sv.line = &line
	pic := sv.selectionPicture(nil, 1)
	want := gfx.R(1, 2, 99, 198).Union(band)
	b := pic.Bounds
	if b.Min.X > want.Min.X+3 || b.Min.Y > want.Min.Y+3 || b.Max.X < want.Max.X-3 || b.Max.Y < want.Max.Y-3 {
		t.Fatalf("bounds %v, want the band %v and the line %v", b, band, want)
	}
}
