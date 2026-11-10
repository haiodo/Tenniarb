package app

import (
	"math"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scenegeom"
)

func TestSceneMapRoundTrip(t *testing.T) {
	sm := SceneMap{TX: 33, TY: 70}
	p := geometry.Point{X: -12.5, Y: 40}
	g := sm.ToGfx(p)
	if back := sm.ToScene(g); back != p {
		t.Fatalf("round trip %v -> %v -> %v", p, g, back)
	}
	if g.Y >= sm.TY || sm.ToGfx(geometry.Point{X: 0, Y: 41}).Y >= g.Y {
		t.Fatal("y must grow upwards in the editor scene")
	}
}

func TestFollowKeepsScreenPosition(t *testing.T) {
	cam := gfx.Camera{Origin: gfx.Point{X: 10, Y: -5}, Zoom: 2}
	old, next := SceneMap{TX: 15, TY: 100}, SceneMap{TX: 55, TY: 130}
	p := geometry.Point{X: 7, Y: 9}
	screen := func(m SceneMap, c gfx.Camera) gfx.Point {
		g := m.ToGfx(p)
		return gfx.Point{X: (g.X - c.Origin.X) * c.Zoom, Y: (g.Y - c.Origin.Y) * c.Zoom}
	}
	before := screen(old, cam)
	after := screen(next, Follow(cam, next.Shift(old)))
	if math.Abs(before.X-after.X) > 1e-9 || math.Abs(before.Y-after.Y) > 1e-9 {
		t.Fatalf("moved on screen: %v -> %v", before, after)
	}
}

func TestViewToScene(t *testing.T) {
	cam := gfx.Camera{Origin: gfx.Point{X: 10, Y: 20}, Zoom: 4}
	if p := ViewToScene(cam, gfx.Point{X: 8, Y: 40}); p != (gfx.Point{X: 12, Y: 30}) {
		t.Fatalf("%v", p)
	}
}

// A click selects without a rebuild that moves the content: the map, and so the camera, stay.
func TestSelectionKeepsSceneMap(t *testing.T) {
	m := model.ReadTenn("element \"D\" {\n item \"A\" { pos -120 40 }\n item \"B\" { pos 200 -90 }\n}\n")
	s, err := editor.New(m.Elements[0], editor.Options{Geom: scenegeom.New(nil, m.Elements[0], gfx.EstimateText{}), NoEvaluate: true})
	if err != nil {
		t.Fatal(err)
	}
	before := MapOf(s.Geometry().Bounds())
	for _, it := range m.Elements[0].Items {
		s.Select([]*model.DiagramItem{it})
		if got := MapOf(s.Geometry().Bounds()); got != before {
			t.Fatalf("selecting %s changed the map %+v -> %+v", it.Name, before, got)
		}
	}
}
