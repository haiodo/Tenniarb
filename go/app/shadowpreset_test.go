//go:build darwin || linux || windows

package app

import (
	"slices"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// itemNamed parses text and finds the node with that name anywhere in it.
func itemNamed(t *testing.T, text, name string) *scene.Node {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", text)
	if tree == nil {
		t.Fatal(err)
	}
	var found *scene.Node
	var walk func(*scene.Node)
	walk = func(n *scene.Node) {
		for _, c := range append(slices.Clone(n.Content), n.Elems...) {
			if c.Name == name {
				found = c
			}
			walk(c)
		}
	}
	walk(tree.File("a.tenn"))
	if found == nil {
		t.Fatalf("no %s in %s", name, text)
	}
	return found
}

// TestSetShadowsEdit: the inspector preset replaces what shadows the node has with the layers of the
// preset, adds them to a node without any, and takes them all away when asked; the edits address one
// text, so they are applied from the last to the first.
func TestSetShadowsEdit(t *testing.T) {
	const text = `element "E" {
  item Card {
    width 10
    shadow 0 -1 2 #00000010
    color #fff
  }
  item Flat { width 4 }
}`
	soft := []string{
		CommandText("shadow", "0", "-4", "12", "#00000014"),
		CommandText("shadow", "0", "-14", "32", "#00000010"),
	}
	run := func(name string, cmds ...string) string {
		got := text
		for _, e := range SetShadowsEdit(text, itemNamed(t, text, name), cmds...) {
			got = apply(got, e)
		}
		return got
	}

	got := run("Card", soft...)
	want := `element "E" {
  item Card {
    width 10
    shadow 0 -4 12 #00000014
    shadow 0 -14 32 #00000010
    color #fff
  }
  item Flat { width 4 }
}`
	if got != want {
		t.Errorf("one shadow replaced by two:\n%s\nwant\n%s", got, want)
	}
	if !sameShadows(shadowsOf(itemNamed(t, got, "Card")), shadowPresets["soft"]) {
		t.Error("the written layers do not read back as the preset")
	}

	// A block of one line is written out over several: the printer of the app does the same.
	flat := run("Flat", soft...)
	if n := itemNamed(t, flat, "Flat"); !sameShadows(shadowsOf(n), shadowPresets["soft"]) {
		t.Errorf("a node without shadows:\n%s", flat)
	}

	bare := run("Card")
	if want := `element "E" {
  item Card {
    width 10
    color #fff
  }
  item Flat { width 4 }
}`; bare != want {
		t.Errorf("removing every shadow:\n%s", bare)
	}
	if len(shadowsOf(itemNamed(t, bare, "Card"))) != 0 {
		t.Error("a shadow is left after removing them")
	}
}
