package render

import (
	"math"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// Port of DrawableLineTests.swift and the primitive tests of DrawableStyleTests.swift.

func lineStyle(t testing.TB, src string) *DrawableLineStyle {
	s := NewDrawableLineStyle(false)
	if src != "" {
		s.ParseStyle(styleTree(t, src), nil)
	}
	return s
}

func makeLine(from, to Rect, style *DrawableLineStyle) *DrawableLine {
	return NewDrawableLine(from, to, style, Point{})
}

func host() *model.DiagramItem { return model.NewDiagramItem(model.ItemKindItem, "Host") }

var (
	left  = Rect{0, 0, 40, 20}
	right = Rect{200, 0, 40, 20}
	above = Rect{0, 200, 40, 20}
)

func TestLineEndpointsSitBetweenBoxes(t *testing.T) {
	l := makeLine(left, right, NewDrawableLineStyle(false))
	if l.Source != (Point{40, 10}) || l.Target != (Point{200, 10}) {
		t.Errorf("endpoints %+v %+v", l.Source, l.Target)
	}
}

func TestLineFollowsVerticalArrangementAndMovedTarget(t *testing.T) {
	l := makeLine(left, above, NewDrawableLineStyle(false))
	if !(l.Source.Y < l.Target.Y) {
		t.Error("bottom to top")
	}
	h := makeLine(left, right, NewDrawableLineStyle(false))
	before := h.Target
	h.UpdateLayout(left, Rect{500, 0, 40, 20})
	if h.Target == before {
		t.Error("moving the target must move the endpoint")
	}
}

func TestLinePointConstructorKeepsCoordinates(t *testing.T) {
	l := NewDrawableLineBetween(Point{1, 2}, Point{3, 4}, lineStyle(t, ""))
	if l.Source != (Point{1, 2}) || l.Target != (Point{3, 4}) || len(l.ExtraPoints) != 0 {
		t.Errorf("%+v", l)
	}
}

func TestLineControlPointBendsThroughOneExtraPoint(t *testing.T) {
	l := NewDrawableLine(left, right, lineStyle(t, ""), Point{0, 50})
	if len(l.ExtraPoints) != 1 || l.ExtraPoints[0] != (Point{120, 60}) {
		t.Errorf("extra %+v", l.ExtraPoints)
	}
}

func TestLineUpdateLayoutClearsWaypoints(t *testing.T) {
	style := lineStyle(t, "layout middle")
	l := makeLine(Rect{0, 0, 40, 20}, Rect{10, 200, 40, 20}, style)
	if len(l.ExtraPoints) == 0 {
		t.Fatal("the middle layout routes around the boxes")
	}
	before := append([]Point(nil), l.ExtraPoints...)
	l.UpdateLayout(left, right)
	if len(l.ExtraPoints) != len(before) || l.ExtraPoints[0] == before[0] {
		t.Errorf("waypoints must be rebuilt, not appended to: %+v", l.ExtraPoints)
	}
}

func TestMiddleLayoutRoutes(t *testing.T) {
	v := makeLine(Rect{0, 0, 40, 20}, Rect{10, 300, 40, 20}, lineStyle(t, "layout middle"))
	if len(v.ExtraPoints) != 2 || !near(v.ExtraPoints[0].X, v.ExtraPoints[1].X, 1e-3) {
		t.Errorf("vertical stack: %+v", v.ExtraPoints)
	}
	h := makeLine(Rect{0, 0, 40, 20}, Rect{300, 5, 40, 20}, lineStyle(t, "layout middle"))
	if len(h.ExtraPoints) != 2 || !near(h.ExtraPoints[0].Y, h.ExtraPoints[1].Y, 1e-3) {
		t.Errorf("horizontal: %+v", h.ExtraPoints)
	}
	d := makeLine(Rect{0, 0, 40, 20}, Rect{200, 100, 40, 20}, lineStyle(t, "layout middle"))
	if d.Source != (Point{40, 10}) || d.Target != (Point{220, 100}) || len(d.ExtraPoints) != 1 || d.ExtraPoints[0] != (Point{220, 10}) {
		t.Errorf("diagonal: %+v %+v %+v", d.Source, d.Target, d.ExtraPoints)
	}
}

func TestLineLabel(t *testing.T) {
	env := testEnv()
	l := makeLine(left, right, NewDrawableLineStyle(false))
	l.AddLabel("edge label", NewElementImageProvider(host(), 1, nil), env)
	if l.Label == nil || l.Label.Bounds().Width <= 0 {
		t.Fatal("label box")
	}
	double := makeLine(left, right, NewDrawableLineStyle(false))
	double.AddLabel(`one\ntwo`, NewElementImageProvider(host(), 1, nil), env)
	single := makeLine(left, right, NewDrawableLineStyle(false))
	single.AddLabel("one", NewElementImageProvider(host(), 1, nil), env)
	if !(double.Label.Bounds().Height > single.Label.Bounds().Height) {
		t.Error("a two-line label is taller")
	}
}

func TestLineBoundsCoverEndpoints(t *testing.T) {
	l := makeLine(left, right, NewDrawableLineStyle(false))
	b := l.Bounds()
	if b.X > math.Min(l.Source.X, l.Target.X)+1e-3 || b.X+b.Width < math.Max(l.Source.X, l.Target.X)-1e-3 {
		t.Errorf("bounds %+v", b)
	}
}

func TestArrowDisplayShortensPathAndWidensBounds(t *testing.T) {
	plain := NewDrawableLineBetween(Point{0, 0}, Point{100, 0}, lineStyle(t, ""))
	arrowed := NewDrawableLineBetween(Point{0, 0}, Point{100, 0}, lineStyle(t, "display arrow"))
	bp := arrowed.buildPath(Point{})
	if !bp.drawArrow || bp.to != (Point{95, 0}) {
		t.Errorf("arrow %+v", bp)
	}
	if !(arrowed.Bounds().Height > plain.Bounds().Height) {
		t.Error("arrow bounds")
	}
	if arrow(Point{0, 0}, Point{3, 0}, 0, 10, 10) != nil {
		t.Error("too short for an arrow")
	}
}

func TestIdenticalBoxesGiveFiniteCoordinates(t *testing.T) {
	box := Rect{10, 10, 40, 20}
	l := makeLine(box, box, NewDrawableLineStyle(false))
	for _, v := range []float64{l.Source.X, l.Source.Y, l.Target.X, l.Target.Y} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("not finite: %+v %+v", l.Source, l.Target)
		}
	}
}

