//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// TestCodeOfAMountShowsItsOwnLine: the Code section of an element with --from-file shows the line of that
// mount, which is written in the declaring file; the content of the mounted file has other offsets and
// anything else cuts a slice of the wrong text.
func TestCodeOfAMountShowsItsOwnLine(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"lib.tenn":  "element \"Buttons\" {\n  item B { width 1 }\n}\n",
		"main.tenn": "element Actions --from-file \"lib.tenn\"\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := scene.Load(dir, "main.tenn")
	if err != nil {
		t.Fatal(err)
	}
	mount := tree.File("main.tenn").Nodes()[0]
	if mount.Mount == nil {
		t.Fatal("the element is not a mount")
	}
	d := &docCore{state: NewState(&Document{Path: filepath.Join(dir, "main.tenn")}), root: dir, stree: tree, text: files["main.tenn"]}
	file := d.ownFileOf(mount)
	if file != "main.tenn" {
		t.Fatalf("the file of the mount is %q, want main.tenn", file)
	}
	if got, want := OwnText(d.fileText(file), mount), "element Actions --from-file \"lib.tenn\""; got != want {
		t.Fatalf("the code of the mount:\n%s\nwant:\n%s", got, want)
	}
	// A node inside the mounted file is written there, and its code comes from that text.
	inner := mount.Nodes()[0]
	if f := d.ownFileOf(inner); f != "lib.tenn" {
		t.Fatalf("the file of the mounted item is %q, want lib.tenn", f)
	}
	if got := OwnText(d.fileText(d.ownFileOf(inner)), inner); got != "element \"Buttons\" {\n  item B { width 1 }\n}" {
		t.Fatalf("the code inside the mounted file:\n%s", got)
	}
}
