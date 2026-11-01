package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/tenniarb/go/skill"
)

// TestInstall writes the skill into a directory: every file lands, and the second run replaces them.
func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tenniarb-design")
	if err := skill.Write(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range skill.Names() {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want, err := skill.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Errorf("%s differs from the embedded copy", name)
		}
	}
	// a stale file does not survive the second run, the embedded texts are what is installed
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := skill.Write(dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); string(b) == "stale" {
		t.Error("the install kept the stale file")
	}
}

// TestRead: an empty name is SKILL.md, an unknown one lists the files.
func TestRead(t *testing.T) {
	main, err := skill.Read("")
	if err != nil || main == "" {
		t.Fatalf("SKILL.md: %v", err)
	}
	if _, err := skill.Read("nope.md"); err == nil {
		t.Error("no error for an unknown file")
	}
}

// TestInstallReplacesLink: an older `make install` left a link there; the install puts the real files.
func TestInstallReplacesLink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "tenniarb-design")
	if err := os.Symlink(filepath.Join(dir, "gone"), link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := skill.Write(link); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Error("the link was not replaced by a directory")
	}
	if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md: %v", err)
	}
}
