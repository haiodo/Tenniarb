package scene

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/gowt/gfx"
)

// A classic diagram takes the default look of its element from styles { item } and styles { line }, as
// go/render parses those two names for every box and link instead of asking a node to use-style them.
func TestClassicDefaultStyles(t *testing.T) {
	root := t.TempDir()
	text := `element "E" {
  styles {
    item { shadow -5 -5 5; color #ff0000 }
    line { color #00ff00 }
  }
  item A { pos 0 0; title "A" }
  item B { pos 100 0; title "B"; shadow 0 0 2 }
  link A B
}
`
	if err := os.WriteFile(filepath.Join(root, "e.tenn"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := Load(root, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	var sc *Screen
	gs := gfx.NewScene()
	gs.Update(func(tx *gfx.Tx) {
		sc, err = tree.Build(tx, nil, "E", Options{Measurer: gfx.DefaultMeasurer})
	})
	if err != nil || sc == nil {
		t.Fatalf("build: %v", err)
	}
	items := map[string]style{}
	var linkCol gfx.Color
	var linkOK bool
	var walk func(rn *RNode)
	walk = func(rn *RNode) {
		switch rn.Kind {
		case RItem:
			items[rn.Name] = sc.p.styles[rn]
		case RLink:
			linkCol, _ = sc.p.linkStyle(rn)
			linkOK = true
		}
		for _, k := range rn.Kids {
			walk(k)
		}
	}
	walk(sc.tree)
	if sh := items["A"].shadows; len(sh) != 1 {
		t.Errorf("A has %d shadows, want the one of styles { item }", len(sh))
	}
	if got := items["A"].fill; got.R != 1 || got.G != 0 || got.B != 0 {
		t.Errorf("A fill %+v, want the red of styles { item }", got)
	}
	if sh := items["B"].shadows; len(sh) != 1 {
		t.Errorf("B has %d shadows, want only its own", len(sh))
	}
	if !linkOK || linkCol.G != 1 || linkCol.R != 0 {
		t.Errorf("link colour %+v, want the green of styles { line }", linkCol)
	}
}
