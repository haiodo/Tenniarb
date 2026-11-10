package editor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/scenegeom"
)

const testSrc = `element "D" {
  item "A" { pos 0 0 }
  item "B" { pos 200 0 }
  item "C" { pos 0 -150 }
  link "A" "B" { display arrow }
  link "B" "C" { display arrow }
}`

// fixture is a session on testSrc with the items named and every OnChange text recorded.
type fixture struct {
	t       *testing.T
	s       *Session
	a, b, c *model.DiagramItem
	changes []string
	redraws []Redraw
}

type fixtureOpts struct {
	readonly bool
	src      string
	pc       bool
	noEval   bool
}

func newFixture(t *testing.T, o ...fixtureOpts) *fixture {
	t.Helper()
	var opt fixtureOpts
	if len(o) > 0 {
		opt = o[0]
	}
	if opt.src == "" {
		opt.src = testSrc
	}
	m := model.ReadTenn(opt.src)
	if m == nil {
		t.Fatal("source does not parse")
	}
	f := &fixture{t: t}
	s, err := New(m.Elements[0], Options{
		Geom:       testGeom(t, m.Elements[0], opt.src, gfx.EstimateText{}),
		NoEvaluate: opt.noEval,
		Readonly:   opt.readonly,
		PCKeys:     opt.pc,
		OnChange:   func(text string) { f.changes = append(f.changes, text) },
		OnRedraw:   func(r Redraw) { f.redraws = append(f.redraws, r) },
		OnError:    func(err error) { t.Errorf("store error: %v", err) },
	})
	if err != nil {
		t.Fatal(err)
	}
	f.s = s
	if len(s.element.Items) >= 3 {
		f.a, f.b, f.c = s.element.Items[0], s.element.Items[1], s.element.Items[2]
	}
	return f
}

// centre is the middle of the box of an item.
func (f *fixture) centre(i *model.DiagramItem) geometry.Point {
	r, ok := f.s.geom().Box(i)
	if !ok {
		f.t.Fatalf("no box for %q", i.Name)
	}
	return geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

// ends is where the line of a link runs: from the middle of one end to the middle of the other.
func (f *fixture) ends(i int) [2]geometry.Point {
	l := f.s.element.Items[i]
	if l.Source == nil || l.Target == nil {
		f.t.Fatalf("%q is not a link with two ends", l.Name)
	}
	return [2]geometry.Point{f.centre(l.Source), f.centre(l.Target)}
}

func (f *fixture) links() int {
	n := 0
	for _, i := range f.s.element.Items {
		if i.Kind == model.ItemKindLink {
			n++
		}
	}
	return n
}

// click presses and releases at p.
func (f *fixture) click(p geometry.Point, mods Modifiers) {
	f.s.MouseDown(MouseEvent{P: p, Mods: mods})
	f.s.MouseUp(MouseEvent{P: p, Mods: mods})
}

// drag presses at from, moves in two steps and releases at to.
func (f *fixture) drag(from, to geometry.Point, mods Modifiers) {
	f.s.MouseDown(MouseEvent{P: from, Mods: mods})
	f.s.MouseMove(MouseEvent{P: geometry.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}, Mods: mods})
	f.s.MouseUp(MouseEvent{P: to, Mods: mods})
}

func (f *fixture) key(key string, mods Modifiers) bool {
	return f.s.Key(KeyEvent{Key: key, Mods: mods})
}

func pos(items ...*model.DiagramItem) []float64 {
	var out []float64
	for _, i := range items {
		out = append(out, i.X, i.Y)
	}
	return out
}

func (f *fixture) props(i *model.DiagramItem) string { return f.s.PropsText(Target{Item: i}) }

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("%q does not contain %q", s, sub)
	}
}

// testGeom is the geometry of an element of src: the scene builds it, as the canvas would.
func testGeom(t *testing.T, el *model.Element, src string, measure gfx.TextMeasurer) Geom {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "test.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	return scenegeom.New(tree, el, measure)
}

// box is the drawn box of an item, as a click tests it.
func (f *fixture) box(it *model.DiagramItem) geometry.Rect {
	f.t.Helper()
	r, ok := f.s.geom().Box(it)
	if !ok {
		f.t.Fatalf("no box for %q", it.Name)
	}
	return r
}

// paint is the box the picture of an item covers, the shadow included.
func (f *fixture) paint(it *model.DiagramItem) geometry.Rect {
	f.t.Helper()
	r, ok := f.s.geom().PaintBox(it)
	if !ok {
		f.t.Fatalf("no painted box for %q", it.Name)
	}
	return r
}

// testGeomFile is testGeom for an element read from a file: the tree sees the file it is written in.
func testGeomFile(t *testing.T, el *model.Element, file, raw string, measure gfx.TextMeasurer) Geom {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), filepath.Base(file), raw)
	if err != nil {
		return scenegeom.New(nil, el, measure) // a file the scene cannot read: the model places the items
	}
	return scenegeom.New(tree, el, measure)
}

// boxOf is the drawn box of an item of a session.
func boxOf(t *testing.T, s *Session, it *model.DiagramItem) geometry.Rect {
	t.Helper()
	b, ok := s.geom().Box(it)
	if !ok {
		t.Fatalf("no box for %q", it.Name)
	}
	return b
}
