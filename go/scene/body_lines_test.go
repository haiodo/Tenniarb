package scene_test

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// BodyLines is the layout of a body the canvas draws, line by line: the stand compares the lines and not
// only the box, which is how a box one line taller than its text became visible.
func TestBodyLinesOfAScreen(t *testing.T) {
	const text = `element "Screen" {
  item A {
    body "one\ntwo\nthree"
  }
}`
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		screen, err := tree.Build(tx, nil, "Screen", scene.Options{Measurer: gfx.DefaultMeasurer})
		if err != nil {
			t.Fatal(err)
		}
		var walk func(p *scene.Placed)
		walk = func(p *scene.Placed) {
			for _, k := range p.Kids {
				if k.Src == nil || k.Src.Kind != scene.KItem {
					walk(k)
					continue
				}
				lines := screen.BodyLines(k.Src, 0)
				if len(lines) != 3 {
					t.Fatalf("%d lines, want 3: %+v", len(lines), lines)
				}
				for i, want := range []string{"one", "two", "three"} {
					if lines[i].Text != want {
						t.Errorf("line %d is %q, want %q", i, lines[i].Text, want)
					}
					if lines[i].H <= 0 {
						t.Errorf("line %d has no height: %+v", i, lines[i])
					}
				}
				walk(k)
			}
		}
		walk(screen.Placed())
	})
}
