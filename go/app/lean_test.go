//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

const leanDoc = "element \"S\" {\n  item A { pos 0 0; width 40; item B { title Hi } }\n}\nelement \"T\" {\n  item C { pos 1 1; item E { width 3 } }\n}\n"

// A scene document keeps the splice of a text edit, not the swapped trees: undo and redo give the same
// text and the same model as the edit did.
func TestLeanTextEditUndoRedoRoundTrip(t *testing.T) {
	v := newDoc(t, &Shell{}, "", leanDoc)
	if v.kind != model.DocScene {
		t.Fatalf("kind %s", v.kind)
	}
	// each step replaces from by to in the text as it is then; "" from appends
	steps := []struct{ from, to string }{
		{"title Hi", `title "Sign in"`},
		{"pos 1 1", "pos 5 6"},
		{"", "element \"New\" { item D { item F { } } }\n"}, // the shape changes
		{"item C ", "item Cc "},
	}
	texts, prints := []string{v.text}, []string{model.ToTennStr(&v.session.Root().Element, false, true)}
	for i, s := range steps {
		at := len(v.text)
		if s.from != "" {
			at = strings.Index(v.text, s.from)
		}
		if err := v.applyText(at, at+len(s.from), s.to); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		texts = append(texts, v.text)
		prints = append(prints, model.ToTennStr(&v.session.Root().Element, false, true))
	}
	check := func(what string, i int) {
		t.Helper()
		if v.text != texts[i] || model.ToTennStr(&v.session.Root().Element, false, true) != prints[i] {
			t.Fatalf("%s to step %d:\n%s", what, i, v.text)
		}
	}
	for i := len(steps) - 1; i >= 0; i-- {
		v.session.Undo()
		check("undo", i)
	}
	for i := range steps {
		v.session.Redo()
		check("redo", i+1)
	}
	for i := len(steps) - 1; i >= 0; i-- {
		v.session.Undo()
		check("undo again", i)
	}
}

// A scene edit loads the scene once: the tree that checked the edit is the one the document keeps.
func TestSceneEditLoadsOnce(t *testing.T) {
	dir := t.TempDir()
	lib := "item Card { pos 0 0; width 10 }\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	main := "element \"kit\" --from-file \"kit.lib.tenn\"\nelement \"S\" { item A { pos 0 0 } }\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), main)
	loads := 0
	loadAuto = func(root, file string, overlay map[string]string) (*scene.Tree, error) {
		loads++
		return scene.LoadAuto(root, file, overlay)
	}
	defer func() { loadAuto = scene.LoadAuto }()

	edit := func(file, from, to string) {
		t.Helper()
		text := v.fileText(file)
		i := strings.Index(text, from)
		e := TextEdit{Start: i, End: i + len(from), New: to}
		loads = 0
		if v.editAddsErrors(file, text, e) {
			t.Fatalf("%s: refused", file)
		}
		if err := v.applyFileText(file, e.Start, e.End, e.New); err != nil {
			t.Fatal(err)
		}
		if loads != 1 {
			t.Errorf("%s: %d scene loads for one edit", file, loads)
		}
		if got, _ := v.stree.Source(file); got != v.fileText(file) {
			t.Errorf("%s: the tree shows another text", file)
		}
	}
	edit("main.tenn", "pos 0 0", "pos 3 4")
	edit("kit.lib.tenn", "width 10", "width 99")
	if v.text != strings.Replace(main, "pos 0 0", "pos 3 4", 1) || v.libText("kit.lib.tenn") != strings.Replace(lib, "width 10", "width 99", 1) {
		t.Errorf("texts %q %q", v.text, v.libText("kit.lib.tenn"))
	}

	// An edit that adds an error is refused and leaves nothing for the next load to take.
	bad := TextEdit{Start: strings.Index(v.text, "item A"), End: strings.Index(v.text, "item A") + 6, New: "use Nowhere as A; item "}
	if !v.editAddsErrors("main.tenn", v.text, bad) {
		t.Fatal("an unknown component must add an error")
	}
	if v.pending != nil {
		t.Error("a refused edit keeps its tree")
	}

	v.session.Undo()
	v.session.Redo()
	if v.libText("kit.lib.tenn") != strings.Replace(lib, "width 10", "width 99", 1) {
		t.Errorf("redo: %q", v.libText("kit.lib.tenn"))
	}
}
