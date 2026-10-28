package scene_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// slotLib is a library split in two files: Panel (with slots of its own) lives in the nested mount,
// Host (the slot owner) in the file around it.
var slotLib = map[string]string{
	"lib/kit.tenn": "element Parts --from-file \"parts.tenn\"\n" +
		"item Host { auto-layout abs; width 500; height 400; clip yes; slot S }\n",
	"lib/parts.tenn": "item Panel {\n  auto-layout column; width 100; height 100; pos 0 %s\n" +
		"  item Head { height 20; slot Q { item Dflt { width 10; height 10 } } }\n" +
		"  item Body { height 40; slot }\n}\n" +
		"item Plain { width 30; height 30 }\n",
}

func slotScreen(t *testing.T, panelPos, content string) (*scene.Screen, *scene.Tree) {
	t.Helper()
	files := map[string]string{
		"lib/kit.tenn":   slotLib["lib/kit.tenn"],
		"lib/parts.tenn": fmt.Sprintf(slotLib["lib/parts.tenn"], panelPos),
		"main.tenn": "element Kit --from-file \"lib/kit.tenn\"\n" +
			"element Screen { use Kit.Host as H { slot S { " + content + " } } }\n",
	}
	tree, err := scene.Load(writeFiles(t, files), "main.tenn")
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

func boxMap(s *scene.Screen) map[string]scene.Box {
	m := map[string]scene.Box{}
	for _, b := range s.Boxes() {
		m[b.Path] = b
	}
	return m
}

func TestSlotHoldsUseOfComponentWithOwnSlots(t *testing.T) {
	s, tree := slotScreen(t, "0", "use Kit.Parts.Panel as P { width 300; height 200; slot Q { item Own { width 5; height 5 } } }")
	m := boxMap(s)
	wantBox(t, m, "H.P", 0, 0, 300, 200)
	wantBox(t, m, "H.P.Head.Own", 0, 0, 5, 5)
	if _, ok := m["H.P.Head.Dflt"]; ok {
		t.Error("the fallback of Q must be replaced")
	}
	if len(tree.Diagnostics()) > 0 || len(s.Warnings) > 0 {
		t.Errorf("diagnostics %v, warnings %v", tree.Diagnostics(), s.Warnings)
	}
}

// A component carries the pos of its place in a library; an instance in an abs parent inherits it
// and may land outside the clip of the parent. That is a warning, not a silent loss.
func TestSlotContentOutsideClipIsReported(t *testing.T) {
	s, _ := slotScreen(t, "-1480", "use Kit.Parts.Panel as P { width 300; height 200 }")
	var found bool
	for _, w := range s.Warnings {
		found = found || (w.Warn && w.Pos.File == "main.tenn" && w.Pos.Line == 2 && strings.Contains(w.Msg, "outside its clipping parent"))
	}
	if !found {
		t.Errorf("no warning for P, got %v", s.Warnings)
	}
	if _, ok := boxMap(s)["H.P"]; !ok {
		t.Error("P must stay in the tree")
	}
	// the use overrides pos: inside the clip, no warning
	s, _ = slotScreen(t, "-1480", "use Kit.Parts.Panel as P { pos 0 0; width 300; height 200 }")
	if len(s.Warnings) > 0 {
		t.Errorf("warnings %v", s.Warnings)
	}
}

// Whatever is written in a slot block ends up in the tree or comes back as a diagnostic.
func TestSlotContentIsNeverDropped(t *testing.T) {
	cases := []struct {
		name, content, box string
		fail               bool
	}{
		{"item", "item I { width 5; height 5 }", "H.I", false},
		{"use plain", "use Kit.Parts.Plain as I", "H.I", false},
		{"use with slots", "use Kit.Parts.Panel as I", "H.I", false},
		{"use with slots sized", "use Kit.Parts.Panel as I { width 9; height 9 }", "H.I", false},
		{"use filled", "use Kit.Parts.Panel as I { slot { item Z { width 1; height 1 } } }", "H.I", false},
		{"use missing", "use Kit.Parts.Nope as I", "", true},
		{"two uses", "use Kit.Parts.Plain as I; use Kit.Parts.Panel as J", "H.J", false},
		{"nested use in item", "item W { use Kit.Parts.Panel as I }", "H.W.I", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				"lib/kit.tenn":   slotLib["lib/kit.tenn"],
				"lib/parts.tenn": fmt.Sprintf(slotLib["lib/parts.tenn"], "0"),
				"main.tenn": "element Kit --from-file \"lib/kit.tenn\"\n" +
					"element Screen { use Kit.Host as H { slot S { " + c.content + " } } }\n",
			}
			tree, err := scene.Load(writeFiles(t, files), "main.tenn")
			if c.fail {
				if err == nil && len(tree.Diagnostics()) == 0 {
					t.Fatal("content that cannot be placed must give a diagnostic")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sc := gfx.NewScene()
			var screen *scene.Screen
			sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "Screen", scene.Options{}) })
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := boxMap(screen)[c.box]; !ok {
				t.Errorf("no box %s in %v and no diagnostic: %v", c.box, paths(screen), tree.Diagnostics())
			}
		})
	}
}
