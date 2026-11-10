//go:build darwin || linux || windows

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
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
