package render

import (
	"github.com/haiodo/gowt/gfx"
	"math"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

const editorSrc = `element "D" {
  item "A" { pos 0 0 }
  item "B" { pos 200 0 }
  item "C" { pos 0 -150 }
  item "Back" {
    pos 0 0
    layer background
  }
  link "A" "B" { display arrow }
  link "B" "C" { display arrow }
}`

func editorScene(t *testing.T) (*DrawableScene, []*model.DiagramItem) {
	t.Helper()
	o := testOpts()
	o.NoEvaluate = true
	el := parseDiagram(t, "element \"X\" {\n"+strings.TrimSuffix(strings.TrimPrefix(editorSrc, "element \"D\" {\n"), "}")+"}")
	return Build(el, o), el.Items
}

func centre(r Rect) Point { return Point{midX(r), midY(r)} }

func TestCrossPointLine(t *testing.T) {
	p1, p2 := Point{0, 0}, Point{100, 0}
	for _, c := range []struct {
		name string
		p    Point
		want bool
	}{
		{"on the line", Point{50, 0}, true},
		{"within tolerance", Point{50, 6}, true},
		{"too far", Point{50, 8}, false},
		{"near the start", Point{-5, 3}, true},
		{"beyond the end, outside the radius", Point{120, 0}, false},
		{"near the end", Point{105, 0}, true},
	} {
		if got := crossPointLine(p1, p2, c.p); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if crossPointLine(Point{10, 10}, Point{10, 10}, Point{30, 30}) {
		t.Error("a zero-length segment hits only near its point")
	}
	if !crossPointLine(Point{0, 0}, Point{0, 100}, Point{3, 50}) {
		t.Error("a vertical line is padded to 5")
	}
}

func TestFind(t *testing.T) {
	s, items := editorScene(t)
	a, b, back := items[0], items[1], items[3]
	hit := func(p Point, all bool) []*model.DiagramItem { return s.Find(p, all) }
	ca := centre(s.Drawables[a].SelectorBounds())
	if got := hit(ca, false); len(got) != 1 || got[0] != a {
		t.Errorf("box: %v", got)
	}
	// The background item lies under A: found only with all, and after A in item order.
	if got := hit(ca, true); len(got) != 2 || got[0] != a || got[1] != back {
		t.Errorf("all: %v", got)
	}
	cb := centre(s.Drawables[b].SelectorBounds())
	mid := Point{(ca.X + cb.X) / 2, (ca.Y + cb.Y) / 2}
	if got := hit(mid, false); len(got) != 1 || got[0].Kind != model.ItemKindLink {
		t.Errorf("line between boxes: %v", got)
	}
	if got := hit(Point{5000, 5000}, false); len(got) != 0 {
		t.Errorf("empty: %v", got)
	}
	// Where a box and a line meet, only the box.
	line := s.Drawables[items[4]].(*DrawableLine)
	if got := hit(Point{line.Source.X - 1, line.Source.Y}, false); len(got) != 1 || got[0] != a {
		t.Errorf("box wins over line: %v", got)
	}
}

func TestLineFindFollowsWaypoints(t *testing.T) {
	l := NewDrawableLineBetween(Point{0, 0}, Point{100, 0}, NewDrawableLineStyle(false))
	l.ExtraPoints = []Point{{100, 100}}
	if !l.Find(Point{100, 50}) || !l.Find(Point{50, 50}) || l.Find(Point{50, 0}) {
		t.Error("the polyline goes through the waypoint")
	}
}

func TestLabelBounds(t *testing.T) {
	s, items := editorScene(t)
	l := s.Drawables[items[4]].(*DrawableLine)
	b := l.Bounds()
	assertRect(t, "no label", l.LabelBounds(), Rect{midX(b) - 50, midY(b) - 15, 100, 30})
	env := testEnv()
	l.AddLabel("lbl", s.newImageProvider(items[4]), env)
	lb := l.LabelBounds()
	size := l.Label.SelectorBounds()
	if lb.Width != size.Width || lb.Height != size.Height {
		t.Errorf("label size %v, label %v", lb, size)
	}
	l.ExtraPoints = []Point{{10, 20}}
	assertRect(t, "after the first waypoint", l.LabelBounds(), Rect{15, 20 - l.Label.Bounds().Height, size.Width, size.Height})
}

func TestUpdateLineTo(t *testing.T) {
	s, items := editorScene(t)
	a, c := items[0], items[2]
	sa, sc := s.Drawables[a].Bounds(), s.Drawables[c].Bounds()
	s.UpdateLineTo(a, Point{300, 300}, nil)
	if s.LineTo == nil {
		t.Fatal("no preview")
	}
	if s.LineTo.Target != (Point{300, 300}) || !containsRect(insetBy(sa, -1e-9, -1e-9), Rect{s.LineTo.Source.X, s.LineTo.Source.Y, 0, 0}) {
		t.Errorf("free end: %v -> %v", s.LineTo.Source, s.LineTo.Target)
	}
	s.UpdateLineTo(a, Point{300, 300}, c)
	end := s.LineTo.Target
	onEdge := math.Abs(end.X-minX(sc)) < 1e-6 || math.Abs(end.X-maxX(sc)) < 1e-6 || math.Abs(end.Y-minY(sc)) < 1e-6 || math.Abs(end.Y-maxY(sc)) < 1e-6
	if !onEdge {
		t.Errorf("the line must end on the edge of the target: %v in %v", end, sc)
	}
	s.RemoveLineTo()
	if s.LineTo != nil {
		t.Error("preview must go")
	}
	s.UpdateLineTo(&model.DiagramItem{}, Point{}, nil) // not in the scene: nothing
	if s.LineTo != nil {
		t.Error("unknown source")
	}
}

func TestUpdateLayoutMatchesFreshBuild(t *testing.T) {
	s, items := editorScene(t)
	a, b := items[0], items[1]
	moved := s.UpdateLayout(map[*model.DiagramItem]Point{a: {40, 30}})
	if len(moved) != 2 { // A and its link
		t.Fatalf("moved %d drawables", len(moved))
	}
	box := s.Drawables[a].SelectorBounds()
	if box.X != 40 || box.Y+box.Height != 30 {
		t.Errorf("A at %v", box)
	}
	// What a fresh build of the moved model looks like.
	a.X, a.Y = 40, 30
	o := testOpts()
	o.NoEvaluate = true
	fresh := Build(a.Parent, o)
	for _, i := range items {
		got, want := s.Drawables[i].SelectorBounds(), fresh.Drawables[i].SelectorBounds()
		if got != want {
			t.Errorf("%s: %v, fresh %v", i.Name, got, want)
		}
	}
	if s.Drawables[items[5]].(*DrawableLine).Source != fresh.Drawables[items[5]].(*DrawableLine).Source {
		t.Error("untouched link differs")
	}
	// A box moved far away stays in the picture.
	s.UpdateLayout(map[*model.DiagramItem]Point{b: {1e5, 1e5}})
	if !s.Drawables[b].Visible() {
		t.Error("layout must not cull a moved box")
	}
	// A link moves its control point.
	link := items[4]
	s.UpdateLayout(map[*model.DiagramItem]Point{link: {30, 40}})
	if s.Drawables[link].(*DrawableLine).Control != (Point{30, 40}) {
		t.Error("control point")
	}
}

func TestSelectorsDraw(t *testing.T) {
	s, items := editorScene(t)
	rec := s.NewRecorder()
	NewSelectorBox(s.Drawables[items[0]].SelectorBounds(), false).Draw(rec, Point{})
	NewSelectorLine(s.Drawables[items[4]].(*DrawableLine)).Draw(rec, Point{})
	(&SelectorBox{Pos: Point{0, 0}, Size: Size{10, 10}, Blur: 5}).Draw(rec, Point{}) // the band: square corners
	pic := rec.Finish()
	if pic.Len() == 0 {
		t.Fatal("nothing drawn")
	}
	var svg strings.Builder
	if err := pic.WriteSVG(&svg, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg.String(), "stroke-dasharray") {
		t.Errorf("selectors are dashed:\n%s", svg.String())
	}
	green := s.NewRecorder()
	NewSelectorBox(Rect{0, 0, 10, 10}, true).Draw(green, Point{})
	if green.Finish().Equal(func() *gfx.Picture {
		r := s.NewRecorder()
		NewSelectorBox(Rect{0, 0, 10, 10}, false).Draw(r, Point{})
		return r.Finish()
	}()) {
		t.Error("the editing outline is green, not blue")
	}
}

func TestItemsOrderAndDrawableOf(t *testing.T) {
	s, items := editorScene(t)
	var want []*model.DiagramItem
	for _, i := range items {
		if i.Kind == model.ItemKindItem {
			want = append(want, i)
		}
	}
	for _, i := range items {
		if i.Kind == model.ItemKindLink {
			want = append(want, i)
		}
	}
	got := s.Items()
	if len(got) != len(want) {
		t.Fatalf("items %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] || DrawableOf(s.Drawables[want[i]]) != want[i] {
			t.Errorf("item %d", i)
		}
	}
	if DrawableOf(NewDrawableContainer()) != nil {
		t.Error("a plain container has no item")
	}
}

func TestBodyTextHelpers(t *testing.T) {
	if got := PrepareBodyText("\n a\\nb\t\n"); got != " a\nb    " {
		t.Errorf("PrepareBodyText = %q", got)
	}
	if s, ok := GetString(nil); ok || s != "" {
		t.Error("nil node")
	}
	if ValueString(int64(3)) != "3" || ValueString("x") != "x" {
		t.Error("ValueString")
	}
}

// shadowScreens lists Shadow.Screen of every shadow op the scene's items record, plus the selectors.
func shadowScreens(t *testing.T, screen bool) []bool {
	t.Helper()
	o := testOpts()
	o.NoEvaluate = true
	o.ScreenShadows = screen
	el := parseDiagram(t, "element \"X\" {\n item \"A\" {\n  shadow 3 3 4\n  stack 2\n  pos 0 0\n }\n item \"B\" {\n  shadow 2 2 3\n  pos 100 0\n }\n link \"A\" \"B\" {\n  shadow 1 1 2\n }\n}")
	s := Build(el, o)
	rec := s.NewRecorder()
	s.Draw(rec, Point{})
	NewSelectorBox(Rect{0, 0, 10, 10}, false).Draw(rec, Point{})
	var got []bool
	var it gfx.Iter
	it.Reset(rec.Finish())
	for it.Next() {
		switch k := it.Op().Kind; k {
		case gfx.OpShadowBox, gfx.OpFillShadow, gfx.OpStrokeShadow, gfx.OpTextShadow:
			got = append(got, it.Op().Shadow.Screen)
		}
	}
	return got
}

func TestScreenShadowsOption(t *testing.T) {
	for _, screen := range []bool{false, true} {
		got := shadowScreens(t, screen)
		if len(got) < 4 {
			t.Fatalf("screen=%v: only %d shadows recorded", screen, len(got))
		}
		for i, g := range got[:len(got)-1] { // the last one is the selector, which has its own field
			if g != screen {
				t.Errorf("screen=%v: shadow %d has Screen=%v", screen, i, g)
			}
		}
	}
}
