package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// A marker is drawn in front of the title, with a space, as go/render draws it: the emoji of a mind map is
// the text of its box, not a decoration the scene may drop.
func TestMarkerIsPartOfTheTitle(t *testing.T) {
	const src = `element "E" {
  item A { title "Go for a walk"; marker "🚶" }
  item B { title "Go for a walk" }
}`
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	var svg bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&svg, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg.String(), ">🚶 Go for a walk<") {
		t.Errorf("the marker is not in front of the title:\n%s", svg.String())
	}
	// The item with a marker is wider by the marker and its space: the box follows the text it draws.
	var marked, plain float64
	for _, b := range screen.Boxes() {
		switch b.Path {
		case "A":
			marked = b.W
		case "B":
			plain = b.W
		}
	}
	if marked <= plain {
		t.Errorf("the marked item is %v wide, the plain one %v: the marker is not measured", marked, plain)
	}
}
