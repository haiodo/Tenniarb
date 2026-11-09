//go:build darwin || linux || windows

package app

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// The frames of the selection are view-space pictures: a zoom changes them, not the scene.
func TestSelectionPictureLeavesSceneAlone(t *testing.T) {
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		tx.Add(nil, gfx.NewNode(1))
	})
	sc.Commit()
	sv := newSceneView(nil, nil)
	sv.off = gfx.Point{X: 10, Y: 20}
	sv.byPath["a"] = &scene.Placed{X: 100, Y: 100, W: 50, H: 40}

	var prev gfx.Rect
	for _, z := range []float64{1, 0.5, 0.25} {
		cam := gfx.Camera{Origin: gfx.Point{X: 5, Y: 5}, Zoom: z}
		pic := sv.selectionPicture([]string{"a", "gone"}, cam)
		if pic.Len() == 0 {
			t.Fatalf("zoom %v: empty picture", z)
		}
		b := pic.Bounds
		// the stroke of 2 points adds under 3 (miter corners) around the rect
		fx, fy := (100+10-5)*z, (100+20-5)*z
		if near := func(a, b float64) bool { return a-b < 3 && b-a < 3 }; !near(b.Min.X, fx) ||
			!near(b.Min.Y, fy) || !near(b.W(), 50*z) || !near(b.H(), 40*z) {
			t.Fatalf("zoom %v: bounds %v, frame at %v,%v size %v x %v", z, b, fx, fy, 50*z, 40*z)
		}
		if b == prev {
			t.Fatalf("zoom %v: picture did not follow the camera", z)
		}
		prev = b
		if d := sc.Commit(); len(d) != 0 {
			t.Fatalf("zoom %v: scene damaged: %v", z, d)
		}
	}
}

// The rubber band and the line of a link being drawn are drawn in view points over the frames.
func TestSelectionPicturePreviewGeometry(t *testing.T) {
	near := func(a, b float64) bool { return a-b < 3 && b-a < 3 }
	cam := gfx.Camera{Origin: gfx.Point{X: 10, Y: 10}, Zoom: 2}
	sv := newSceneView(nil, nil)

	band := gfx.R(20, 30, 40, 10) // gfx scene: view (20,40) 80x20
	sv.band = &band
	b := sv.selectionPicture(nil, cam).Bounds
	if !near(b.Min.X, 20) || !near(b.Min.Y, 40) || !near(b.W(), 80) || !near(b.H(), 20) {
		t.Errorf("band bounds %v, want 20,40 80x20", b)
	}

	sv.band = nil
	line := [2]gfx.Point{{X: 20, Y: 30}, {X: 60, Y: 30}} // gfx scene: view x 20..100 at y 40
	sv.line = &line
	b = sv.selectionPicture(nil, cam).Bounds
	if !near(b.Min.X, 20) || !near(b.Max.X, 100) || !near((b.Min.Y+b.Max.Y)/2, 40) {
		t.Errorf("line bounds %v, want 20..100 at y 40", b)
	}
}
