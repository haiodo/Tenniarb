package bundle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/tenniarb/go/bundle"
)

// TestText: the editor holds a document in memory, so Text expands the mounts of the text it is given,
// not the text of the file on disk.
func TestText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.tenn"), []byte("element Part { item a { title b } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := bundle.Text(dir, filepath.Join(dir, "main.tenn"), "element kit --from-file \"lib.tenn\"\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "element kit {\n  element Part { item a { title b } }\n}\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
