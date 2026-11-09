//go:build darwin || linux || windows

package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
)

// The export follows the canvas: a classic diagram drawn by the scene is exported by the scene writer,
// and the rollback switch puts it back on go/render.
func TestClassicExportFollowsTheCanvas(t *testing.T) {
	text := `element "Old" {
  item A { width 40; height 20; title "A" }
}
`
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "m.tenn"), text)
	e := Resolve(&v.session.Root().Element, []string{"Old"})
	if e == nil {
		t.Fatal("no element Old")
	}
	was := sceneCanvasOn
	defer func() { sceneCanvasOn = was }()
	sceneCanvasOn = false
	classic, err := v.exportSVGOf(e, DefaultPrefs())
	if err != nil {
		t.Fatal(err)
	}
	sceneCanvasOn = true
	drawn, err := v.exportSVGOf(e, DefaultPrefs())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(classic, drawn) {
		t.Fatal("the scene canvas still exports the classic diagram with go/render")
	}
	if !strings.Contains(string(drawn), ">A<") {
		t.Errorf("the scene export lost the title:\n%s", drawn)
	}
}

// A classic element exported through the scene engine takes the values of its expressions from the
// element model, as the classic renderer did: exportSVGOf keeps that renderer until the canvas moves.
func TestExportSceneClassicValues(t *testing.T) {
	text := `element "Screen" {
  item col { auto-layout column }
}
element "Old" {
  item A { width ${name == "A" ? 82 : 10}; height 10; title "${name} wide" }
}
`
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "m.tenn"), text)
	e := Resolve(&v.session.Root().Element, []string{"Old"})
	if e == nil {
		t.Fatal("no element Old")
	}
	sc, screen, err := v.exportScene(e)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := screen.WriteSVG(&buf, sc, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "A wide") {
		t.Errorf("the title does not name the item:\n%s", buf.String())
	}
	for _, b := range screen.Boxes() {
		if b.Name == "A" && b.W != 82 {
			t.Errorf("width %g, want 82", b.W)
		}
	}
}
