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

// A file mounted inside a mounted file shows no row when another row already shows that file: the library
// a screen mounts again, and the Icon a category file mounts. Dirty marks the rows that contain the edit.
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
	if len(hidden) != 2 || !hidden[screen.Elements[0]] || !hidden[kit.Elements[0].Elements[0]] {
		t.Fatalf("hidden %d: the ui_kit inside Screen and the Icon inside Actions", len(hidden))
	}
	if hidden[kit] || hidden[screen] || hidden[kit.Elements[0]] || hidden[kit.Elements[1]] || hidden[screen.Elements[1]] {
		t.Fatal("top-level mounts, categories and pages stay rows")
	}
	icon := kit.Elements[1].Elements[0]
	v.session.RenameElement(icon, "Glyphs2")
	if !v.rowDirty(kit.Elements[1]) || !v.rowDirty(kit) || !v.rowDirty(screen) {
		t.Errorf("dirty: Icon %v, ui_kit %v, Screen (holds the same Icon) %v", v.rowDirty(kit.Elements[1]), v.rowDirty(kit), v.rowDirty(screen))
	}
}
