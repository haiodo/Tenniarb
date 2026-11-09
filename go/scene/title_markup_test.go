package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// A title of a classic diagram reads the emphasis of the markdown, as go/render reads it: the asterisks of
// **bold** are markup, not characters, and the run is bold.
func TestTitleTakesTheMarkdownEmphasis(t *testing.T) {
	const src = `element "E" {
  item A { title "*ProjectType*" }
  item B { title "plain title" }
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
	out := svg.String()
	if strings.Contains(out, "*ProjectType*") {
		t.Errorf("the asterisks are drawn:\n%s", out)
	}
	if !strings.Contains(out, ">ProjectType<") {
		t.Errorf("no bold ProjectType in:\n%s", out)
	}
	if !strings.Contains(out, `font-weight="700"`) {
		t.Errorf("the run carries no weight:\n%s", out)
	}
}
