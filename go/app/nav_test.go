//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

const screensText = `item Home { pos 0 0; width 100; height 50; auto-layout column; item Title { title Hi }; item Row; item Row }
item "Home 2" { pos 300 0; width 100; height 50 }
item Note
`

func loadText(t *testing.T, dir, text string) *scene.Tree {
	t.Helper()
	tree, err := scene.LoadSource(dir, "a.tenn", text)
	if tree == nil {
		t.Fatal(err)
	}
	return tree
}

func labels(rows []ItemRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Label+"="+r.Path)
	}
	return out
}

func TestSceneRows(t *testing.T) {
	root := loadText(t, t.TempDir(), screensText).File("a.tenn")
	rows := SceneRows(root, "")
	if got := labels(rows); !slices.Equal(got, []string{"Home=Home", "Home 2=Home_2", "Note=Note"}) {
		t.Fatalf("top rows %q", got)
	}
	if !rows[0].Kids || rows[1].Kids || rows[0].Kind != "item" {
		t.Errorf("kids or kind: %+v", rows)
	}
	kids := SceneRows(NodeAt(root, rows[0].Path), rows[0].Path)
	if got := labels(kids); !slices.Equal(got, []string{"Title=Home.Title", "Row=Home.Row", "Row=Home.Row#1"}) {
		t.Fatalf("kids %q", got)
	}
	for _, k := range kids {
		if NodeAt(root, k.Path) == nil {
			t.Errorf("%s does not name a node", k.Path)
		}
	}
}

func TestDiagramRows(t *testing.T) {
	e := model.NewElement("Map")
	a := e.AddGet(model.NewDiagramItem(model.ItemKindItem, "Idea"))
	b := e.AddGet(model.NewDiagramItem(model.ItemKindItem, ""))
	e.AddGet(model.NewLinkItem(model.ItemKindLink, "", a, b))
	rows := DiagramRows(e)
	if got := []string{rows[0].Label, rows[1].Label, rows[2].Label}; len(rows) != 3 || !slices.Equal(got, []string{"Idea", "(no name)", "Idea -> "}) {
		t.Fatalf("rows %+v", rows)
	}
	if rows[0].Item != a || rows[2].Kind != "link" {
		t.Errorf("rows %+v", rows)
	}
}

func TestDupNameAndFreeRight(t *testing.T) {
	taken := map[string]bool{"Screen": true, "Screen_2": true}
	for name, want := range map[string]string{"Screen": "Screen 3", "Screen 2": "Screen 3", "Login": "Login 2", "": ""} {
		if got := DupName(name, taken); got != want {
			t.Errorf("DupName(%q) = %q, want %q", name, got, want)
		}
	}
	at := Box{0, 0, 100, 50}
	if x := FreeRight(at, []Box{at}, 100); x != 200 {
		t.Errorf("alone: %v", x)
	}
	// a box in the way, and another past it that the first move runs into
	if x := FreeRight(at, []Box{at, {250, 20, 100, 100}, {420, -40, 50, 50}}, 100); x != 570 {
		t.Errorf("blocked: %v", x)
	}
	if x := FreeRight(at, []Box{at, {250, 160, 100, 100}}, 100); x != 200 {
		t.Errorf("a box below the row is not in the way: %v", x)
	}
}

func TestDuplicateEdit(t *testing.T) {
	dir := t.TempDir()
	tree := loadText(t, dir, screensText)
	root := tree.File("a.tenn")
	home, home2 := NodeAt(root, "Home"), NodeAt(root, "Home_2")
	src := []DupSource{{Node: home, At: &Box{0, 0, 100, 50}, Free: true, Others: []Box{{0, 0, 100, 50}, {300, 0, 100, 50}}}}
	e, starts, ok := DuplicateEdit(screensText, src)
	if !ok {
		t.Fatal("not made")
	}
	got := apply(screensText, e)
	after := loadText(t, dir, got)
	if errs := after.Diagnostics(); len(errs) > 0 {
		t.Fatalf("%v\n%s", errs, got)
	}
	// "Home 2" is taken and in the way: the copy is Home 3, right of it
	cp := nodeStartingAt(after.File("a.tenn"), starts[0])
	if cp == nil || cp.Name != "Home 3" || !strings.Contains(got, `item "Home 3" { pos 500 0; width 100`) {
		t.Fatalf("copy %v at %d:\n%s", cp, starts, got)
	}
	if NodePath(after.File("a.tenn"), cp) != "Home_3" || len(sceneKids(cp)) != 3 {
		t.Errorf("copy path %q, %d kids", NodePath(after.File("a.tenn"), cp), len(sceneKids(cp)))
	}

	// nested: in the same parent, no pos; two at once are one edit
	row, title := NodeAt(root, "Home.Row"), NodeAt(root, "Home.Title")
	e, starts, _ = DuplicateEdit(screensText, []DupSource{{Node: row}, {Node: title}, {Node: home2}})
	got = apply(screensText, e)
	after = loadText(t, dir, got)
	h := NodeAt(after.File("a.tenn"), "Home")
	var names []string
	for _, k := range sceneKids(h) {
		names = append(names, k.Name)
	}
	if !slices.Equal(names, []string{"Title", "Title 2", "Row", "Row 2", "Row"}) || len(starts) != 3 {
		t.Fatalf("nested copies %q:\n%s", names, got)
	}
	if NodeAt(after.File("a.tenn"), "Home_3") == nil {
		t.Errorf("Home 2 gives Home 3:\n%s", got)
	}
	for _, s := range starts {
		if nodeStartingAt(after.File("a.tenn"), s) == nil {
			t.Errorf("no copy starts at %d:\n%s", s, got)
		}
	}
	// a node inside another selected one goes with it
	if _, starts, _ := DuplicateEdit(screensText, []DupSource{{Node: home}, {Node: title}}); len(starts) != 1 {
		t.Errorf("inner node copied twice: %v", starts)
	}
}

