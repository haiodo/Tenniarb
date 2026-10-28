//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

const stubDoc = `item Page {
  color white
  item Header {
    title Hi
    item Logo { color red }
  }
  item Row { item A; item B { gap 1 }; gap 2 }
  use kit.Button as ok {
    title Go
    at inner { item X { color red } }
  }
  item Row2
}
`

const stubPage = `item Page {
  color white
  item Header
  item Row
  use kit.Button as ok
  item Row2
}`

// merge is the file text after the Code section of the node at path was changed from OwnText to panel.
func merge(t *testing.T, text, path, panel string) string {
	t.Helper()
	return apply(text, ReplaceOwnEdit(text, nodeOf(t, text, path), panel))
}

func TestOwnTextStubsKids(t *testing.T) {
	cases := []struct{ path, want string }{
		{"Page", stubPage},
		{"Page.Header", "item Header {\n  title Hi\n  item Logo\n}"},
		{"Page.Row", "item Row { item A; item B; gap 2 }"},
		{"Page.ok", "use kit.Button as ok {\n  title Go\n  at inner { item X }\n}"},
		{"Page.Row2", "item Row2"},
	}
	for _, c := range cases {
		if got := OwnText(stubDoc, nodeOf(t, stubDoc, c.path)); got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.path, got, c.want)
		}
	}
}

func TestStubRoundTrip(t *testing.T) {
	cyr := "item Экран {\n  item Шапка { title Привет }\n  color red\n}\n"
	for _, text := range []string{stubDoc, cyr, "item P {\n  item A { x 1 }\n  item A { x 2 }\n}\n"} {
		loaded, _ := scene.LoadSource(t.TempDir(), "a.tenn", text)
		tree := loaded.File("a.tenn")
		var walk func(path string)
		walk = func(path string) {
			n := NodeAt(tree, path)
			if got := merge(t, text, path, OwnText(text, n)); got != text {
				t.Errorf("%s:\n%s", path, got)
			}
			for _, k := range sceneKids(n) {
				walk(NodePath(tree, k))
			}
		}
		for _, k := range sceneKids(tree) {
			walk(NodePath(tree, k))
		}
	}
}

