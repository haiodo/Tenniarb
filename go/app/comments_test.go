package app

import (
	"testing"
	"time"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/comments"
)

var pinElement = []string{"Login"}

func pinBoxes(m map[string]gfx.Rect) func(string) (gfx.Rect, bool) {
	return func(n string) (gfx.Rect, bool) { r, ok := m[n]; return r, ok }
}

// off is the content origin on the gfx scene.
func pinOrigin(p comments.Point) gfx.Point { return gfx.Point{X: 20 + p.X, Y: 30 + p.Y} }

func TestPlacePins(t *testing.T) {
	boxes := pinBoxes(map[string]gfx.Rect{"card": gfx.R(100, 200, 80, 40)})
	cs := []comments.Comment{
		{ID: "a", State: comments.Open, Anchor: comments.Anchor{Element: pinElement, Node: "card", Point: &comments.Point{X: 5, Y: 6}}},
		{ID: "b", State: comments.Open, Anchor: comments.Anchor{Element: pinElement, Node: "card"}},
		{ID: "c", State: comments.Open, Anchor: comments.Anchor{Element: pinElement, Point: &comments.Point{X: 7, Y: 8}}},
		{ID: "d", State: comments.Open, Anchor: comments.Anchor{Element: pinElement, Node: "gone", Point: &comments.Point{X: 9, Y: 9}}},
		{ID: "e", State: comments.Resolved, Anchor: comments.Anchor{Element: pinElement, Node: "card"}},
		{ID: "f", State: comments.Open, Anchor: comments.Anchor{Element: []string{"Other"}, Node: "card"}},
	}
	got := PlacePins(cs, pinElement, false, boxes, pinOrigin)
	want := []Pin{
		{N: 1, ID: "a", At: gfx.Point{X: 105, Y: 206}},
		{N: 2, ID: "b", At: gfx.Point{X: 180, Y: 200}}, // no point: the top right corner of the box
		{N: 3, ID: "c", At: gfx.Point{X: 27, Y: 38}},   // no node: from the content origin
		{N: 4, ID: "d", At: gfx.Point{X: 20, Y: 30}, Detached: true},
	}
	if len(got) != len(want) {
		t.Fatalf("pins %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pin %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	// Resolved ones come back on request and keep the numbers of the others.
	all := PlacePins(cs, pinElement, true, boxes, pinOrigin)
	if len(all) != 5 || all[4].ID != "e" || all[4].N != 5 || !all[4].Resolved {
		t.Errorf("with resolved: %+v", all)
	}
}

func TestAnchorAt(t *testing.T) {
	toElement := func(p gfx.Point) comments.Point { return comments.Point{X: p.X - 20, Y: p.Y - 30} }
	a := AnchorAt(pinElement, "card", gfx.R(100, 200, 80, 40), gfx.Point{X: 110, Y: 215}, toElement)
	if a.Node != "card" || a.Point == nil || *a.Point != (comments.Point{X: 10, Y: 15}) {
		t.Fatalf("on a node: %+v", a)
	}
	a = AnchorAt(pinElement, "", gfx.Rect{}, gfx.Point{X: 50, Y: 60}, toElement)
	if a.Node != "" || a.Point == nil || *a.Point != (comments.Point{X: 30, Y: 30}) {
		t.Fatalf("on empty space: %+v", a)
	}
	// The pin lands where the click was, on the node and off it.
	boxes := pinBoxes(map[string]gfx.Rect{"card": gfx.R(100, 200, 80, 40)})
	pins := PlacePins([]comments.Comment{{ID: "x", State: comments.Open, Anchor: AnchorAt(pinElement, "card", gfx.R(100, 200, 80, 40), gfx.Point{X: 110, Y: 215}, toElement)}}, pinElement, false, boxes, pinOrigin)
	if len(pins) != 1 || pins[0].At != (gfx.Point{X: 110, Y: 215}) {
		t.Fatalf("round trip: %+v", pins)
	}
}

func TestPinAtView(t *testing.T) {
	cam := gfx.Camera{Origin: gfx.Point{X: 10, Y: 10}, Zoom: 2}
	pins := []Pin{{N: 1, ID: "a", At: gfx.Point{X: 50, Y: 60}}, {N: 2, ID: "b", At: gfx.Point{X: 50, Y: 60}}}
	c := pinView(pins[0], cam)
	if c.X != 80+pinRadius*0.6 || c.Y != 100-pinRadius*0.6 {
		t.Fatalf("marker centre %+v", c)
	}
	if p, ok := PinAtView(pins, cam, c); !ok || p.ID != "b" {
		t.Fatalf("the pin drawn last wins: %+v %v", p, ok)
	}
	if _, ok := PinAtView(pins, cam, gfx.Point{X: c.X + pinRadius + 2, Y: c.Y}); ok {
		t.Fatal("a click beside the marker hit it")
	}
}

func TestCommentLabels(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local)
	if got := commentWhen(time.Date(2026, 10, 28, 9, 5, 0, 0, time.Local), now); got != "09:05" {
		t.Errorf("today: %q", got)
	}
	if got := commentWhen(time.Date(2026, 10, 27, 9, 5, 0, 0, time.Local), now); got != "27 Oct 09:05" {
		t.Errorf("before: %q", got)
	}
	c := comments.Comment{Author: "user", Text: "  too\n tight  ", State: comments.Resolved}
	if got := commentLabel(3, c); got != "3  user: too tight (resolved)" {
		t.Errorf("label %q", got)
	}
}
