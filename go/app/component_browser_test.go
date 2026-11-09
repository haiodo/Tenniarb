//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
)

// importC4 writes a document that mounts a copy of libraries/c4.tenn (a library has to be inside the
// workspace root, as the app insists), and opens it.
func importC4(t *testing.T) *Viewer {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(root, "libraries", "c4.tenn"))
	if err != nil {
		t.Skip("libraries/c4.tenn is not here")
	}
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "c4.tenn"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	text := "element \"c4\" --from-file \"lib/c4.tenn\"\n" +
		"element \"Screen\" {\n    item Frame { width 300; height 200 }\n}\n"
	path := filepath.Join(dir, "browser.tenn")
	v := newDoc(t, &Shell{dir: dir}, path, text)
	if v.stree == nil {
		t.Fatal("the document has no scene tree")
	}
	e := v.state.Current
	v.session.SetElement(e)
	sv := newSceneView(e, v.elementNode(e))
	v.scene = gfx.NewScene()
	if err := v.buildSceneView(sv); err != nil {
		t.Fatal(err)
	}
	v.sv = sv
	// A mounted library is not a canvas of its own: the document's own element is.
	for _, e := range v.state.Doc.Model.Elements {
		if e.Name != "Screen" {
			continue
		}
		v.session.SetElement(e)
		sv := newSceneView(e, v.elementNode(e))
		v.scene = gfx.NewScene()
		if err := v.buildSceneView(sv); err != nil {
			t.Fatal(err)
		}
		v.sv = sv
	}
	return v
}

// The pane of the browser lists what the libraries offer when it is put in that mode, and a component
// dragged out of it writes its use where it lands.
func TestComponentBrowserRowsAndDrop(t *testing.T) {
	v := importC4(t)
	if len(v.libraries()) == 0 {
		f := v.file()
		if f == nil {
			t.Fatal("no file node")
		}
		for _, e := range f.Elems {
			t.Logf("element %q mount=%v", e.Name, e.Mount != nil)
		}
		t.Fatal("the library is not mounted")
	}
	v.setItemsMode(true)
	rows := v.itemRows()
	if len(rows) < 11 {
		t.Fatalf("%d component rows, want the eleven of libraries/c4.tenn", len(rows))
	}
	var person *ItemRow
	for i := range rows {
		if rows[i].Path == "c4.Person" {
			person = &rows[i]
		}
		if !rows[i].Component || rows[i].Kind != "component" {
			t.Errorf("row %q is %q, not a component", rows[i].Label, rows[i].Kind)
		}
	}
	if person == nil {
		t.Fatalf("no c4.Person among %v", rows)
	}

	// A drop carries the path of the row: the prefix is how the canvas tells it from a text drop.
	if !v.takeComponentDrop(componentDragPrefix+"c4.Person", 10, 10) {
		t.Fatal("the drop of a component was not taken")
	}
	text := v.fileText("browser.tenn")
	if !strings.Contains(text, "use c4.Person as Person") {
		t.Errorf("the use is not in the document:\n%s", text)
	}
	if !strings.Contains(text, "pos ") {
		t.Errorf("the dropped use has no position of the drop:\n%s", text)
	}
}

// A text drop that is not a component is left to the other handlers, and a component the library does not
// offer writes nothing.
func TestComponentDropIgnoresOtherText(t *testing.T) {
	v := importC4(t)
	v.setItemsMode(true)
	if v.takeComponentDrop("hello", 0, 0) {
		t.Error("a plain text drop was taken as a component")
	}
	before := v.fileText("browser.tenn")
	v.takeComponentDrop(componentDragPrefix+"c4.Nothing", 0, 0)
	if got := v.fileText("browser.tenn"); got != before {
		t.Errorf("a component the library does not offer was written:\n%s", got)
	}
}

// The browser lists nothing when no library is mounted, and the pane says so instead of an empty list.
func TestComponentBrowserWithoutLibrary(t *testing.T) {
	dir := t.TempDir()
	v := newDoc(t, &Shell{dir: dir}, filepath.Join(dir, "plain.tenn"),
		"element \"E\" { item A { width 10; height 10 } }\n")
	v.setItemsMode(true)
	rows := v.itemRows()
	if len(rows) != 1 || rows[0].Kind != "hint" || rows[0].Component {
		t.Fatalf("the pane without a library: %v, want one hint row", rows)
	}
	if !strings.Contains(rows[0].Label, "Add Library") {
		t.Errorf("the hint says %q", rows[0].Label)
	}
}

// A double click on a component row puts the component on the canvas, at the free spot the button picks.
func TestComponentRowActivateInserts(t *testing.T) {
	v := importC4(t)
	v.setItemsMode(true)
	if rows := v.itemRows(); len(rows) == 0 {
		t.Fatal("no component rows")
	}
	// activateItem reads the row from the node data, and the tree of the pane is what a click comes from:
	// the row itself is enough for the branch under test.
	v.activateItemRow(ItemRow{Label: "Person", Kind: "component", Path: "c4.Person", Component: true}, nil)
	if text := v.fileText("browser.tenn"); !strings.Contains(text, "use c4.Person as Person") {
		t.Errorf("a double click did not insert the component:\n%s", text)
	}
}
