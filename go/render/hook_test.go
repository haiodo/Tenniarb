package render_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// ClassicHook reads a value from the element model of a classic diagram, and stays out of the way of
// a path that names no element of the file.
func TestClassicHookResolvesValues(t *testing.T) {
	root := t.TempDir()
	const src = `element "E" {
  item A { width ${40 + 2} }
}
`
	if err := os.WriteFile(filepath.Join(root, "e.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := scene.Load(root, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	hook := render.ClassicHook(tree, root, "e.tenn", "E")
	if hook == nil {
		t.Fatal("no hook for the element E")
	}
	got, ok := hook(scene.ClassicRef{Kind: scene.RItem, Name: "A"}, "width")
	if !ok || len(got) != 1 || got[0] != "42" {
		t.Errorf("A|width = %v %v, want 42", got, ok)
	}
	for _, path := range []string{"", "Gone"} {
		if hook := render.ClassicHook(tree, root, "e.tenn", path); hook != nil {
			t.Errorf("path %q has a hook", path)
		}
	}
}
