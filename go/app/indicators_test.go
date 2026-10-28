package app

import (
	"testing"

	"github.com/haiodo/gowt/gfx"
)

func TestEdgeDots(t *testing.T) {
	cam := gfx.Camera{Origin: gfx.Point{X: 100, Y: 50}, Zoom: 2} // shows x 100..400, y 50..250
	view := func(bounds ...gfx.Rect) int { return len(EdgeDots(bounds, cam, 600, 400)) }

	if n := view(gfx.R(150, 100, 40, 40)); n != 0 {
		t.Errorf("an item in view has no dot: %d", n)
	}
	if n := view(); n != 0 {
		t.Errorf("no items, no dots: %d", n)
	}
	if got := EdgeDots([]gfx.Rect{gfx.R(0, 100, 20, 20)}, gfx.Camera{}, 600, 400); got != nil {
		t.Errorf("no zoom, no dots: %v", got)
	}

	left := EdgeDots([]gfx.Rect{gfx.R(0, 100, 20, 20)}, cam, 600, 400)
	if len(left) != 1 || left[0].X != 2 || left[0].Count != 1 {
		t.Fatalf("left: %+v", left)
	}
	right := EdgeDots([]gfx.Rect{gfx.R(500, 100, 20, 20), gfx.R(600, 100, 20, 20)}, cam, 600, 400)
	if len(right) != 1 || right[0].Count != 2 || right[0].X < 300-15 {
		t.Fatalf("right: %+v", right)
	}
	// y is down on the gfx scene: an item above the view lands in the top row, which is the high Y of the dots.
	above := EdgeDots([]gfx.Rect{gfx.R(250, -300, 20, 20)}, cam, 600, 400)
	if len(above) != 1 || above[0].Y < 200-15 {
		t.Fatalf("above: %+v", above)
	}
	below := EdgeDots([]gfx.Rect{gfx.R(250, 900, 20, 20)}, cam, 600, 400)
	if len(below) != 1 || below[0].Y > 10 {
		t.Fatalf("below: %+v", below)
	}
	// Partly in view is not off screen.
	if n := view(gfx.R(90, 100, 20, 20)); n != 0 {
		t.Errorf("an item that crosses the edge has no dot: %d", n)
	}
}
