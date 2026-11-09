package editor

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Covers lets the window veto a hit: the box go/render gives an item can reach where go/scene draws
// nothing, and the press there must not grab the item.
func TestCoversVetoesAPress(t *testing.T) {
	m := model.ReadTenn(testSrc)
	if m == nil {
		t.Fatal("source does not parse")
	}
	veto := false
	s, err := New(m.Elements[0], Options{
		Render:  render.Options{Measurer: gfx.EstimateText{}, NoEvaluate: true},
		Covers:  func(r render.Rect, it *model.DiagramItem) bool { return !veto },
		OnError: func(err error) { t.Error(err) },
	})
	if err != nil {
		t.Fatal(err)
	}
	a := m.Elements[0].Items[0]
	d := s.Scene().Drawables[a]
	if d == nil {
		t.Fatal("the item has no drawable")
	}
	b := d.Bounds()
	at := render.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
	if hits := s.Hit(at); len(hits) == 0 || hits[0] != a {
		t.Fatalf("the press on the box hits %v", hits)
	}
	if !s.MouseDown(MouseEvent{P: at, Clicks: 1}) {
		t.Fatal("the press on the box grabbed nothing")
	}
	s.MouseUp(MouseEvent{P: at})

	veto = true
	if hits := s.Hit(at); len(hits) != 0 {
		t.Fatalf("the picture shows nothing there, the hit test finds %v", hits)
	}
	if s.MouseDown(MouseEvent{P: at, Clicks: 1}) {
		t.Fatal("the press was taken though the picture shows nothing")
	}
	s.MouseUp(MouseEvent{P: at})
	if sel := s.Selection(); len(sel) != 0 {
		t.Fatalf("the press selected %v", sel)
	}
	// The right click follows the picture too.
	s.Pick(at)
	if sel := s.Selection(); len(sel) != 0 {
		t.Fatalf("the right click selected %v", sel)
	}
	// A band over the item does not take it either.
	s.MouseDown(MouseEvent{P: render.Point{X: b.X - 20, Y: b.Y - 20}, Mods: ModShift})
	s.MouseMove(MouseEvent{P: render.Point{X: b.X + b.Width + 20, Y: b.Y + b.Height + 20}})
	if sel := s.Selection(); len(sel) != 0 {
		t.Fatalf("the band selected %v", sel)
	}
	s.MouseUp(MouseEvent{P: render.Point{X: b.X + b.Width + 20, Y: b.Y + b.Height + 20}, Mods: ModShift})
}
