//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// The element tree shows the mounted file, and edits never write its content into the document.
func TestFromFileDocument(t *testing.T) {
	dir := t.TempDir()
	lib := "element \"Parts\" {\n  item Button { width 40; height 20 }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "element \"kit\" --from-file \"kit.lib.tenn\"\nelement \"Map\" {\n    item \"Idea\" {\n        pos 0 0\n    }\n}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text)
	kit := v.session.Root().Elements[0]
	if len(kit.Elements) != 1 || kit.Elements[0].Name != "Parts" {
		t.Fatalf("the mounted element has no content: %+v", kit.Elements)
	}
	v.session.RenameElement(v.session.Root().Elements[1], "Map 2")
	if !strings.Contains(v.text, `element "Map 2"`) || !strings.Contains(v.text, `--from-file`) ||
		strings.Contains(v.text, "Parts") || strings.Contains(v.text, "Button") {
		t.Fatalf("a classic edit inlined or lost the library line:\n%s", v.text)
	}
	if v.session.Root().Elements[0] != kit || len(kit.Elements) != 1 {
		t.Error("an edit elsewhere must keep the mounted elements")
	}
	if got := model.KindOfModel(v.session.Root()); got == model.DocDiagram {
		t.Errorf("kind %s", got)
	}
	v.session.RenameElement(kit.Elements[0], "Other")
	if kit.Elements[0].Name != "Other" || strings.Contains(v.text, "Other") {
		t.Errorf("an edit inside the mounted file goes to the file, not to the document:\n%s", v.text)
	}
}

const (
	wbLib  = "element \"Parts\" {\n  item Button { width 40; height 20 }\n}\n\n// kept\nitem Card { width 10 }\n"
	wbMain = "element \"kit\" --from-file \"kit.lib.tenn\"\nelement \"Map\" {\n    item \"Idea\" {\n        pos 0 0\n    }\n}\n"
)

func writeBack(t *testing.T) (v *Viewer, dir string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(wbLib), 0o644); err != nil {
		t.Fatal(err)
	}
	return newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), wbMain), dir
}

func fileText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A classic edit inside the mount changes the text of its file, marks it unsaved, and the save writes
// that file only; undo and redo bring the same bytes back.
func TestMountClassicEditWritesBack(t *testing.T) {
	v, dir := writeBack(t)
	lib := filepath.Join(dir, "kit.lib.tenn")
	kit := v.session.Root().Elements[0]
	parts := kit.Elements[0]
	v.session.RenameElement(parts, "Other")
	want := strings.Replace(wbLib, `"Parts"`, `"Other"`, 1)
	if v.libText("kit.lib.tenn") != want {
		t.Fatalf("text of the file:\n%s", v.libText("kit.lib.tenn"))
	}
	if v.text != wbMain || v.isDirty() {
		t.Fatalf("the document must not change:\n%s", v.text)
	}
	if !v.mountDirty(kit) || fileText(t, lib) != wbLib || kit.Elements[0] != parts {
		t.Fatalf("unsaved edit is held until save: dirty %v, same element %v", v.mountDirty(kit), kit.Elements[0] == parts)
	}
	if err := v.saveLibs(); err != nil {
		t.Fatal(err)
	}
	if fileText(t, lib) != want || v.mountDirty(kit) || fileText(t, filepath.Join(dir, "main.tenn")) != wbMain {
		t.Fatalf("after save:\n%s", fileText(t, lib))
	}
	v.session.Undo()
	if v.libText("kit.lib.tenn") != wbLib || !v.mountDirty(kit) || kit.Elements[0].Name != "Parts" {
		t.Fatalf("undo:\n%s", v.libText("kit.lib.tenn"))
	}
	v.session.Redo()
	if v.libText("kit.lib.tenn") != want {
		t.Fatalf("redo:\n%s", v.libText("kit.lib.tenn"))
	}
	v.session.Undo()
	if err := v.saveLibs(); err != nil || fileText(t, lib) != wbLib || v.libsDirty() {
		t.Fatalf("undo and save:\n%s", fileText(t, lib))
	}
}

