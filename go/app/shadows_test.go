//go:build darwin || linux || windows

package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

const shadowDoc = `element "D" {
  styles {
    item {
      shadow 0 -4 12 #00000010
      shadow 0 -10 10 #00000010
    }
  }
  item "A" { pos 0 0 }
}`

// An item that writes no shadow of its own is drawn with the item defaults of the element: the list
// shows those layers, and changing one writes them all as the item's own, an override.
func TestShownShadowsFallBackToItemDefaults(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "s.tenn"), shadowDoc)
	el := v.session.Root().Elements[0]
	v.session.SetElement(el)
	v.session.Select([]*model.DiagramItem{el.Items[0]})
	if !v.shadowsInherited() {
		t.Fatal("the item writes no shadow: the list shows the defaults")
	}
	layers := v.shownShadows()
	if len(layers) != 2 {
		t.Fatalf("%d layers, want 2", len(layers))
	}
	if !v.setShadows(layers) {
		t.Fatal("refused")
	}
	if v.shadowsInherited() {
		t.Fatal("the item writes its own shadows now")
	}
	if n := strings.Count(v.text, "shadow 0 -"); n != 4 {
		t.Fatalf("%d shadow commands, want 2 in the item and 2 in the defaults:\n%s", n, v.text)
	}
}

const twoShadowItem = `element "D" {
  item "A" { pos 0 0; shadow 0 -2 4 #00000010; shadow 0 -8 16 #00000008 }
}`

// The classic list edits one layer of a two-layer item: the other layer keeps its bytes, and the whole
// list is one undo step.
func TestShadowListEditsOneLayerOfAClassicItem(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "c.tenn"), twoShadowItem)
	el := v.session.Root().Elements[0]
	v.session.SetElement(el)
	v.session.Select([]*model.DiagramItem{el.Items[0]})
	if n := len(v.shownShadows()); n != 2 {
		t.Fatalf("%d layers, want 2", n)
	}
	before := v.text
	if !shadowLayerSrc(v.shownShadows, v.setShadows, 1).arg("shadow", 2, "20") {
		t.Fatal("refused")
	}
	if got := v.shownShadows()[1].Arg(2).Text; got != "20" {
		t.Errorf("the second layer has blur %q, want 20", got)
	}
	if got := v.shownShadows()[0].Arg(2).Text; got != "4" {
		t.Errorf("the first layer changed to blur %q", got)
	}
	if !strings.Contains(v.text, "shadow 0 -8 20 #00000008") || !strings.Contains(v.text, "shadow 0 -2 4 #00000010") {
		t.Fatalf("the file does not hold the layers the list shows:\n%s", v.text)
	}
	v.session.Undo()
	if v.text != before {
		t.Fatalf("one undo does not put the layers back:\n%s\nwant\n%s", v.text, before)
	}
}

const sceneShadowsDoc = `item Screen {
  pos 0 0
  item Card {
    pos 0 0
    shadow 0 -2 4 #00000010
    shadow 0 -8 16 #00000008
  }
}
`

// drawSceneFor draws the document from its current tree and selects one of its nodes, as a redraw and a
// click do.
func drawSceneFor(t *testing.T, v *Viewer, path string) {
	t.Helper()
	v.sv = viewOf(t, v.stree, v.file())
	v.selNodes = []string{path}
}

// The scene list reads the layers the node writes, edits one of them, adds and drops a layer, and each
// write is one undo step that leaves the file with exactly the layers shown.
func TestSceneShadowList(t *testing.T) {
	const path = "Screen.Card"
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "s.tenn"), sceneShadowsDoc)
	drawSceneFor(t, v, path)
	layers := v.sceneShadows()
	if len(layers) != 2 || layers[0].Arg(2).Text != "4" || layers[1].Arg(2).Text != "16" {
		t.Fatalf("layers %v", layers)
	}

	if !shadowLayerSrc(v.sceneShadows, v.setSceneShadows, 1).arg("shadow", 2, "20") {
		t.Fatal("changing a layer was refused")
	}
	drawSceneFor(t, v, path)
	if got := v.sceneShadows(); len(got) != 2 || got[1].Arg(2).Text != "20" || got[0].Arg(2).Text != "4" {
		t.Fatalf("after the edit %v", got)
	}
	if want := strings.Replace(sceneShadowsDoc, "16 #00000008", "20 #00000008", 1); v.text != want {
		t.Fatalf("the file:\n%s\nwant\n%s", v.text, want)
	}
	v.session.Undo()
	drawSceneFor(t, v, path)
	if v.text != sceneShadowsDoc {
		t.Fatalf("one undo of a layer change:\n%s", v.text)
	}

	added := append(v.sceneShadows(), editor.Prop{Set: true, Args: argList([]string{"0", "-1", "3", "#00000020"})})
	if !v.setSceneShadows(added) {
		t.Fatal("adding a layer was refused")
	}
	drawSceneFor(t, v, path)
	if got := v.sceneShadows(); len(got) != 3 || got[2].Arg(2).Text != "3" {
		t.Fatalf("after adding %v", got)
	}
	if want := strings.Replace(sceneShadowsDoc, "    shadow 0 -8 16 #00000008\n", "    shadow 0 -8 16 #00000008\n    shadow 0 -1 3 #00000020\n", 1); v.text != want {
		t.Fatalf("the file after adding:\n%s\nwant\n%s", v.text, want)
	}
	v.session.Undo()
	drawSceneFor(t, v, path)
	if v.text != sceneShadowsDoc {
		t.Fatalf("one undo of an added layer:\n%s", v.text)
	}

	if !v.setSceneShadows(slices.Delete(v.sceneShadows(), 0, 1)) {
		t.Fatal("dropping a layer was refused")
	}
	drawSceneFor(t, v, path)
	if got := v.sceneShadows(); len(got) != 1 || got[0].Arg(1).Text != "-8" {
		t.Fatalf("after dropping the first layer %v", got)
	}
	if want := strings.Replace(sceneShadowsDoc, "    shadow 0 -2 4 #00000010\n", "", 1); v.text != want {
		t.Fatalf("the file after dropping:\n%s\nwant\n%s", v.text, want)
	}
	v.session.Undo()
	drawSceneFor(t, v, path)
	if v.text != sceneShadowsDoc {
		t.Fatalf("one undo of a dropped layer:\n%s", v.text)
	}

	if !v.setSceneShadows(nil) {
		t.Fatal("dropping every layer was refused")
	}
	drawSceneFor(t, v, path)
	if len(v.sceneShadows()) != 0 || strings.Contains(v.text, "shadow") {
		t.Fatalf("layers left after dropping the last one:\n%s", v.text)
	}
	v.session.Undo()
	drawSceneFor(t, v, path)
	if v.text != sceneShadowsDoc {
		t.Fatalf("one undo of the last layer:\n%s", v.text)
	}
}
