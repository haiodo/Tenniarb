//go:build darwin || linux || windows

package app

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

const rowLib = "item Card {\n\tauto-layout column; width 100; height 70\n\titem Title { width 100; height 20 }\n" +
	"\titem Body { auto-layout row; width 100; height 20; item Cell { width 10; height 10 } }\n\titem Panel { auto-layout abs; width 100; height 30; slot extra }\n}\n" +
	"item Badge { width 30; height 10 }\n"

// The app file mounts the library itself, as a screen file of a design does, and the document mounts both.
const rowApp = "element kit --from-file \"lib.tenn\"\nelement Boards {\n" +
	"\titem Screen {\n\t\tauto-layout abs; width 400; height 300\n" +
	"\t\tuse kit.Card as Top { pos 10 -10 }\n" +
	"\t\titem Inner {\n\t\t\tauto-layout abs; width 200; height 100; pos 0 -120\n" +
	"\t\t\tuse kit.Badge as Tag { pos 5 -5 }\n\t\t\tuse kit.Badge as Tag { pos 50 -5 }\n\t\t}\n" +
	"\t}\n" +
	"\tuse kit.Card as Second {\n\t\tpos 500 -10\n" +
	"\t\tslot extra {\n\t\t\titem Wrap {\n\t\t\t\tauto-layout abs; width 60; height 30\n\t\t\t\tuse kit.Badge as Tag { pos 2 -2 }\n\t\t\t\titem Dup { width 5; height 5 }\n\t\t\t\titem Dup { width 5; height 5; pos 10 -10 }\n\t\t\t}\n\t\t}\n" +
	"\t}\n}\n"

func rowSceneView(t *testing.T) (*Viewer, *sceneView) {
	t.Helper()
	dir := t.TempDir()
	writeTenn(t, dir, "lib.tenn", rowLib)
	writeTenn(t, dir, "app.tenn", rowApp)
	main := "element kit --from-file \"lib.tenn\"\nelement App --from-file \"app.tenn\"\n"
	writeTenn(t, dir, "main.tenn", main)
	tree, err := scene.Load(dir, filepath.Join(dir, "main.tenn"))
	if err != nil {
		t.Fatal(err)
	}
	var boards *scene.Node
	for _, e := range tree.File("main.tenn").Elems {
		if e.Name == "App" {
			for _, c := range e.Elems {
				if c.Name == "Boards" {
					boards = c
				}
			}
		}
	}
	if boards == nil {
		t.Fatal("no Boards in the mounted App")
	}
	sv := viewOf(t, tree, boards)
	return &Viewer{docCore: &docCore{}, sv: sv}, sv
}

// nestRows lists every row with the paths of the rows above it, as the tree nests them.
func nestRows(v *Viewer, rows []ItemRow, above []string, out map[string][]string) {
	for _, r := range rows {
		chain := append(slices.Clone(above), r.Path)
		out[r.Path] = chain
		if r.Kids {
			nestRows(v, v.rowsAt(r.Path), chain, out)
		}
	}
}

// Every row, in the document, in a file it mounts that mounts the same library again, in an instance and
// in what a use passes into its slot, selects the node drawn for it, with the box where it is drawn, and
// has the row chain that the tree nests.
func TestEveryRowSelectsWhatIsDrawn(t *testing.T) {
	v, sv := rowSceneView(t)
	nested := map[string][]string{}
	nestRows(v, v.itemRows(), nil, nested)
	for _, want := range []string{"Screen", "Screen.Top", "Screen.Top/Title", "Screen.Top/Body.Cell", "Screen.Inner.Tag#1",
		"Second", "Second/Body", "Second/Panel", "Second.Wrap", "Second.Wrap.Tag", "Second.Wrap.Dup#1"} {
		if nested[want] == nil {
			t.Errorf("no row %s in %v", want, slices.Sorted(maps.Keys(nested)))
		}
	}
	for path, chain := range nested {
		v.selNodes = []string{path}
		_, pl := v.selectedScene()
		if pl == nil || pl != sv.byPath[path] {
			t.Errorf("row %s selects %v", path, pl)
			continue
		}
		if got := sv.rowChain(path); !slices.Equal(got, chain) {
			t.Errorf("row %s: chain %q, the tree nests %q", path, got, chain)
		}
		if last := path[strings.LastIndexAny(path, "./")+1:]; !strings.HasPrefix(last, pl.Src.Key) && !strings.HasPrefix(last, "Cell") {
			t.Errorf("row %s selects the item %q", path, pl.Src.Key)
		}
		// The centre of the box hits this item or one drawn inside it: the frame lies where it is drawn.
		c := sv.hitScene(gfx.Point{X: pl.X + pl.W/2, Y: pl.Y + pl.H/2})
		for ; c != nil && c != pl; c = c.Parent {
		}
		if c == nil {
			t.Errorf("row %s: box %v,%v %vx%v is not where it is drawn", path, pl.X, pl.Y, pl.W, pl.H)
		}
	}
}

// A click on an instance selects the instance, a double click goes one level in, and with something inside
// selected a click selects what is under the pointer there.
func TestPickInstanceFirst(t *testing.T) {
	_, sv := rowSceneView(t)
	cell := sv.byPath["Screen.Top/Body.Cell"]
	top, body := sv.byPath["Screen.Top"], sv.byPath["Screen.Top/Body"]
	if got := sv.pick(sv.hitScene(gfx.Point{X: cell.X + 1, Y: cell.Y + 1}), nil, 1); got != top {
		t.Fatalf("first click selects %q, want the use Screen.Top", sv.pathOf[got])
	}
	if got := sv.pick(cell, top, 1); got != top {
		t.Errorf("click on the selected use selects %q", sv.pathOf[got])
	}
	if got := sv.pick(cell, top, 2); got != body {
		t.Errorf("double click selects %q, want Screen.Top/Body", sv.pathOf[got])
	}
	if got := sv.pick(cell, body, 2); got != cell {
		t.Errorf("second double click selects %q, want the Cell", sv.pathOf[got])
	}
	title := sv.byPath["Screen.Top/Title"]
	if got := sv.pick(title, body, 1); got != title {
		t.Errorf("inside the instance a click selects %q", sv.pathOf[got])
	}
	screen := sv.byPath["Screen"]
	if got := sv.pick(screen, nil, 1); got != screen {
		t.Errorf("an item that is no instance is selected as it is: %q", sv.pathOf[got])
	}
	if got := sv.pick(sv.byPath["Screen.Inner.Tag"], screen, 1); got != sv.byPath["Screen.Inner.Tag"] {
		t.Errorf("a use in an item selects the use: %q", sv.pathOf[got])
	}
}

func TestInstanceRowIsGrey(t *testing.T) {
	if !instanceRow(&ItemRow{Kind: "use", Path: "A"}) || instanceRow(&ItemRow{Kind: "item", Path: "A.B"}) || !instanceRow(&ItemRow{Kind: "item", Path: "A/B"}) {
		t.Error("use and parts are grey, written items are not")
	}
}
