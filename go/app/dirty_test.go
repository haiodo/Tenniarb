package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrackerUndoToSavedIsClean(t *testing.T) {
	tr := NewTracker("a")
	if tr.Dirty() {
		t.Fatal("new tracker is dirty")
	}
	tr.Changed("b")
	if !tr.Dirty() {
		t.Fatal("change not seen")
	}
	tr.Changed("a")
	if tr.Dirty() {
		t.Fatal("undo to the saved text left it dirty")
	}
	tr.Changed("c")
	tr.Saved("c")
	if tr.Dirty() {
		t.Fatal("dirty after save")
	}
	tr.Changed("a")
	if !tr.Dirty() {
		t.Fatal("undo past the save is a change")
	}
}

func TestDecisions(t *testing.T) {
	if DecideReload(false) != ReloadApply || DecideReload(true) != ReloadAsk {
		t.Fatal("reload decision")
	}
}

func TestTitle(t *testing.T) {
	cases := []struct {
		path  string
		dirty bool
		want  string
	}{
		{"/x/Example.tenn", false, "Example.tenn - Tenniarb"},
		{"/x/Example.tenn", true, "Example.tenn - Edited - Tenniarb"},
		{"", false, "Untitled - Tenniarb"},
	}
	for _, c := range cases {
		if got := Title(c.path, c.dirty); got != c.want {
			t.Errorf("Title(%q,%v) = %q, want %q", c.path, c.dirty, got, c.want)
		}
	}
	if WithExt("/x/a") != "/x/a.tenn" || WithExt("/x/a.txt") != "/x/a.txt" {
		t.Fatal("WithExt")
	}
}

func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if strings.Contains(e.Name(), ".tmp") {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tenn")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Fatalf("content %q", got)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
	if l := leftovers(t, dir); len(l) != 0 {
		t.Fatalf("temp files left: %v", l)
	}

	link := filepath.Join(dir, "link.tenn")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("via link")); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was replaced")
	}
	if got, _ := os.ReadFile(path); string(got) != "via link" {
		t.Fatalf("target %q", got)
	}
}

func TestWriteFileAtomicFailureKeepsOld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tenn")
	os.WriteFile(path, []byte("old"), 0o644)
	if err := WriteFileAtomic(filepath.Join(dir, "missing", "a.tenn"), []byte("x")); err == nil {
		t.Fatal("no error for a missing directory")
	}
	// a directory in place of the file: the rename fails after the temp file was written
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "keep"), nil, 0o644)
	if err := WriteFileAtomic(sub, []byte("x")); err == nil {
		t.Fatal("renamed a file over a directory")
	}
	if l := leftovers(t, dir); len(l) != 0 {
		t.Fatalf("temp files left: %v", l)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("old file damaged: %q", got)
	}
}
