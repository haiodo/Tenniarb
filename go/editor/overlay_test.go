package editor

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

func svgOf(t *testing.T, f *fixture) string {
	t.Helper()
	var b strings.Builder
	if err := f.s.Overlay().WriteSVG(&b, 1); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestOverlayFollowsSelection(t *testing.T) {
	f := newFixture(t)
	if f.s.Overlay().Len() != 0 {
		t.Error("nothing selected, nothing drawn")
	}
	f.click(f.centre(f.a), 0)
	one := f.s.Overlay()
	if one.Len() == 0 {
		t.Fatal("a selected item is outlined")
	}
	f.click(f.centre(f.b), ModMeta)
	if f.s.Overlay().Len() <= one.Len() {
		t.Error("two outlines are more than one")
	}
	f.s.Select([]*model.DiagramItem{f.s.element.Items[3]})
	if !strings.Contains(svgOf(t, f), "stroke-dasharray") {
		t.Error("a selected link is outlined dashed")
	}
	f.s.Select(nil)
	if f.s.Overlay().Len() != 0 {
		t.Error("selection cleared")
	}
}

func TestOverlayDoesNotTouchTheScene(t *testing.T) {
	f := newFixture(t)
	scene := f.s.Scene()
	f.s.Select([]*model.DiagramItem{f.a})
	f.s.Overlay()
	f.s.SetZoom(2)
	f.s.Overlay()
	if f.s.Scene() != scene {
		t.Error("the selection must not rebuild the scene")
	}
	for _, r := range f.redraws {
		if r.Rebuilt {
			t.Error("selection is an overlay-only redraw")
		}
	}
}

func TestOverlayBandAndEditing(t *testing.T) {
	f := newFixture(t)
	f.s.MouseDown(MouseEvent{P: render.Point{X: -50, Y: -50}, Mods: ModShift})
	f.s.MouseMove(MouseEvent{P: render.Point{X: 20, Y: 20}, Mods: ModShift})
	if f.s.Overlay().Len() == 0 {
		t.Error("the band is drawn")
	}
	f.s.MouseUp(MouseEvent{P: render.Point{X: 20, Y: 20}, Mods: ModShift})
	blue := f.s.Overlay()
	f.s.BeginEdit(f.a, EditName)
	if f.s.Overlay().Equal(blue) {
		t.Error("the item being edited is outlined green")
	}
}

func TestRedrawReportsRebuild(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	f.key("Delete", 0)
	rebuilt := false
	for _, r := range f.redraws {
		rebuilt = rebuilt || r.Rebuilt
	}
	if !rebuilt {
		t.Errorf("a committed change rebuilds the scene: %+v", f.redraws)
	}
}

func TestIndicatorDots(t *testing.T) {
	visible := render.Rect{X: 0, Y: 0, Width: 300, Height: 200}
	in := render.Rect{X: 10, Y: 10, Width: 20, Height: 20}
	if dots := IndicatorDots([]render.Rect{in}, visible); len(dots) != 0 {
		t.Errorf("an item in view has no dot: %v", dots)
	}
	left := render.Rect{X: -100, Y: 100, Width: 20, Height: 20}
	right := render.Rect{X: 400, Y: 100, Width: 20, Height: 20}
	dots := IndicatorDots([]render.Rect{left, right, right}, visible)
	if len(dots) != 2 {
		t.Fatalf("dots %v", dots)
	}
	if dots[0].X != 2 || dots[0].Count != 1 {
		t.Errorf("left dot %+v", dots[0])
	}
	if dots[1].Count != 2 || dots[1].Size <= dots[0].Size {
		t.Errorf("two items make a bigger dot on the right: %+v", dots[1])
	}
	// Above the view: a dot in the top row, near the item's column.
	above := render.Rect{X: 150, Y: 500, Width: 20, Height: 20}
	d := IndicatorDots([]render.Rect{above}, visible)
	if len(d) != 1 || d[0].Y < visible.Height-20 {
		t.Errorf("above: %+v", d)
	}
	// Past a corner it counts towards a side.
	corner := render.Rect{X: 5000, Y: 5000, Width: 20, Height: 20}
	d = IndicatorDots([]render.Rect{corner}, visible)
	if len(d) != 1 || d[0].X < visible.Width-20 {
		t.Errorf("corner: %+v", d)
	}
}

func TestIndicatorsPicture(t *testing.T) {
	f := newFixture(t)
	far := render.Rect{X: 5000, Y: 5000, Width: 100, Height: 100}
	pic := f.s.Indicators(far, 1, false)
	if pic.Len() == 0 {
		t.Error("everything is off screen: dots expected")
	}
	all := render.Rect{X: -1000, Y: -1000, Width: 3000, Height: 3000}
	if f.s.Indicators(all, 1, true).Len() != 0 {
		t.Error("everything in view: no dots")
	}
}
