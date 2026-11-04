//go:build darwin || linux || windows

package app_test

import (
	"os"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// TestEditKeepsCamera: an edit that grows the element bounds leaves the camera where it was. A camera
// that follows the bounds in the same turn as the rebuild showed a frame of the jump before it moved.
func TestEditKeepsCamera(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	defer h.open(fixture)
	h.open(fixture)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Basic steps").In(qa.ID("outline"))).Click()
	a.Settle()

	// the leftmost item: moving it further left grows the bounds, which is what the map is built from
	st := editor(t, a)
	left := "Item"
	for name, b := range st.Screen {
		if b[0] < st.Screen[left][0] {
			left = name
		}
	}
	x, y := center(st.Screen[left])
	a.Find(qa.ID("canvas")).DragTo(x, y, x-40, y)
	st2 := editor(t, a)
	if moved := st2.Screen[left]; moved[0] >= st.Screen[left][0] {
		t.Fatalf("%s did not move: %v -> %v", left, st.Screen[left], moved)
	}
	if st2.OriginX != st.OriginX || st2.OriginY != st.OriginY || st2.Zoom != st.Zoom {
		t.Fatalf("the camera moved: %v,%v@%v -> %v,%v@%v", st.OriginX, st.OriginY, st.Zoom, st2.OriginX, st2.OriginY, st2.Zoom)
	}
}

// TestWheelShiftPansSideways: a mouse wheel has no horizontal axis, so Shift+wheel pans the canvas
// sideways and leaves the vertical position alone.
func TestWheelShiftPansSideways(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	defer h.open(fixture)
	h.open(fixture)
	a.State("command", "name", "close-others")
	a.Settle()
	before := editor(t, a)
	a.Find(qa.ID("canvas")).ScrollEvent(qa.ScrollEvent{X: 200, Y: 200, DY: 3, Mods: "Shift"})
	a.Settle()
	after := editor(t, a)
	if after.OriginX == before.OriginX || after.OriginY != before.OriginY {
		t.Fatalf("Shift+wheel panned to %v,%v, from %v,%v", after.OriginX, after.OriginY, before.OriginX, before.OriginY)
	}
}

// TestSwitchKeepsCamera: an element the outline switched away from keeps its viewport, and an element it
// switches to for the first time is shown at the zoom left behind, not at its own fit.
func TestSwitchKeepsCamera(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	defer h.open(fixture)
	h.open(fixture)
	a.State("command", "name", "close-others")
	a.Settle()
	// The tests share one window: leave it on the root element, as they find it.
	defer func() {
		a.State("select", "path", fixture, "nodes", []string{})
		a.Settle()
	}()
	a.Find(qa.Text("Basic steps").In(qa.ID("outline"))).Click()
	a.Settle()
	a.State("zoom", "factor", 1.5)
	a.Settle()
	first := editor(t, a)
	a.Find(qa.Text("General info").In(qa.ID("outline"))).Click()
	a.Settle()
	elsewhere := editor(t, a)
	if elsewhere.Zoom != first.Zoom {
		t.Fatalf("the switch changed the zoom: %v -> %v", first.Zoom, elsewhere.Zoom)
	}
	a.Find(qa.Text("Basic steps").In(qa.ID("outline"))).Click()
	a.Settle()
	back := editor(t, a)
	if back.Zoom != first.Zoom || back.OriginX != first.OriginX || back.OriginY != first.OriginY {
		t.Fatalf("the viewport of the element left changed: %v,%v@%v -> %v,%v@%v",
			first.OriginX, first.OriginY, first.Zoom, back.OriginX, back.OriginY, back.Zoom)
	}
}