// 1x1 transparent PNG.
const png1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

func TestImageProvider(t *testing.T) {
	item := host()
	item.Properties.Append(imageCmd("logo", png1))
	p := NewElementImageProvider(item, 1, StdImageDecoder)
	if _, ok := p.ResolveImagePath("missing"); ok {
		t.Error("unknown name")
	}
	r, ok := p.ResolveImagePath("logo")
	if !ok || r.Rect.Width != 1 || r.Rect.Height != 1 {
		t.Errorf("declared image: %+v %v", r, ok)
	}
	scaled, ok := NewElementImageProvider(item, 1, StdImageDecoder).ResolveImagePath("logo|20x10")
	if !ok || scaled.Rect.Width != 20 || scaled.Rect.Height != 10 {
		t.Errorf("scaled: %+v", scaled)
	}

	broken := host()
	broken.Properties.Append(imageCmd("broken", "not-base64!!"))
	if _, ok := NewElementImageProvider(broken, 1, StdImageDecoder).ResolveImagePath("broken"); ok {
		t.Error("broken data must not resolve")
	}
	truncated := host()
	truncated.Properties.Append(imageCmd("cut", png1[:40]))
	if _, ok := NewElementImageProvider(truncated, 1, StdImageDecoder).ResolveImagePath("cut"); ok {
		t.Error("truncated data must not resolve")
	}
}