// An element added into the mount lands in its file, an item added to a mounted element is kept in it.
func TestMountClassicAddAndMove(t *testing.T) {
	v, _ := writeBack(t)
	kit := v.session.Root().Elements[0]
	el := v.session.AddElement(kit)
	if el == nil || !strings.Contains(v.libText("kit.lib.tenn"), "element") || !strings.Contains(v.libText("kit.lib.tenn"), "// kept") {
		t.Fatalf("add into the mount: %v\n%s", el, v.libText("kit.lib.tenn"))
	}
	if v.text != wbMain {
		t.Fatalf("the document must not change:\n%s", v.text)
	}
	if err := v.session.Store().Move(v.session.Root().Elements[1], kit, v.session.UndoManager(), nil, 0); err != model.ErrMounted {
		t.Errorf("an element does not move from the document into a mounted file: %v", err)
	}
}

// A scene edit of a node that is written in the mounted file goes to that file and never to the
// document; undo steps over it.
func TestMountSceneEditGoesToItsFile(t *testing.T) {
	dir := t.TempDir()
	lib := "item Card { pos 0 0; width 10 }\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	main := "element \"kit\" --from-file \"kit.lib.tenn\"\nelement \"S\" { item A { pos 0 0 } }\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), main)
	kit := v.session.Root().Elements[0]
	node := NodeAt(v.elementNode(kit), "Card")
	if node == nil || node.Pos.File != "kit.lib.tenn" || v.fileOf(node) != "kit.lib.tenn" {
		t.Fatalf("node %+v", node)
	}
	e, ok := SetPropEdit(v.fileText(v.fileOf(node)), node, "width", "99")
	if !ok {
		t.Fatal("no edit")
	}
	if err := v.applyFileText(v.fileOf(node), e.Start, e.End, e.New); err != nil {
		t.Fatal(err)
	}
	want := "item Card { pos 0 0; width 99 }\n"
	if v.libText("kit.lib.tenn") != want || v.text != main || !v.mountDirty(kit) {
		t.Fatalf("lib %q document %q", v.libText("kit.lib.tenn"), v.text)
	}
	if err := v.saveLibs(); err != nil || fileText(t, filepath.Join(dir, "kit.lib.tenn")) != want || fileText(t, filepath.Join(dir, "main.tenn")) != main {
		t.Fatalf("saved:\n%s", fileText(t, filepath.Join(dir, "kit.lib.tenn")))
	}
	v.session.Undo()
	if v.libText("kit.lib.tenn") != lib || !v.mountDirty(kit) {
		t.Fatalf("undo: %q", v.libText("kit.lib.tenn"))
	}
	// the root of the mount takes a new node, not the line of the mounting element
	root := v.elementNode(kit)
	ins := InsertEdit(v.fileText(v.fileOf(root)), root, "item New { pos 5 5 }")
	if v.fileOf(root) != "kit.lib.tenn" {
		t.Fatalf("the mounting element edits its file, not %s", v.fileOf(root))
	}
	if err := v.applyFileText(v.fileOf(root), ins.Start, ins.End, ins.New); err != nil || !strings.Contains(v.libText("kit.lib.tenn"), "item New") || v.text != main {
		t.Fatalf("insert: %v\n%s", err, v.libText("kit.lib.tenn"))
	}
	if err := v.applyFileText("other.tenn", 0, 0, "x"); err == nil {
		t.Error("a file the document does not mount is refused")
	}
}

// A file that another window has takes the edit as its own: its text changes, it is the one to save.
func TestMountEditGoesToTheOpenWindowOfTheFile(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "kit.lib.tenn")
	sh := &Shell{}
	peer := newDoc(t, sh, lib, wbLib)
	main := newDoc(t, sh, filepath.Join(dir, "main.tenn"), wbMain)
	kit := main.session.Root().Elements[0]
	main.session.RenameElement(kit.Elements[0], "Other")
	if !strings.Contains(peer.text, `element "Other"`) || !peer.isDirty() || !main.mountDirty(kit) || main.libsDirty() {
		t.Fatalf("the window of the file did not take the edit: dirty %v\n%s", peer.isDirty(), peer.text)
	}
	if kit.Elements[0].Name != "Other" {
		t.Error("the edited element must stay")
	}
}
