//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTenn(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A file shows the row of its first mount in the tree and hides the later ones: the library a screen mounts
// again, and the Icon the file mounts after Actions mounted it. Dirty marks the rows that contain the edit.
func TestHiddenMounts(t *testing.T) {
	dir := t.TempDir()
	writeTenn(t, dir, "icon.tenn", "element \"Glyphs\" {\n  item A { width 1 }\n}\n")
	writeTenn(t, dir, "actions.tenn", "element Icon --from-file \"icon.tenn\"\nelement \"Buttons\" {\n  item B { width 1 }\n}\n")
	writeTenn(t, dir, "kit.tenn", "element Actions --from-file \"actions.tenn\"\nelement Icon --from-file \"icon.tenn\"\n")
	writeTenn(t, dir, "screen.tenn", "element ui_kit --from-file \"kit.tenn\"\nelement \"Page\" {\n  item P { width 1 }\n}\n")
	main := "element ui_kit --from-file \"kit.tenn\"\nelement Screen --from-file \"screen.tenn\"\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), main)
	root := v.session.Root()
	kit, screen := root.Elements[0], root.Elements[1]
	hidden := v.hiddenMounts()
	if len(hidden) != 2 || !hidden[screen.Elements[0]] || !hidden[kit.Elements[1]] {
		t.Fatalf("hidden %d: the ui_kit inside Screen and the Icon kit mounts after Actions", len(hidden))
	}
	if hidden[kit] || hidden[screen] || hidden[kit.Elements[0]] || hidden[kit.Elements[0].Elements[0]] || hidden[screen.Elements[1]] {
		t.Fatal("top-level mounts, categories, the first Icon and pages stay rows")
	}
	icon := kit.Elements[1].Elements[0]
	v.session.RenameElement(icon, "Glyphs2")
	if !v.rowDirty(kit.Elements[1]) || !v.rowDirty(kit) || !v.rowDirty(screen) {
		t.Errorf("dirty: Icon %v, ui_kit %v, Screen (holds the same Icon) %v", v.rowDirty(kit.Elements[1]), v.rowDirty(kit), v.rowDirty(screen))
	}
}

// A mount inside an inline element of the document is a row like any other: Tasks holds the files it mounts,
// and the mount of the same file in a second branch is the duplicate without a row.
func TestMountsInInlineElement(t *testing.T) {
	dir := t.TempDir()
	writeTenn(t, dir, "x.tenn", "element \"X content\" {\n  item A { width 1 }\n}\n")
	main := "element Tasks {\n" +
		"  element X --from-file \"x.tenn\"\n" +
		"}\n" +
		"element Other {\n" +
		"  element X again --from-file \"x.tenn\"\n" +
		"}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), main)
	root := v.session.Root()
	tasks, other := root.Elements[0], root.Elements[1]
	hidden := v.hiddenMounts()
	if hidden[tasks.Elements[0]] {
		t.Fatal("the first mount of x.tenn inside Tasks has no row")
	}
	if !hidden[other.Elements[0]] {
		t.Fatal("the second mount of x.tenn has a row of its own")
	}
	if len(hidden) != 1 {
		t.Fatalf("hidden %d mounts, want only the second x.tenn", len(hidden))
	}
}

// The first mount keeps its row wherever it is: the one a mounted file writes comes before the one the
// document writes below it, and the document's is the duplicate.
func TestFirstMountWins(t *testing.T) {
	dir := t.TempDir()
	writeTenn(t, dir, "x.tenn", "element \"X content\" {\n  item A { width 1 }\n}\n")
	writeTenn(t, dir, "screen.tenn", "element X --from-file \"x.tenn\"\n")
	main := "element Screen --from-file \"screen.tenn\"\nelement Late {\n  element X --from-file \"x.tenn\"\n}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), main)
	root := v.session.Root()
	screen, late := root.Elements[0], root.Elements[1]
	hidden := v.hiddenMounts()
	if hidden[screen.Elements[0]] {
		t.Fatal("the mount Screen writes has no row")
	}
	if !hidden[late.Elements[0]] {
		t.Fatal("the mount the document writes below is a duplicate with a row")
	}
}
