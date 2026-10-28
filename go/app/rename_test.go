//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

const (
	renameKit  = "element Parts {\n  item Button { width 40; height 20 }\n}\nelement Demo {\n  use Parts.Button as a\n}\n"
	renameMain = "element kit --from-file kit.tenn\n" +
		"element Screen {\n  use kit.Parts.Button as b\n  use kit.Demo.a as c\n}\n" +
		"element Other {\n  item Parts { }\n  use Parts as p\n}\n"
)

func renameDoc(t *testing.T, files map[string]string, main string) *Viewer {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return newDoc(t, &Shell{}, filepath.Join(dir, main), files[main])
}

func mustRename(t *testing.T, v *Viewer, plan *renamePlan, err error, update bool) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if err := v.applyRename(plan, update); err != nil {
		t.Fatal(err)
	}
}

// An element is renamed with the paths that go through it, in its own file and in the one that mounts
// it; a path of the same name in another scope stays. One undo brings every file back byte for byte.
func TestRenameElementUpdatesReferences(t *testing.T) {
	v := renameDoc(t, map[string]string{"main.tenn": renameMain, "kit.tenn": renameKit}, "main.tenn")
	parts := v.session.Root().Elements[0].Elements[0]
	plan, err := v.planElementRename(parts, "Big Parts")
	if err == nil && len(plan.refs) != 2 {
		t.Fatalf("references %d", len(plan.refs))
	}
	lines := v.refLines(plan.refs, 1)
	if !strings.HasPrefix(lines, "kit.tenn:5  use Parts.Button as a\n") || !strings.HasSuffix(lines, "and 1 more") {
		t.Errorf("dialog lines:\n%s", lines)
	}
	mustRename(t, v, plan, err, true)
	wantMain := strings.Replace(renameMain, "kit.Parts.Button", "kit.Big_Parts.Button", 1)
	wantKit := strings.Replace(strings.Replace(renameKit, "element Parts", `element "Big Parts"`, 1), "use Parts.Button", "use Big_Parts.Button", 1)
	if v.text != wantMain || v.libText("kit.tenn") != wantKit || v.sceneErr != "" {
		t.Fatalf("main:\n%s\nkit:\n%s\nerrors %q", v.text, v.libText("kit.tenn"), v.sceneErr)
	}
	if parts.Name != "Big Parts" {
		t.Errorf("the element keeps its identity, got %q", parts.Name)
	}
	v.session.Undo()
	if v.text != renameMain || v.libText("kit.tenn") != renameKit || parts.Name != "Parts" || v.sceneErr != "" {
		t.Fatalf("undo:\n%s\nkit:\n%s", v.text, v.libText("kit.tenn"))
	}
	v.session.Redo()
	if v.text != wantMain || v.libText("kit.tenn") != wantKit {
		t.Fatalf("redo:\n%s\nkit:\n%s", v.text, v.libText("kit.tenn"))
	}
}

// The alias of a mounted file is an element too: its line and every path that starts with it change.
func TestRenameMountAlias(t *testing.T) {
	v := renameDoc(t, map[string]string{"main.tenn": renameMain, "kit.tenn": renameKit}, "main.tenn")
	kit := v.session.Root().Elements[0]
	plan, err := v.planElementRename(kit, "lib")
	if err == nil && len(plan.refs) != 2 {
		t.Fatalf("references %d", len(plan.refs))
	}
	mustRename(t, v, plan, err, true)
	want := strings.NewReplacer("element kit ", `element "lib" `, "kit.Parts", "lib.Parts", "kit.Demo", "lib.Demo").Replace(renameMain)
	if v.text != want || v.sceneErr != "" || v.libText("kit.tenn") != renameKit {
		t.Fatalf("main:\n%s\nerrors %q", v.text, v.sceneErr)
	}
	v.session.Undo()
	if v.text != renameMain || v.libText("kit.tenn") != renameKit {
		t.Fatalf("undo:\n%s", v.text)
	}
}

// Rename only leaves the references as they are, which the scene then reports.
func TestRenameOnlyLeavesReferences(t *testing.T) {
	v := renameDoc(t, map[string]string{"main.tenn": renameMain, "kit.tenn": renameKit}, "main.tenn")
	parts := v.session.Root().Elements[0].Elements[0]
	plan, err := v.planElementRename(parts, "Pieces")
	mustRename(t, v, plan, err, false)
	if v.text != renameMain || !strings.Contains(v.libText("kit.tenn"), `element "Pieces"`) || !strings.Contains(v.libText("kit.tenn"), "use Parts.Button") {
		t.Fatalf("main:\n%s\nkit:\n%s", v.text, v.libText("kit.tenn"))
	}
	if v.sceneErr == "" {
		t.Error("the references to the old name must show as errors")
	}
	v.session.Undo()
	if v.libText("kit.tenn") != renameKit || v.sceneErr != "" {
		t.Fatalf("undo:\n%s", v.libText("kit.tenn"))
	}
}

