package editor

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

const execSrc = `element D {
  item A {
    pos 0 0
    width 120
  }
  item B {
    pos 0 -100
    title ${"w=" + items.A.width}
  }
}`

func drawnText(t *testing.T, f *fixture, item *model.DiagramItem) string {
	t.Helper()
	var b strings.Builder
	if err := f.s.scene.Picture(f.s.scene.Drawables[item]).WriteSVG(&b, 1); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// A change made through the session reaches the expressions that read it, and undo takes it back.
func TestExpressionsRecalculateAfterEdit(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: execSrc})
	b := f.s.element.Items[1]
	// SVG writes text as glyph runs of the measurer, so look at the evaluated value another way.
	eval := func() string {
		vals := f.s.PropsValues(Target{Item: b}, "title ${\"w=\" + items.A.width}")
		return vals[0]
	}
	if got := eval(); got != "w=120" {
		t.Fatalf("before: %q", got)
	}
	f.s.Select([]*model.DiagramItem{f.s.element.Items[0]})
	f.s.SetQuickStyle("width", "300")
	if got := eval(); got != "w=300" {
		t.Errorf("after the edit: %q", got)
	}
	f.s.Undo()
	if got := eval(); got != "w=120" {
		t.Errorf("after undo: %q", got)
	}
	if drawnText(t, f, b) == "" {
		t.Error("B must draw")
	}
}

func TestNoEvaluateKeepsTokens(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: execSrc, noEval: true})
	if f.s.exec != nil {
		t.Error("no engine expected")
	}
	if got := f.s.PropsValues(Target{Item: f.s.element.Items[1]}, "title ${1+1}"); len(got) != 0 {
		t.Errorf("values without an engine: %v", got)
	}
	f.s.Select([]*model.DiagramItem{f.s.element.Items[0]})
	f.s.SetQuickStyle("color", "red")
	contains(t, f.s.Text(), "color red")
}
