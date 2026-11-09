package scene_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// tiffB64 is a 8 by 4 TIFF, the format an old file keeps a screenshot in: go and the Swift app read it
// with CoreGraphics, so the decoders of the scene have to know it too.
const tiffB64 = "SUkqAIgAAAAAAAD//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////w0AAAEDAAEAAAAIAAAAAQEDAAEAAAAEAAAAAgEDAAQAAAAqAQAAAwEDAAEAAAABAAAABgEDAAEAAAACAAAAEQEEAAEAAAAIAAAAFQEDAAEAAAAEAAAAFgEDAAEAAAAEAAAAFwEEAAEAAACAAAAAGgEFAAEAAAAyAQAAGwEFAAEAAAA6AQAAKAEDAAEAAAACAAAAUgEDAAEAAAACAAAAAAAAAAgACAAIAAgASAAAAAEAAABIAAAAAQAAAA=="

// The ${...} of a classic title and body is an expression of the element model: the sentence drawn is the
// value, as go/render drew it, not the source of the expression.
func TestClassicTextTakesTheModelValue(t *testing.T) {
	src := `element "E" {
  item A { value 41; title "value ${value}"; body %{sum ${value + 1}} }
}`
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	hook := func(at scene.ClassicRef, prop string) ([]string, bool) {
		switch prop {
		case "title":
			return []string{"value 41"}, true
		case "body":
			return []string{"sum 42"}, true
		}
		return nil, false
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}, Classic: hook})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	var svg bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&svg, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">value 41<", ">sum 42<"} {
		if !strings.Contains(svg.String(), want) {
			t.Errorf("no %q in the picture:\n%s", want, svg.String())
		}
	}
	if strings.Contains(svg.String(), "${value") {
		t.Errorf("the expression is drawn as written:\n%s", svg.String())
	}
}

// An inline picture of a classic item may be a TIFF: the old renderer read it with CoreGraphics, so the
// scene reads it too instead of drawing the item empty.
func TestClassicInlineTIFF(t *testing.T) {
	src := fmt.Sprintf(`element "E" {
  item Pic { image "shot.tiff" @(%s); title "@(shot.tiff|40)" }
}`, tiffB64)
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
	for _, w := range screen.Warnings {
		if strings.Contains(w.Msg, "shot.tiff") {
			t.Errorf("the TIFF is refused: %s", w.Msg)
		}
	}
	var svg bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&svg, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg.String(), "<image") {
		t.Errorf("no picture in the item:\n%s", svg.String())
	}
}
