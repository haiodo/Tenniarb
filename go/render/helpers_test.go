package render

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// testOpts measure with the 0.6 em estimate, so the numbers do not depend on the machine's fonts.
func testOpts() Options { return Options{Measurer: gfx.EstimateText{}} }

func testEnv() *textEnv { return newTextEnv(gfx.EstimateText{}, "") }

// parseDiagram parses tenn source and returns the first nested element (SceneRenderTests.parseDiagram).
func parseDiagram(t testing.TB, src string) *model.Element {
	t.Helper()
	m := model.ReadTenn(src)
	if m == nil {
		t.Fatal("source must parse cleanly")
	}
	return m.Elements[0]
}

// styleTree parses property lines the way an item body is parsed.
func styleTree(t testing.TB, src string) *tenn.Node {
	t.Helper()
	tree, p := tenn.Parse(src)
	if p.Errors.HasErrors() {
		t.Fatalf("parse %q: %v", src, p.Errors.Errors)
	}
	return tree
}

func near(a, b, tol float64) bool {
	d := a - b
	return d <= tol && d >= -tol
}

func assertNear(t testing.TB, what string, got, want, tol float64) {
	t.Helper()
	if !near(got, want, tol) {
		t.Errorf("%s = %v, want %v (+-%v)", what, got, want, tol)
	}
}

func assertRect(t testing.TB, what string, got, want Rect) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %+v, want %+v", what, got, want)
	}
}

// fakeExec gives every expression token of an element's items and properties a fixed value.
type fakeExec struct {
	values map[string]any // expression source -> value
}

func (f fakeExec) SetScaleFactor(float64)          {}
func (f fakeExec) SetElement(*model.Element)       {}
func (f fakeExec) UpdateAll(func())                {}
func (f fakeExec) NotifyChanges(*model.ModelEvent) {}
func (f fakeExec) GetEvaluated(target model.EvalTarget, _ *tenn.Node, _ *model.DrawableSize) map[*tenn.Token]any {
	var props *model.ModelProperties
	switch x := target.(type) {
	case *model.DiagramItem:
		props = x.Properties
	case *model.Element:
		props = x.Properties
	}
	out := map[*tenn.Token]any{}
	props.AsNode().Traverse(func(n *tenn.Node) bool {
		if (n.Kind == tenn.KindExpression || n.Kind == tenn.KindExpressionBlock) && n.Token != nil {
			if v, ok := f.values[n.Token.Literal]; ok {
				out[n.Token] = v
			}
		}
		return true
	})
	return out
}

func imageCmd(name, data string) *tenn.Node {
	return tenn.NewCommand("image", tenn.NewIdent(name), tenn.NewIdent(data))
}

func (b *itemBase) item() *model.DiagramItem { return b.Item }