const kitText = "item Card { width 40; height 20; auto-layout column; slot }\n" +
	"item Panel { auto-layout column; item head { slot title }; slot body { item hint } }\n" +
	"item Both { auto-layout column; slot; slot extra }\n" +
	"item Plain { width 10; height 10 }\n" +
	"item Group { item Inner { width 5; height 5 }; use Card as inst { item y } }\n"

func TestDuplicateUseAndUndo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kit.tenn"), []byte(kitText), 0o644)
	text := "element kit --from-file kit.tenn\nuse kit.Card as \"Доска - вид\" {\n  pos 10 -20\n  item a\n}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text)
	use := NodeAt(v.file(), "Доска___вид")
	if use == nil {
		t.Fatal("no use")
	}
	e, _, ok := DuplicateEdit(v.text, []DupSource{{Node: use, At: &Box{10, 20, 40, 20}, Free: true, Pos: [2]float64{10, -20}}})
	if !ok {
		t.Fatal("not made")
	}
	if err := v.applyText(e.Start, e.End, e.New); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v.text, "use kit.Card as \"Доска - вид 2\" {\n  pos 150 -20\n  item a\n}") || v.sceneErr != "" {
		t.Fatalf("copy of a use (%s):\n%s", v.sceneErr, v.text)
	}
	v.session.Undo()
	if v.text != text {
		t.Fatalf("one undo step:\n%s", v.text)
	}
}

func TestInsertTarget(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kit.tenn"), []byte(kitText), 0o644)
	text := "element kit --from-file kit.tenn\nitem Frame { auto-layout column }\nuse kit.Card as c\nuse kit.Panel as p\nuse kit.Both as b\nuse kit.Plain as q\n"
	os.WriteFile(filepath.Join(dir, "main.tenn"), []byte(text), 0o644)
	tree, err := scene.Load(dir, "main.tenn")
	if tree == nil {
		t.Fatal(err)
	}
	f := tree.File("main.tenn")
	cases := []struct {
		path  string
		into  string
		slots []string
	}{
		{"", "", nil},
		{"Frame", "Frame", nil},
		{"c", "c", nil},
		{"p", "p", []string{"title", "body"}},
		{"b", "b", []string{"", "extra"}},
		{"q", "", nil},
	}
	for _, c := range cases {
		into, slots := InsertTarget(tree, NodeAt(f, c.path))
		got := ""
		if into != nil {
			got = NodePath(f, into)
		}
		if got != c.into || !slices.Equal(slots, c.slots) {
			t.Errorf("%q: into %q slots %q, want %q %q", c.path, got, slots, c.into, c.slots)
		}
	}
	// what the rules pick loads without errors: the default slot, a named one
	for _, in := range []struct{ path, slot string }{{"c", ""}, {"p", "body"}, {"Frame", ""}} {
		e := InsertEdit(text, NodeAt(f, in.path), SlotCommand(in.slot, "use kit.Plain as x"))
		tr, _ := scene.LoadOverlay(dir, "main.tenn", map[string]string{"main.tenn": apply(text, e)})
		if errs := tr.Diagnostics(); len(errs) > 0 {
			t.Errorf("%s %q: %v", in.path, in.slot, errs)
		}
	}
}

func TestUsePathAndComponents(t *testing.T) {
	root := loadText(t, t.TempDir(), kitText).File("a.tenn")
	for path, want := range map[string]string{"Card": "kit.Card", "Group.Inner": "kit.Group.Inner", "Group.inst": "kit.Group.inst"} {
		if got, ok := UsePath("kit", NodeAt(root, path)); !ok || got != want {
			t.Errorf("%s: %q %v", path, got, ok)
		}
	}
	if _, ok := UsePath("kit", NodeAt(root, "Group.inst.y")); ok {
		t.Error("a node passed into an instance is not a component")
	}
	comps := Components("kit", root, "in", 10)
	comps = slices.DeleteFunc(comps, func(c Component) bool { return c.Name == "Plain" })
	var got []string
	for _, c := range comps {
		got = append(got, c.Label+"="+c.Path)
	}
	if !slices.Equal(got, []string{"Inner - Group=kit.Group.Inner", "inst - Group=kit.Group.inst"}) {
		t.Errorf("components %q", got)
	}
	if n := len(Components("kit", root, "", 2)); n != 2 {
		t.Errorf("limit: %d", n)
	}
}
