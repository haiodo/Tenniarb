//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/penimport"
)

// copyPen copies the fixtures into dir, as penImport writes next to the file it converts.
func copyPen(t *testing.T, dir string, names ...string) string {
	t.Helper()
	var first string
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join("..", "penimport", "testdata", n))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = p
		}
	}
	return first
}

// TestPenImport converts the fixtures: the .tenn of the file asked for is written next to it, and the
// notes are what the conversion left out.
func TestPenImport(t *testing.T) {
	dir := t.TempDir()
	pen := copyPen(t, dir, "imports.pen", "lib.pen")
	main, notes, n, err := penImport([]string{pen})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "imports.tenn"); main != want {
		t.Fatalf("main %q, want %q", main, want)
	}
	if b, err := os.ReadFile(main); err != nil || !strings.Contains(string(b), "element") {
		t.Fatalf("%s: %v", main, err)
	}
	// a warning and two unsupported features
	if n != 3 || !strings.Contains(notes, "Gone") || !strings.Contains(notes, "unresolved ref (1)") {
		t.Fatalf("notes %d: %q", n, notes)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "lib.tenn")); err != nil || len(b) == 0 {
		t.Fatalf("the imported library: %v", err)
	}
}

// TestPenImportClean is the import of a file the conversion has nothing to say about: no notes.
func TestPenImportClean(t *testing.T) {
	dir := t.TempDir()
	main, notes, n, err := penImport([]string{copyPen(t, dir, "components.pen")})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || notes != "" {
		t.Fatalf("notes %d: %q", n, notes)
	}
	if _, err := os.Stat(main); err != nil {
		t.Fatal(err)
	}
}

// TestPenNotes lists a warning per line, then the unsupported features by name, the rest counted.
func TestPenNotes(t *testing.T) {
	f := &penimport.File{
		Warnings:    []string{"b", "a"},
		Unsupported: map[string]int{"z": 2, "y": 1},
	}
	text, n := penNotes(f, 8)
	if want := "b\na\ny (1)\nz (2)"; n != 4 || text != want {
		t.Fatalf("notes %d: %q, want %d: %q", n, text, 4, want)
	}
	text, n = penNotes(f, 2)
	if want := "b\na\nand 2 more"; n != 4 || text != want {
		t.Fatalf("notes %d: %q, want %d: %q", n, text, 4, want)
	}
}
