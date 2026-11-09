package scene

import (
	"testing"

	"github.com/haiodo/gowt/gfx"
)

func buildScreen(t *testing.T, src string, o Options) *Screen {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "a.tenn", []byte(src))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "E", o) })
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	return screen
}

// firstFillX is the x of the first point of the first filled path: the corner radius of a box.
func firstFillX(t *testing.T, p *gfx.Picture) float64 {
	t.Helper()
	var it gfx.Iter
	it.Reset(p)
	for it.Next() {
		if op := it.Op(); op.Kind == gfx.OpFill {
			if op.Path == nil || len(op.Path.Pts) == 0 {
				t.Fatalf("fill without a path")
			}
			return op.Path.Pts[0].X
		}
	}
	t.Fatalf("no fill in the picture")
	return 0
}

func TestClassicDefaultRadius(t *testing.T) {
	sc := buildScreen(t, `element "E" {
  item A { pos 0 0; width 40; height 20 }
  item B { pos 0 40; width 60; height 1 }
}`, Options{})
	if !sc.Classic() {
		t.Fatal("a flat element is a classic diagram")
	}
	if got := firstFillX(t, placed(t, sc, "A").Gfx.Picture()); got != 8 {
		t.Errorf("radius of A = %v, want 8", got)
	}
	// A hairline is a divider: the classic renderer keeps it square.
	if got := firstFillX(t, placed(t, sc, "B").Gfx.Picture()); got != 0 {
		t.Errorf("radius of B = %v, want 0 for a hairline", got)
	}
}

func TestClassicLinkDrawsUnderItems(t *testing.T) {
	sc := buildScreen(t, `element "E" {
  item A { pos 0 0; width 40; height 20 }
  item B { pos 100 0; width 40; height 20 }
  link A B
}`, Options{})
	var kinds []string
	for _, n := range sc.Root.Children() {
		pic := n.Picture()
		if pic == nil {
			continue
		}
		kind := "item"
		var it gfx.Iter
		it.Reset(pic)
		for it.Next() {
			if op := it.Op(); op.Kind == gfx.OpStroke {
				kind = "link"
			}
		}
		kinds = append(kinds, kind)
	}
	if len(kinds) != 3 {
		t.Fatalf("children = %v, want the link and two items", kinds)
	}
	if kinds[0] != "link" {
		t.Errorf("order = %v, want the link first", kinds)
	}
}

func TestScreenShadows(t *testing.T) {
	const src = `element "E" {
  item A { pos 0 0; width 40; height 20; shadow -5 -5 5 }
}`
	for _, want := range []bool{false, true} {
		sc := buildScreen(t, src, Options{ScreenShadows: want})
		pic := placed(t, sc, "A").Gfx.Picture()
		var it gfx.Iter
		it.Reset(pic)
		seen := false
		for it.Next() {
			if op := it.Op(); op.Kind == gfx.OpFillShadow {
				seen = true
				if op.Shadow.Screen != want {
					t.Errorf("ScreenShadows %v: shadow.Screen = %v", want, op.Shadow.Screen)
				}
			}
		}
		if !seen {
			t.Fatal("no shadow in the picture")
		}
	}
}
