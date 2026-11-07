package fontset

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDirs: the config fonts come first, Env adds the paths it names, and empty entries of the list are
// nothing to register.
func TestDirs(t *testing.T) {
	sep := string(os.PathListSeparator)
	t.Setenv(Env, filepath.Join("a", "b")+sep+sep+filepath.Join("c", "d"))
	got := Dirs(filepath.Join("cfg"))
	want := []string{filepath.Join("cfg", "fonts"), filepath.Join("a", "b"), filepath.Join("c", "d")}
	if len(got) != len(want) {
		t.Fatalf("Dirs = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Dirs = %q, want %q", got, want)
		}
	}

	t.Setenv(Env, "")
	if got := Dirs("cfg"); len(got) != 1 || got[0] != filepath.Join("cfg", "fonts") {
		t.Fatalf("without %s: %q", Env, got)
	}
}
