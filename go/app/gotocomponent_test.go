//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/haiodo/gowt/gfx"
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

// Go to Component of a use in a part file with no library window opens the file that mounts the library in
// the current window, not in a new one, and selects the component there.
func TestGoToComponentOpensMountInTheSameWindow(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"apps", "lib"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTenn(t, dir, "lib/icon.tenn", iconLib)
	writeTenn(t, dir, "apps/part.tenn", partFile)
	writeTenn(t, dir, "ui-kit.lib.tenn", kitFile)
	writeTenn(t, dir, "design.tenn", designFile)
	sh := &Shell{}
	part := newDoc(t, sh, filepath.Join(dir, "apps/part.tenn"), partFile)
	part.scene = gfx.NewScene()
	use := part.stree.File("apps/part.tenn").Elems[0].Content[0]
	part.goToComponentNode(use)
	if len(sh.viewers) != 1 || sh.active != part {
		t.Fatalf("opened %d windows, active %v; want the one window", len(sh.viewers), sh.active == part)
	}
	if want := filepath.Join(dir, "ui-kit.lib.tenn"); realPath(part.state.Doc.Path) != realPath(want) {
		t.Errorf("the window shows %q, want %q", part.state.Doc.Path, want)
	}
	if got := part.state.Current; got == nil || got.FromFile != "lib/icon.tenn" {
		t.Fatalf("the window shows %v, want the element that mounts lib/icon.tenn", got)
	}
	if !slices.Equal(part.selNodes, []string{"Crosshair"}) {
		t.Errorf("selected %q, want the component", part.selNodes)
	}
}

// A library below the workspace root is mounted by a path from the folder of the file that mounts it, not
// from the root: Go to Component of a document that lives in a subfolder finds its own mount.
func TestGoToComponentOfFileInSubfolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	design := filepath.Join(dir, "design")
	if err := os.Mkdir(design, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTenn(t, dir, "design/icon.tenn", iconLib)
	kits := "element Icon --from-file \"icon.tenn\"\nelement \"Panel\" {\n  use Icon.Crosshair as Crosshair\n}\n"
	writeTenn(t, dir, "design/kit.tenn", kits)
	sh := &Shell{}
	kit := newDoc(t, sh, filepath.Join(design, "kit.tenn"), kits)
	use := kit.stree.File("design/kit.tenn").Elems[1].Content[0]
	kit.goToComponentNode(use)
	if len(sh.viewers) != 1 {
		t.Fatalf("%d windows, want the one that was open", len(sh.viewers))
	}
	if got := kit.state.Current; got == nil || got.FromFile != "icon.tenn" {
		t.Errorf("the window shows %v, want the element that mounts icon.tenn", got)
	}
}