func TestStubMerge(t *testing.T) {
	header := "  item Header {\n    title Hi\n    item Logo { color red }\n  }\n"
	row2 := "  item Row2\n"
	cases := []struct{ name, panel, want string }{
		{"own property", strings.Replace(stubPage, "white", "black", 1), strings.Replace(stubDoc, "white", "black", 1)},
		{"own property added", strings.Replace(stubPage, "  item Row2\n", "  item Row2\n  gap 4\n", 1), strings.Replace(stubDoc, "  item Row2\n", "  item Row2\n  gap 4\n", 1)},
		{"moved", strings.Replace(strings.Replace(stubPage, "  item Row2\n", "", 1), "  item Header\n", "  item Row2\n  item Header\n", 1),
			strings.Replace(strings.Replace(stubDoc, row2, "", 1), header, row2+header, 1)},
		{"deleted in the middle", strings.Replace(stubPage, "  item Row\n", "", 1), stubDoc},
		{"deleted first", strings.Replace(stubPage, "  item Header\n", "", 1), stubDoc},
		{"deleted last", strings.Replace(stubPage, "  item Row2\n", "", 1), stubDoc},
		{"renamed", strings.Replace(stubPage, "item Row2", "item Rowx", 1), stubDoc},
		{"new item", strings.Replace(stubPage, "  item Row\n", "  item Fresh { color blue }\n  item Row\n", 1), strings.Replace(stubDoc, "  item Row {", "  item Fresh { color blue }\n  item Row {", 1)},
		{"trailing newline", stubPage + "\n", stubDoc},
	}
	for _, c := range cases {
		if got := merge(t, stubDoc, "Page", c.panel); got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
	bare := strings.Replace(stubPage, "  item Row2\n", "  item Row2\n  item Z\n", 1)
	if got := merge(t, stubDoc, "Page", bare); got != strings.Replace(stubDoc, "  item Row2\n", "  item Row2\n  item Z\n", 1) {
		t.Errorf("new bare item:\n%s", got)
	}
}

func TestStubMergeNested(t *testing.T) {
	got := merge(t, stubDoc, "Page.Header", "item Header {\n  title Yo\n  item Logo\n}")
	if want := strings.Replace(stubDoc, "title Hi", "title Yo", 1); got != want {
		t.Errorf("nested:\n%s", got)
	}
	cyr := "item Экран {\n  item Шапка { title Привет }\n  color red\n}\n"
	if got, want := merge(t, cyr, "Экран", "item Экран {\n  item Шапка\n  color blue\n}"), strings.Replace(cyr, "red", "blue", 1); got != want {
		t.Errorf("cyrillic:\n%s", got)
	}
}

func TestStubMergeOneLine(t *testing.T) {
	got := merge(t, stubDoc, "Page.Row", "item Row { item A; item B; gap 3 }")
	if want := strings.Replace(stubDoc, "gap 2", "gap 3", 1); got != want {
		t.Errorf("edit:\n%s", got)
	}
	got = merge(t, stubDoc, "Page.Row", "item Row { item B; gap 2 }")
	if got != stubDoc {
		t.Errorf("deleted A:\n%s", got)
	}
	got = merge(t, stubDoc, "Page.Row", "item Row { gap 2; item B; item A }")
	if want := strings.Replace(stubDoc, "item A; item B { gap 1 }; gap 2", "gap 2; item B { gap 1 }; item A", 1); got != want {
		t.Errorf("moved:\n%s", got)
	}
}

func TestStubMergeDeletedAll(t *testing.T) {
	got := merge(t, stubDoc, "Page", "item Page {\n  color white\n}")
	for _, kid := range []string{"item Header {\n    title Hi\n    item Logo { color red }\n  }", "item Row { item A; item B { gap 1 }; gap 2 }", "use kit.Button as ok {\n    title Go\n    at inner { item X { color red } }\n  }", "item Row2"} {
		if !strings.Contains(got, kid) {
			t.Errorf("lost %q:\n%s", kid, got)
		}
	}
	if n := len(sceneKids(nodeOf(t, got, "Page"))); n != 4 {
		t.Errorf("%d kids:\n%s", n, got)
	}
}

func TestStubMergeSameName(t *testing.T) {
	text := "item P {\n  item A { x 1 }\n  item A { x 2 }\n}\n"
	if got := merge(t, text, "P", "item P {\n  item A\n  item A\n}"); got != text {
		t.Errorf("unchanged:\n%s", got)
	}
	// stubs of one name are matched by order: the first stub is the first kid
	if got := merge(t, text, "P", "item P {\n  item A\n  color red\n  item A\n}"); got != "item P {\n  item A { x 1 }\n  color red\n  item A { x 2 }\n}\n" {
		t.Errorf("property between:\n%s", got)
	}
	if got := merge(t, text, "P", "item P {\n  item A\n}"); got != text {
		t.Errorf("deleted one:\n%s", got)
	}
}

func TestFindStubs(t *testing.T) {
	stubs, root := findStubs(nodeOf(t, stubDoc, "Page"), stubPage)
	if root == nil || len(stubs) != 4 {
		t.Fatalf("%d stubs", len(stubs))
	}
	for i, name := range []string{"Header", "Row", "ok", "Row2"} {
		if s := stubs[i]; s.Kid.Name != name || !strings.HasSuffix(stubPage[s.Start:s.End], name) {
			t.Errorf("stub %d: %q for %s", i, stubPage[s.Start:s.End], s.Kid.Name)
		}
	}
	if stubs, root := findStubs(nodeOf(t, stubDoc, "Page"), "item Page {"); root != nil || stubs != nil {
		t.Errorf("text that does not parse: %v", stubs)
	}
}

// What penimport writes: tabs, and plain commands packed on one line.
const tabDoc = "item Page {\n\tcolor white; gap 1\n\titem Row {\n\t\tgap 1\n\t\titem A { title a }\n\t}\n\titem Empty\n}\n"

func TestEditsCopyTabIndent(t *testing.T) {
	row := nodeOf(t, tabDoc, "Page.Row")
	cases := []struct {
		name string
		e    TextEdit
		want string
	}{
		{"property into a block", InsertEdit(tabDoc, row, "color red"), strings.Replace(tabDoc, "\t\titem A { title a }\n", "\t\titem A { title a }\n\t\tcolor red\n", 1)},
		{"several lines into a block", InsertEdit(tabDoc, row, "item B {\n\ttitle b\n}"), strings.Replace(tabDoc, "\t\titem A { title a }\n", "\t\titem A { title a }\n\t\titem B {\n\t\t\ttitle b\n\t\t}\n", 1)},
		{"several lines into a node without block", InsertEdit(tabDoc, nodeOf(t, tabDoc, "Page.Empty"), "item B {\n\ttitle b\n}"), strings.Replace(tabDoc, "\titem Empty\n", "\titem Empty {\n\t\titem B {\n\t\t\ttitle b\n\t\t}\n\t}\n", 1)},
		{"property into a packed block", InsertEdit(tabDoc, nodeOf(t, tabDoc, "Page"), "width 9"), strings.Replace(tabDoc, "\titem Empty\n", "\titem Empty\n\twidth 9\n", 1)},
	}
	for _, c := range cases {
		if got := apply(tabDoc, c.e); got != c.want {
			t.Errorf("%s:\n%q\nwant\n%q", c.name, got, c.want)
		}
	}
	if got := indentUnit(tabDoc); got != "\t" {
		t.Errorf("indent unit %q", got)
	}
	// a command packed after another one goes with its ;, a line of its own with the line
	if e, _ := SetPropEdit(tabDoc, nodeOf(t, tabDoc, "Page"), "gap"); apply(tabDoc, e) != strings.Replace(tabDoc, "; gap 1", "", 1) {
		t.Errorf("remove from a packed line: %q", apply(tabDoc, e))
	}
	if e, _ := SetPropEdit(tabDoc, row, "gap"); apply(tabDoc, e) != strings.Replace(tabDoc, "\t\tgap 1\n", "", 1) {
		t.Errorf("remove a line: %q", apply(tabDoc, e))
	}
}

func TestCodeSectionKeepsTabIndent(t *testing.T) {
	row := nodeOf(t, tabDoc, "Page.Row")
	code := NodeText(tabDoc, row)
	if code != "item Row {\n\tgap 1\n\titem A { title a }\n}" {
		t.Fatalf("code %q", code)
	}
	got := apply(tabDoc, ReplaceNodeEdit(tabDoc, row, strings.Replace(code, "gap 1", "gap 2", 1)))
	if want := strings.Replace(tabDoc, "gap 1\n\t\titem", "gap 2\n\t\titem", 1); got != want {
		t.Errorf("replace:\n%q\nwant\n%q", got, want)
	}
	if got := merge(t, tabDoc, "Page.Row", strings.Replace(OwnText(tabDoc, row), "gap 1", "gap 3", 1)); got != strings.Replace(tabDoc, "\t\tgap 1\n", "\t\tgap 3\n", 1) {
		t.Errorf("own text:\n%q", got)
	}
}

// penimport shares a repeated subtree as a template; the copies are uses of it. A part of a copy is
// selected by the path of its use, and edited by an at override in that use.
const sharedDoc = "element tpl {\n\titem Card {\n\t\tauto-layout column; width 80\n\t\titem Title { title Hi; font-size 14 }\n\t\titem Row { item Cell }\n\t}\n}\n" +
	"item Page {\n\tauto-layout row\n\tuse tpl.Card as one\n\tuse tpl.Card as two { pos 5 -5 }\n}\n"

// viewOf draws the node of a loaded tree the way the canvas does.
func viewOf(t *testing.T, tree *scene.Tree, node *scene.Node) *sceneView {
	t.Helper()
	var screen *scene.Screen
	var err error
	gfx.NewScene().Update(func(tx *gfx.Tx) {
		screen, err = tree.BuildElement(tx, nil, node, scene.Options{Measurer: gfx.EstimateText{}})
	})
	if err != nil {
		t.Fatal(err)
	}
	sv := newSceneView(nil, node)
	sv.placed = screen.Placed()
	sv.index(sv.placed)
	return sv
}

// walkRows lists every row of the navigator, opening all of them.
func walkRows(v *Viewer, rows []ItemRow, out *[]ItemRow) {
	for _, r := range rows {
		*out = append(*out, r)
		if r.Kids {
			walkRows(v, v.rowsAt(r.Path), out)
		}
	}
}

// Every row of the navigator selects something that is drawn, in a plain element and inside an instance.
func TestNavigatorRowsSelect(t *testing.T) {
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", sharedDoc)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.File("a.tenn")
	v := &Viewer{docCore: &docCore{}, sv: viewOf(t, tree, file)}
	var rows []ItemRow
	walkRows(v, v.itemRows(), &rows)
	var got []string
	for _, r := range rows {
		got = append(got, r.Path)
		v.selNodes = []string{r.Path}
		if _, pl := v.selectedScene(); pl == nil {
			t.Errorf("row %s selects nothing", r.Path)
		}
		if c := v.sv.rowChain(r.Path); c[len(c)-1] != r.Path {
			t.Errorf("row %s: chain %v", r.Path, c)
		}
	}
	want := []string{"Page", "Page.one", "Page.one/Title", "Page.one/Row", "Page.one/Row.Cell", "Page.two", "Page.two/Title", "Page.two/Row", "Page.two/Row.Cell"}
	if !slices.Equal(got, want) {
		t.Fatalf("rows %q\nwant %q", got, want)
	}
	v.selNodes = []string{"Page.two/Row.Cell"}
	if n, pl := v.selectedScene(); n != nil || pl == nil || pl.Src.Key != "Cell" {
		t.Errorf("selected %v %v", n, pl)
	}
	if ref, ok := v.selectedInst(); !ok || ref.at != "Row.Cell" || ref.use.Name != "two" {
		t.Errorf("ref %+v", ref)
	}
}

func TestEditOfACopyStaysInItsUse(t *testing.T) {
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", sharedDoc)
	if err != nil {
		t.Fatal(err)
	}
	sv := viewOf(t, tree, tree.File("a.tenn"))
	text := sharedDoc
	titleOf := func(path string) instRef { return sv.inst[sv.byPath[path]] }
	ref := titleOf("Page.two/Title")
	e, _ := AtPropEdit(text, ref.use, ref.at, "title", "Bye")
	text1 := apply(text, e)
	if want := strings.Replace(text, "pos 5 -5 }", "pos 5 -5; at Title { title Bye } }", 1); text1 != want {
		t.Fatalf("first edit:\n%s", text1)
	}
	// the same part again: its at is updated in place, and a second property goes into it
	reload := func(text string) {
		tree, err = scene.LoadSource(t.TempDir(), "a.tenn", text)
		if err != nil {
			t.Fatal(err)
		}
		sv = viewOf(t, tree, tree.File("a.tenn"))
	}
	reload(text1)
	ref = titleOf("Page.two/Title")
	e, _ = AtPropEdit(text1, ref.use, ref.at, "title", "Again")
	text2 := apply(text1, e)
	ref = titleOf("Page.two/Title")
	reload(text2)
	ref = titleOf("Page.two/Title")
	e, _ = AtPropEdit(text2, ref.use, ref.at, "font-size", "20")
	text3 := apply(text2, e)
	if !strings.Contains(text3, "at Title { title Again; font-size 20 }") || strings.Count(text3, "at Title") != 1 {
		t.Errorf("update in place:\n%s", text3)
	}
	// the template and the other copy are as they were
	if !strings.HasPrefix(text3, sharedDoc[:strings.Index(sharedDoc, "use tpl.Card as two")]) {
		t.Errorf("the template or the first copy changed:\n%s", text3)
	}
	reload(text3)
	if p := sv.byPath["Page.one/Title"].Prop("title"); p == nil || p.Values()[0] != "Hi" {
		t.Errorf("the other copy shows %v", p)
	}
	if p := sv.byPath["Page.two/Title"].Prop("font-size"); p == nil || p.Values()[0] != "20" {
		t.Errorf("the copy shows %v", p)
	}
	// removing the last property of a part leaves the at, and the part shows the component's value
	ref = titleOf("Page.two/Title")
	e, ok := AtPropEdit(text3, ref.use, ref.at, "font-size")
	if got := apply(text3, e); !ok || strings.Contains(got, "font-size 20") {
		t.Errorf("remove:\n%s", got)
	}
	// a part with no unique name cannot be addressed
	if r := titleOf("Page.one/Row"); r.at != "Row" {
		t.Errorf("row %+v", r)
	}
}

// A part reached through a slot is named through the slot.
func TestAtPathThroughASlot(t *testing.T) {
	doc := "element tpl {\n\titem Box { item Head; slot body { item Fallback { title a } } }\n}\nuse tpl.Box as b\n"
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", doc)
	if err != nil {
		t.Fatal(err)
	}
	sv := viewOf(t, tree, tree.File("a.tenn"))
	ref := sv.inst[sv.byPath["b/Fallback"]]
	if ref.at != "body.Fallback" {
		t.Fatalf("at %q", ref.at)
	}
	e, _ := AtPropEdit(doc, ref.use, ref.at, "title", "z")
	got := apply(doc, e)
	if _, err := scene.LoadSource(t.TempDir(), "a.tenn", got); err != nil {
		t.Errorf("%v\n%s", err, got)
	}
}

// The items of a mounted element are written in the file the element mounts, not in the element.
func TestMountedElementRowsSelect(t *testing.T) {
	dir := t.TempDir()
	lib := "item Forms {\n  item Toggle { title T }\n  item Header\n}\nitem Note\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	main := "element \"kit\" --from-file \"kit.lib.tenn\"\nelement \"S\" { item A { pos 0 0 } }\n"
	doc := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), main)
	node := doc.elementNode(doc.session.Root().Elements[0])
	v := &Viewer{docCore: &docCore{}, sv: viewOf(t, doc.stree, node)}
	var rows []ItemRow
	walkRows(v, v.itemRows(), &rows)
	if len(rows) != 4 {
		t.Fatalf("rows %v", labels(rows))
	}
	for _, r := range rows {
		v.selNodes = []string{r.Path}
		if n, pl := v.selectedScene(); n == nil || pl == nil {
			t.Errorf("row %s selects nothing", r.Path)
		}
	}
}

// The inspector write goes through sceneEdit as one undo step; the template and the other copy keep
// their bytes.
func TestInstancePropEditUndo(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "a.tenn"), sharedDoc)
	v.sv = viewOf(t, v.stree, v.file())
	ref := v.sv.inst[v.sv.byPath["Page.two/Title"]]
	if !v.setInstProp(ref, "title", "Bye") {
		t.Fatal("not applied")
	}
	if want := strings.Replace(sharedDoc, "pos 5 -5 }", "pos 5 -5; at Title { title Bye } }", 1); v.text != want {
		t.Fatalf("text:\n%s", v.text)
	}
	v.session.Undo()
	if v.text != sharedDoc {
		t.Errorf("undo:\n%s", v.text)
	}
}
