package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func touchLib(t *testing.T, root, rel string) string {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// An installed library shadows a shipped one of the same name.
func TestFinderInstalledFirst(t *testing.T) {
	installed, shipped := t.TempDir(), t.TempDir()
	want := touchLib(t, installed, "icons.tenn")
	touchLib(t, shipped, "icons.tenn")
	got, err := Finder{Installed: []string{installed}, Shipped: []string{shipped}}.Resolve(Path{Name: "icons"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Resolve = %q, want the installed %q", got, want)
	}
}

// A shipped folder that does not have the name is skipped, and the next one is looked at.
func TestFinderShippedOrder(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	want := touchLib(t, second, "icons.tenn")
	got, err := Finder{Shipped: []string{first, second}}.Resolve(Path{Name: "icons"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Resolve = %q, want the second folder %q", got, want)
	}
	if _, err := (Finder{Shipped: []string{first, second}}).Resolve(Path{Name: "nope"}); err == nil {
		t.Fatal("an unknown name has no error")
	} else {
		for _, want := range []string{"library nope not found", "nope.tenn", first, second} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
	}
}

// TENNIARB_LIBRARIES is the first shipped folder, so a checkout of a library shadows the one of the
// application.
func TestShippedEnvFirst(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvLibraries, dir)
	shipped := Shipped(t.TempDir())
	if len(shipped) == 0 || shipped[0] != dir {
		t.Fatalf("Shipped = %v, want %q first", shipped, dir)
	}
}

// A libraries folder of the repository is found by walking up from the workspace, and a folder named
// libraries without a .tenn file is not taken for it.
func TestShippedRepoUp(t *testing.T) {
	t.Setenv(EnvLibraries, "")
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, "libraries"), 0o755) // no .tenn: not the folder
	workspace := filepath.Join(repo, "a", "b", "c")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := Shipped(workspace), filepath.Join(repo, "libraries"); containsDir(got, want) {
		t.Errorf("Shipped(%q) = %v already holds %q without a .tenn file", workspace, got, want)
	}
	libs := filepath.Join(repo, "libraries")
	touchLib(t, libs, "icons.tenn")
	if got := Shipped(workspace); !containsDir(got, libs) {
		t.Errorf("Shipped(%q) = %v, want %q", workspace, got, libs)
	}
}

func containsDir(dirs []string, dir string) bool {
	for _, d := range dirs {
		if d == dir {
			return true
		}
	}
	return false
}

// The installed libraries live in the config folder, under libraries.
func TestDefaultInstalled(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("TENNIARB_CONFIG_DIR", cfg)
	t.Setenv("GOWT_QA_DATA", "")
	f := Default(t.TempDir())
	if len(f.Installed) != 1 || f.Installed[0] != filepath.Join(cfg, "libraries") {
		t.Errorf("Default.Installed = %v, want %q", f.Installed, filepath.Join(cfg, "libraries"))
	}
}

// The same folder is looked at once, even when it is both installed and shipped.
func TestFinderNoDuplicateDirs(t *testing.T) {
	dir := t.TempDir()
	want := touchLib(t, dir, "icons.tenn")
	got, err := Finder{Installed: []string{dir}, Shipped: []string{dir}}.Resolve(Path{Name: "icons"})
	if err != nil || got != want {
		t.Fatalf("Resolve = %q, %v; want %q", got, err, want)
	}
}

// Contains tells a file of a library folder from a file of the workspace, including the folders that
// sit beside it.
func TestFinderContains(t *testing.T) {
	root := t.TempDir()
	lib := touchLib(t, root, "haiodo/c4.tenn")
	f := Finder{Shipped: []string{root}}
	if !f.Contains(lib) {
		t.Errorf("Contains(%q) = false", lib)
	}
	for _, p := range []string{filepath.Join(root, "..", "main.tenn"), filepath.Join(t.TempDir(), "c4.tenn"), ""} {
		if f.Contains(p) {
			t.Errorf("Contains(%q) = true", p)
		}
	}
}
