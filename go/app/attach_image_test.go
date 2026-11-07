//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestImageRef: a picture inside the workspace is linked by a path relative to the file, one from
// outside is copied into assets - a path that leaves the workspace root is a validation error - and a
// second copy of the same name never replaces the first.
func TestImageRef(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "examples")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "assets", "logo.png")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(outside, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	if ref, err := imageRef(root, dir, inside); err != nil || ref != "../assets/logo.png" {
		t.Errorf("a picture in the workspace: %q, %v", ref, err)
	}
	ref, err := imageRef(root, dir, outside)
	if err != nil || ref != "assets/shot.png" {
		t.Fatalf("a picture from outside: %q, %v", ref, err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(ref))); err != nil {
		t.Errorf("the copy is not there: %v", err)
	}
	if ref, err := imageRef(root, dir, outside); err != nil || ref != "assets/shot-2.png" {
		t.Errorf("a second copy of one name: %q, %v", ref, err)
	}
}