const renameScene = `element S {
  item Card { item title { }; item body { } }
  use Card as c1 { at title { color red } }
  item A { pos 0 0 }
  item B { pos 100 0 }
  link A B
  link B A
}
`

func sceneNode(t *testing.T, v *Viewer, path string) *scene.Node {
	t.Helper()
	n, err := v.stree.Resolve(v.file(), path)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// A node is renamed with the at paths that reach it and the link ends next to it.
func TestRenameNodeUpdatesAtAndLinks(t *testing.T) {
	v := renameDoc(t, map[string]string{"m.tenn": renameScene}, "m.tenn")
	plan, err := v.planNodeRename(sceneNode(t, v, "S.Card.title"), "heading")
	mustRename(t, v, plan, err, true)
	want := strings.NewReplacer("item title", "item heading", "at title", "at heading").Replace(renameScene)
	if v.text != want || v.sceneErr != "" {
		t.Fatalf("text:\n%s\nerrors %q", v.text, v.sceneErr)
	}
	plan, err = v.planNodeRename(sceneNode(t, v, "S.A"), "Alpha beta")
	if err == nil && len(plan.refs) != 2 {
		t.Fatalf("link ends %d", len(plan.refs))
	}
	mustRename(t, v, plan, err, true)
	want = strings.NewReplacer("item A ", `item "Alpha beta" `, "link A B", `link "Alpha beta" B`, "link B A", `link B "Alpha beta"`).Replace(want)
	if v.text != want || v.sceneErr != "" {
		t.Fatalf("text:\n%s\nerrors %q", v.text, v.sceneErr)
	}
	v.session.Undo()
	v.session.Undo()
	if v.text != renameScene {
		t.Fatalf("two undos:\n%s", v.text)
	}
}

// A part opened in the context of its entry: the references in the entry and in a sibling part follow
// a rename in the library.
func TestRenameAcrossFilesThroughEntry(t *testing.T) {
	files := map[string]string{
		"design.tenn": "element ui --from-file ui.tenn\nelement p --from-file part.tenn\nelement q --from-file other.tenn\nelement R { use ui.Button as r }\n",
		"ui.tenn":     "item Button { width 40 }\n",
		"part.tenn":   "element Screen { use ui.Button as b }\n",
		"other.tenn":  "element Screen2 { use ui.Button as b }\n",
	}
	v := renameDoc(t, files, "part.tenn")
	if filepath.Base(v.entry) != "design.tenn" {
		t.Fatalf("entry %q", v.entry)
	}
	btn, err := v.stree.Resolve(v.stree.File("design.tenn"), "ui.Button")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := v.planNodeRename(btn, "Primary")
	if err == nil && len(plan.refs) != 3 {
		t.Fatalf("references %d", len(plan.refs))
	}
	mustRename(t, v, plan, err, true)
	for file, want := range map[string]string{
		"ui.tenn":     "item Primary { width 40 }\n",
		"design.tenn": strings.Replace(files["design.tenn"], "ui.Button", "ui.Primary", 1),
		"other.tenn":  strings.Replace(files["other.tenn"], "ui.Button", "ui.Primary", 1),
	} {
		if got := v.fileText(file); got != want {
			t.Errorf("%s:\n%s", file, got)
		}
	}
	if v.text != strings.Replace(files["part.tenn"], "ui.Button", "ui.Primary", 1) || v.sceneErr != "" {
		t.Fatalf("part:\n%s\nerrors %q", v.text, v.sceneErr)
	}
	v.session.Undo()
	for file, want := range files {
		if got := v.fileText(file); got != want {
			t.Errorf("undo %s:\n%s", file, got)
		}
	}
}

// A name that would take over the paths of something else is refused before anything changes.
func TestRenameRefusedWhenItCapturesReferences(t *testing.T) {
	text := "item Card { width 10 }\nelement S {\n  item Title { width 5 }\n  use Card as c\n}\n"
	v := renameDoc(t, map[string]string{"m.tenn": text}, "m.tenn")
	plan, err := v.planNodeRename(sceneNode(t, v, "S.Title"), "Card")
	if err != nil {
		t.Fatal(err)
	}
	can := v.session.UndoManager().CanUndo()
	if err := v.applyRename(plan, true); err == nil || !strings.Contains(err.Error(), "take over") || v.text != text || v.session.UndoManager().CanUndo() != can {
		t.Fatalf("a capture must be refused whole: %v\n%s", err, v.text)
	}
}

// A new scene error refuses the rename and leaves every file alone.
func TestRenameRefusedWhenItAddsErrors(t *testing.T) {
	text := "element S {\n  item A { item inner { } }\n  item B { }\n  link A B\n}\n"
	v := renameDoc(t, map[string]string{"m.tenn": text}, "m.tenn")
	plan, err := v.planNodeRename(sceneNode(t, v, "S.A"), "C")
	if err != nil {
		t.Fatal(err)
	}
	plan.edits = nil // a plan whose link end was left behind
	if err := v.applyRename(plan, true); err != errRenameScene || v.text != text {
		t.Fatalf("error %v\n%s", err, v.text)
	}
}

func TestRenameRefusals(t *testing.T) {
	v := renameDoc(t, map[string]string{"m.tenn": renameScene}, "m.tenn")
	for _, c := range []struct{ path, name string }{{"S.A", "B"}, {"S.A", "  "}, {"S.Card", "c1"}, {"S.A", "b"}} {
		if _, err := v.planNodeRename(sceneNode(t, v, c.path), c.name); c.name == "b" && err != nil || c.name != "b" && err == nil {
			t.Errorf("%s to %q: %v", c.path, c.name, err)
		}
	}
	// Only the case changes: the key is the same, so the sibling named B is no obstacle to B.
	plan, err := v.planNodeRename(sceneNode(t, v, "S.B"), "b")
	if err != nil || len(plan.refs) != 2 {
		t.Fatalf("%v %+v", err, plan)
	}
	two := "element S {\n  item X { item i { } }\n  item X { }\n  link X X { target-index 1 }\n}\n"
	v = renameDoc(t, map[string]string{"m.tenn": two}, "m.tenn")
	x := sceneNode(t, v, "S").Nodes()[0]
	if _, err := v.planNodeRename(x, "Z"); err == nil || !strings.Contains(err.Error(), "several") {
		t.Errorf("a link that addresses the name by index blocks the rename: %v", err)
	}
}

// A diagram document: the link holds the item, so the printed link has the new name.
func TestRenameClassicItemKeepsLinks(t *testing.T) {
	text := "element D {\n  item A { pos 0 0; width ${B.width} }\n  item B { pos 100 0 }\n  link A B\n}\n" +
		"element E {\n  item X { inherit \"../B\" }\n}\n"
	v := renameDoc(t, map[string]string{"c.tenn": text}, "c.tenn")
	d := v.session.Root().Elements[0]
	b := d.Items[1]
	refs := ClassicItemRefs(d, b)
	if len(refs) != 1 || !strings.Contains(refs[0].Text, "B.width") || refs[0].Item != d.Items[0] {
		t.Fatalf("refs %+v", refs)
	}
	e := v.session.Root().Elements[1]
	if refs := ClassicItemRefs(&model.Element{Elements: []*model.Element{e}}, b); len(refs) != 1 || refs[0].Text != `inherit ../B` {
		t.Errorf("inherit %+v", refs)
	}
	v.session.CommitEdit(b, editor.EditName, "Beta")
	if !strings.Contains(v.text, `link "A" "Beta"`) {
		t.Errorf("the link keeps its end:\n%s", v.text)
	}
}

func TestMentions(t *testing.T) {
	for _, c := range []struct {
		script, id string
		want       bool
	}{
		{"B.width + 1", "B", true},
		{"AB.width", "B", false},
		{"x.B", "B", false},
		{"B", "B", true},
		{"(B)", "B", true},
		{"B2", "B", false},
		{"", "B", false},
	} {
		if got := mentions(c.script, c.id); got != c.want {
			t.Errorf("%q in %q: %v", c.id, c.script, got)
		}
	}
}

func TestMovedPaths(t *testing.T) {
	for _, c := range []struct {
		p, want string
		ok      bool
	}{
		{"a.b", "a.x", true},
		{"a.b.c", "a.x.c", true},
		{"a.b/part", "a.x/part", true},
		{"a.bc", "a.bc", false},
		{"a", "a", false},
	} {
		if got, ok := movedPath(c.p, "a.b", "a.x"); got != c.want || ok != c.ok {
			t.Errorf("%s: %s %v", c.p, got, ok)
		}
	}
	if renamedNodePath("a.b", "x") != "a.x" || renamedNodePath("b", "x") != "x" {
		t.Error("renamedNodePath")
	}
	s := Selection{Element: []string{"E", "F"}, Nodes: []string{"a.b", "a.b.c", "z"}}
	got := s.afterNodeRename([]string{"E", "F"}, "a.b", "a.x")
	if !slices.Equal(got.Nodes, []string{"a.x", "a.x.c", "z"}) || s.Nodes[0] != "a.b" {
		t.Errorf("nodes %v, the old selection must stay as it was", got.Nodes)
	}
	got = Selection{Element: []string{"E", "F", "G"}}.afterElementRename([]string{"E", "F"}, []string{"E", "H"})
	if !slices.Equal(got.Element, []string{"E", "H", "G"}) {
		t.Errorf("element %v", got.Element)
	}
	got = Selection{Element: []string{"E"}, Items: []string{"A", "B"}}.afterItemRename([]string{"E"}, "A", "Z")
	if !slices.Equal(got.Items, []string{"Z", "B"}) {
		t.Errorf("items %v", got.Items)
	}
}
