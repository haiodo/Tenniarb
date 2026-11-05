package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// mountScreen is a screen of one element with --from-file: the declaration is in the document, the
// content in the mounted file.
func mountScreen(t *testing.T) (v *Viewer, dir string) {
	t.Helper()
	dir = t.TempDir()
	lib := "item Frame { auto-layout abs; width 200; height 100\n\titem Button { pos 8 8; width 40; height 20 }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "element kit --from-file \"kit.lib.tenn\"\n"
	return newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text), dir
}

// TestMountElementFiles: the props of the mounting element are written in the file that declares it, the
// props of what it shows in the mounted file.
func TestMountElementFiles(t *testing.T) {
	v, _ := mountScreen(t)
	if v.stree == nil {
		t.Fatalf("no scene: %s", v.sceneErr)
	}
	mount := v.stree.File(v.rel()).Elems[0]
	if got := v.ownFileOf(mount); got != v.rel() {
		t.Fatalf("the element itself belongs to %q, want %q", got, v.rel())
	}
	if got := v.fileOf(mount); got != "kit.lib.tenn" {
		t.Fatalf("its content belongs to %q", got)
	}
	btn := NodeAt(mount, "Frame.Button")
	if btn == nil {
		t.Fatal("no Button")
	}
	if got := v.ownFileOf(btn); got != "kit.lib.tenn" {
		t.Fatalf("an item of the mounted file belongs to %q", got)
	}

	// a prop of an item goes to the mounted file, never to the document
	e, ok := SetPropEdit(v.fileText(v.ownFileOf(btn)), btn, "width", "60")
	if !ok {
		t.Fatal("no edit")
	}
	if err := v.applyFileText(v.ownFileOf(btn), e.Start, e.End, e.New); err != nil {
		t.Fatalf("%v (edit %+v)", err, e)
	}
	if strings.Contains(v.text, "width 60") {
		t.Fatalf("the document took the edit:\n%s", v.text)
	}
	if !strings.Contains(v.libText("kit.lib.tenn"), "width 60") {
		t.Fatalf("the mounted file missed the edit:\n%s", v.libText("kit.lib.tenn"))
	}
}

// TestMountAddItem: an item added while the mounting element is on the canvas is written in the mounted
// file, at the end of it.
func TestMountAddItem(t *testing.T) {
	v, _ := mountScreen(t)
	if v.stree == nil {
		t.Fatalf("no scene: %s", v.sceneErr)
	}
	mount := v.stree.File(v.rel()).Elems[0]
	file := v.fileOf(mount)
	f := v.stree.File(file)
	if f == nil {
		t.Fatalf("no file %q in the scene", file)
	}
	e := InsertEdit(v.fileText(file), f, "item New { width 10; height 10 }")
	if err := v.applyFileText(file, e.Start, e.End, e.New); err != nil {
		t.Fatalf("%v (edit %+v)", err, e)
	}
	got := v.libText(file)
	if !strings.Contains(got, "item New") {
		t.Fatalf("no item in the mounted file:\n%s", got)
	}
	tree, err := scene.LoadSource(filepath.Dir(v.state.Doc.Path), file, got)
	if err != nil {
		t.Fatalf("%v\n%s", err, got)
	}
	if tree.File(file) == nil {
		t.Fatal("the mounted file does not parse")
	}
}
