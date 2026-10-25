package editor

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
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
		Render:   render.Options{Measurer: gfx.EstimateText{}, NoEvaluate: opt.noEval},
		Readonly: opt.readonly,
		PCKeys:   opt.pc,
		OnChange: func(text string) { f.changes = append(f.changes, text) },
		OnRedraw: func(r Redraw) { f.redraws = append(f.redraws, r) },
		OnError:  func(err error) { t.Errorf("store error: %v", err) },
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
func (f *fixture) centre(i *model.DiagramItem) render.Point {
	r := f.s.scene.Drawables[i].SelectorBounds()
	return render.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

func (f *fixture) ends(i int) [2]render.Point {
	l := f.s.scene.Drawables[f.s.element.Items[i]].(*render.DrawableLine)
	return [2]render.Point{l.Source, l.Target}
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
func (f *fixture) click(p render.Point, mods Modifiers) {
	f.s.MouseDown(MouseEvent{P: p, Mods: mods})
	f.s.MouseUp(MouseEvent{P: p, Mods: mods})
}

// drag presses at from, moves in two steps and releases at to.
func (f *fixture) drag(from, to render.Point, mods Modifiers) {
	f.s.MouseDown(MouseEvent{P: from, Mods: mods})
	f.s.MouseMove(MouseEvent{P: render.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}, Mods: mods})
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
