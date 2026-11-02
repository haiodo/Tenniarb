package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// replaceSlotLib is a component whose part carries a slot: a replace puts another node in its place,
// and the slot of that node is filled from the block of the use (spec 4.7a).
const replaceSlotLib = `item "Card" {
    auto-layout column
    width 100; height 100
    item "Body" {
        auto-layout column
        width 50; height 50
        slot "content" { item "Fallback" { width 1; height 1 } }
    }
}
`

func replaceSlotScreen(t *testing.T, block string) (*scene.Screen, *scene.Tree) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "r.tenn",
		replaceSlotLib+"element \"Screen\" {\n  use Card as c {\n"+block+"\n  }\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "Screen", scene.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	return screen, tree
}

func TestReplaceSlotFilledFromOutside(t *testing.T) {
	replace := `    replace Body {
      item "Box" {
        auto-layout column
        width 40; height 40
        slot "content" { item "OwnFallback" { width 2; height 2 } }
      }
    }`
	cases := []struct {
		name, fill, want, gone string
		w                      float64
	}{
		{"filled", `slot content { item "Passed" { width 3; height 3 } }`, "c.Box.Passed", "OwnFallback", 3},
		{"fallback", ``, "c.Box.OwnFallback", "c.Box.Passed", 2},
		{"emptied", `slot content {}`, "", "OwnFallback", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			block := replace
			if c.fill != "" {
				block += "\n    " + c.fill
			}
			screen, tree := replaceSlotScreen(t, block)
			m := boxMap(screen)
			if _, ok := m["c.Body"]; ok {
				t.Errorf("Body was not replaced: %v", paths(screen))
			}
			if c.want != "" {
				wantBox(t, m, c.want, 0, 0, c.w, c.w)
			}
			if c.gone != "" {
				for p := range m {
					if strings.HasSuffix(p, c.gone) {
						t.Errorf("%s is drawn: %v", c.gone, paths(screen))
					}
				}
			}
			if len(tree.Diagnostics()) > 0 || len(screen.Warnings) > 0 {
				t.Errorf("diagnostics %v, warnings %v", tree.Diagnostics(), screen.Warnings)
			}
		})
	}
}

// at reaches the fallback of a slot the replace declared, as it does in the component.
func TestReplaceSlotFallbackReachedByAt(t *testing.T) {
	screen, _ := replaceSlotScreen(t, `    replace Body {
      item "Box" {
        auto-layout column
        width 40; height 40
        slot "content" { item "OwnFallback" { width 2; height 2 } }
      }
    }
    at Box.content.OwnFallback { width 8 }`)
	wantBox(t, boxMap(screen), "c.Box.OwnFallback", 0, 0, 8, 2)
}

// A fill goes into the slot the replace declared, not one of the same name the component declares.
func TestReplaceSlotWinsOverComponentSlot(t *testing.T) {
	lib := `item "Card" {
    auto-layout column
    width 100; height 100
    item "Body" {
        auto-layout column
        width 50; height 50
        slot "content" { item "Fallback" { width 1; height 1 } }
    }
    slot "content" { item "RootFallback" { width 4; height 4 } }
}
`
	tree, err := scene.LoadSource(t.TempDir(), "r.tenn", lib+`element "Screen" {
  use Card as c {
    replace Body {
      item "Box" {
        auto-layout column
        width 40; height 40
        slot "content" { item "OwnFallback" { width 2; height 2 } }
      }
    }
    slot content { item "Passed" { width 3; height 3 } }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "Screen", scene.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	m := boxMap(screen)
	wantBox(t, m, "c.Box.Passed", 0, 0, 3, 3)
	wantBox(t, m, "c.RootFallback", 0, 40, 4, 4)
	if _, ok := m["c.Box.OwnFallback"]; ok {
		t.Errorf("the slot of the component got the fill: %v", paths(screen))
	}
}
