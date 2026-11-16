//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	iconLib  = "item Crosshair {\n  width 16; height 16\n  item Dot { width 4; height 4; pos 6 6 }\n}\n"
	kitFile  = "entry \"design.tenn\"\nelement Icon --from-file \"lib/icon.tenn\"\n"
	partFile = "entry \"../design.tenn\"\nelement \"Screen\" {\n  auto-layout abs; width 100; height 100\n" +
		"  use Libraries.ui_kit.Icon.Crosshair as Crosshair { pos 1 -1 }\n}\n"
	designFile = "element Libraries {\n  element ui_kit --from-file \"ui-kit.lib.tenn\"\n}\n" +
		"element screen --from-file \"apps/part.tenn\"\n"
)

// Go to Component of a use in a part file goes to the library the entry mounts, not to the file that uses
// it: the component comes into view in the window of the file that mounts the library, and is selected.
func TestGoToComponentMountedByEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil { // the workspace root
		t.Fatal(err)
	}
	for _, sub := range []string{"apps", "lib"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTenn(t, dir, "lib/icon.tenn", iconLib)
	writeTenn(t, dir, "apps/part.tenn", partFile)
	sh := &Shell{}
	kit := newDoc(t, sh, filepath.Join(dir, "ui-kit.lib.tenn"), kitFile)
	// The canvas the window draws: the element that mounts the library the part file uses.
	kit.sv = viewOf(t, kit.stree, kit.elementNode(kit.session.Root().Elements[0]))
	newDoc(t, sh, filepath.Join(dir, "design.tenn"), designFile)
	part := newDoc(t, sh, filepath.Join(dir, "apps/part.tenn"), partFile)
	if part.entry != "design.tenn" || part.sceneErr != "" {
		t.Fatalf("entry %q, errors %q", part.entry, part.sceneErr)
	}
	use := part.stree.File("apps/part.tenn").Elems[0].Content[0]
	part.goToComponentNode(use)
	if got := kit.state.Current; got == nil || got.FromFile != "lib/icon.tenn" {
		t.Errorf("the window shows %v, want the element that mounts lib/icon.tenn", got)
	}
	if !slices.Equal(kit.selNodes, []string{"Crosshair"}) {
		t.Errorf("selected %q, want the component", kit.selNodes)
	}
	if sh.active != kit {
		t.Error("the window that shows the component is not the active one")
	}
}
