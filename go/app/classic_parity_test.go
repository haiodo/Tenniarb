//go:build darwin || linux || windows

package app

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// classicPicture builds element e of file as the canvas draws it, and returns the scene view.
func classicPicture(t *testing.T, file string, e *model.Element) (*sceneView, *gfx.Scene) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, filepath.Base(file))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, &Shell{dir: dir}, path, string(data))
	v.session.SetElement(e)
	v.state.Current = e
	sv := newSceneView(e, v.elementNode(e))
	sv.classic = true
	v.scene = gfx.NewScene()
	if err := v.buildSceneView(sv); err != nil {
		t.Fatal(err)
	}
	sv.index(sv.placed)
	return sv, v.scene
}

// elementOf is the element with this name anywhere in the document.
func parityElement(t *testing.T, file, name string) *model.Element {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := model.ReadTenn(string(data))
	if m == nil {
		t.Fatalf("%s: does not parse", file)
	}
	var found *model.Element
	var walk func(*model.Element)
	walk = func(e *model.Element) {
		if e.Name == name {
			found = e
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)
	if found == nil {
		t.Fatalf("%s: no element %q", file, name)
	}
	return found
}

// itemBoxes are the boxes of the items of e as the old renderer gives them, keyed by item.
func itemBoxes(e *model.Element) map[*model.DiagramItem]gfx.Rect {
	ds := render.Build(e, render.Options{Measurer: gfx.DefaultMeasurer, FontFamily: "Helvetica, Arial, sans-serif"})
	out := map[*model.DiagramItem]gfx.Rect{}
	for _, it := range e.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		if d := ds.Drawables[it]; d != nil {
			b := d.SelectorBounds()
			out[it] = gfx.R(b.X, b.Y, b.Width, b.Height)
		}
	}
	return out
}

// parityDoc is a classic element whose boxes follow their text: the metrics of a line, the wrapping of a
// body and a list, the inset of the text.
const parityDoc = `element "P" {
    item Title { pos 0 0; title "a title of some length" }
    item Body { pos 0 -80; body %{
line one
line two
} }
    item List { pos 0 -160; body %{
* first item of a list
* second item
} }
    item Narrow { pos 300 0; width 70; body "a word wider than the seventy points" }
    item Dot { pos 300 -120; width 8; height 8 }
    item Padded { pos 300 -200; padding 10; title "padded" }
}
`

// The picture of a classic element matches go/render box for box: the stand of the engine work is pinned
// here, so a change of the scene that moves an item fails now instead of after a release.
func TestClassicPictureMatchesRender(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "p.tenn")
	if err := os.WriteFile(file, []byte(parityDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	e := parityElement(t, file, "P")
	sv, _ := classicPicture(t, file, e)
	old := itemBoxes(e)
	var sx, sy float64
	var pairs [][3]interface{}
	for _, it := range e.Items {
		pl := classicPlacedOf(sv, it)
		b, ok := old[it]
		if pl == nil || !ok {
			continue
		}
		nx, ny := pl.X, -(pl.Y + pl.H)
		sx += nx - b.Min.X
		sy += ny - b.Min.Y
		pairs = append(pairs, [3]interface{}{it.Name, b, gfx.R(nx, ny, pl.W, pl.H)})
	}
	if len(pairs) != len(e.Items) {
		t.Fatalf("%d of %d items drawn by both renderers", len(pairs), len(e.Items))
	}
	sx, sy = sx/float64(len(pairs)), sy/float64(len(pairs))
	for _, p := range pairs {
		name := p[0].(string)
		b, n := p[1].(gfx.Rect), p[2].(gfx.Rect)
		dx, dy := math.Abs(n.Min.X-sx-b.Min.X), math.Abs(n.Min.Y-sy-b.Min.Y)
		dw, dh := math.Abs(n.W()-b.W()), math.Abs(n.H()-b.H())
		// The boxes stand where go/render puts them and are as tall: the width of a text box is up to a few
		// points narrower in the scene (the old renderer measured a line a little wider), so it is held to
		// a wider tolerance and the vertical metrics are held tight.
		if worst := math.Max(math.Max(dx, dy), dh); worst > 0.5 {
			t.Errorf("%s: off by %.2f (old %v, scene %v, frame %v,%v)", name, worst, b, n, sx, sy)
		}
		if dw > 8 {
			t.Errorf("%s: %v wide, go/render %v", name, n.W(), b.W())
		}
	}
}

// classicPlacedOf is where the scene drew the item, or nil when it drew nothing.
func classicPlacedOf(sv *sceneView, it *model.DiagramItem) *scene.Placed {
	for _, pl := range sv.byPath {
		if pl.Src != nil && classicItemAt(sv.el, pl.Src) == it {
			return pl
		}
	}
	return nil
}

// An expression of a classic body is drawn as its value, not as the source of the expression, and a TIFF
// inline picture is drawn as the Swift app drew it: the two things the stand found in docs/Example.tenn.
func TestClassicTextAndPicture(t *testing.T) {
	e := parityElement(t, "../../docs/Example.tenn", "Executions")
	sv, sc := classicPicture(t, "../../docs/Example.tenn", e)
	text := pictureText(t, sc, sv.root)
	if !strings.Contains(text, "Tasks: 9") {
		t.Errorf("no evaluated expression in the picture:\n%s", text)
	}
	if strings.Contains(text, "parent.items.filter") {
		t.Errorf("the source of the expression is drawn:\n%s", text)
	}

	e = parityElement(t, "../../docs/Example.tenn", "Circle image")
	sv, sc = classicPicture(t, "../../docs/Example.tenn", e)
	if !strings.Contains(pictureText(t, sc, sv.root), "Image") {
		t.Error("no Image item")
	}
	for _, pl := range sv.byPath {
		if pl.Gfx == nil || pl.W < 10 {
			continue
		}
		if pic := pl.Gfx.Picture(); pic != nil && hasImage(pic) {
			return
		}
	}
	t.Error("no inline TIFF picture drawn in the element")
}

// pictureText is the text of the SVG of root.
func pictureText(t *testing.T, sc *gfx.Scene, root *gfx.Node) string {
	t.Helper()
	sc.Commit()
	var buf bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&buf, sc, root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// hasImage reports whether the picture draws a raster.
func hasImage(pic *gfx.Picture) bool {
	var it gfx.Iter
	it.Reset(pic)
	for it.Next() {
		if it.Op().Kind == gfx.OpImage {
			return true
		}
	}
	return false
}