func TestRoundBoxBoundsAndSetPath(t *testing.T) {
	bounds := Rect{1, 2, 30, 40}
	box := NewRoundBox(bounds, NewDrawableItemStyle(false), true)
	if !containsRect(box.Bounds(), bounds) || math.Abs(box.Bounds().Width-bounds.Width) >= 1 || box.Path() == nil {
		t.Errorf("bounds %+v", box.Bounds())
	}
	moved := Rect{5, 6, 20, 20}
	box.SetPath(moved)
	if !containsRect(box.Bounds(), moved) || math.Abs(midX(box.Bounds())-midX(moved)) > 1e-3 {
		t.Errorf("moved %+v", box.Bounds())
	}
}

func TestRoundBoxPathHonoursRadii(t *testing.T) {
	s := NewDrawableItemStyle(false)
	s.ParseStyle(styleTree(t, "corner-radius 2 4 6 8"), nil)
	box := NewRoundBox(Rect{0, 0, 50, 50}, s, true)
	assertRect(t, "path box", box.Path().boundingBox(), Rect{0, 0, 50, 50})
	if box.radius != [4]float64{2, 4, 6, 8} {
		t.Errorf("radii %v", box.radius)
	}
}

func TestShadowGrowsBoundsOnShadowSide(t *testing.T) {
	s := NewDrawableItemStyle(false)
	s.ParseStyle(styleTree(t, "shadow 3 -3 4"), nil)
	b := NewRoundBox(Rect{0, 0, 50, 50}, s, true).Bounds()
	if !(b.Width > 50 && b.Height > 50 && b.Y < 0) {
		t.Errorf("shadow bounds %+v", b)
	}
}

func TestEmptyAndCircleBoxKeepBounds(t *testing.T) {
	bounds := Rect{3, 4, 15, 25}
	empty := NewEmptyBox(bounds, newDrawableStyle(false))
	assertRect(t, "empty", empty.Bounds(), bounds)
	moved := Rect{0, 0, 1, 1}
	empty.SetPath(moved)
	assertRect(t, "empty moved", empty.Bounds(), moved)
	cb := Rect{0, 0, 40, 40}
	assertRect(t, "circle", NewCircleBox(cb, newDrawableStyle(false), true).Bounds(), cb)
}

func TestTextBoxFrame(t *testing.T) {
	tb := &TextBox{frame: Rect{1, 1, 50, 20}, attr: PlainText("hello", systemFont(12)), env: testEnv()}
	if tb.Bounds().Width != 50 || !tb.Visible() || !tb.Selectable() {
		t.Error("text box")
	}
	tb.SetFrame(Rect{0, 0, 80, 30})
	if tb.Bounds().Width != 80 {
		t.Error("SetFrame")
	}
	visited := 0
	tb.Traverse(func(Drawable) bool { visited++; return true })
	if visited != 1 {
		t.Errorf("visited %d", visited)
	}
}

func TestBackgroundLayerNotSelectable(t *testing.T) {
	b := &itemBase{visible: true}
	if !b.Selectable() {
		t.Error("default layer is selectable")
	}
	b.layer = LayerBackground
	if b.Selectable() {
		t.Error("background is not")
	}
}

func TestContainerUnionOfChildren(t *testing.T) {
	a := NewEmptyBox(Rect{0, 0, 10, 10}, newDrawableStyle(false))
	b := NewEmptyBox(Rect{100, 100, 10, 10}, newDrawableStyle(false))
	assertRect(t, "union", NewDrawableContainer(a, b).Bounds(), Rect{0, 0, 110, 110})
	if len(NewDrawableContainer().Children) != 0 {
		t.Error("empty container")
	}
}

func TestArcToPathShapes(t *testing.T) {
	// A corner radius over half the side makes the tangent point run backwards, as CGPath.addArc does.
	box := NewRoundBox(Rect{0, 0, 10, 10}, NewDrawableItemStyle(false), true)
	g := box.Path().toGfx(0, 0)
	if g.Empty() {
		t.Fatal("empty path")
	}
	b := g.Bounds()
	if b.W() < 10-1e-6 || b.H() < 10-1e-6 {
		t.Errorf("path bounds %+v", b)
	}
}
