package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFontDirs: the config fonts come first, TENNIARB_FONTS adds the paths it names, and empty entries
// of the list are nothing to register.
func TestFontDirs(t *testing.T) {
	sep := string(os.PathListSeparator)
	t.Setenv(envFonts, filepath.Join("a", "b")+sep+sep+filepath.Join("c", "d"))
	got := fontDirs(filepath.Join("cfg"))
	want := []string{filepath.Join("cfg", "fonts"), filepath.Join("a", "b"), filepath.Join("c", "d")}
	if len(got) != len(want) {
		t.Fatalf("fontDirs = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fontDirs = %q, want %q", got, want)
		}
	}

	t.Setenv(envFonts, "")
	if got := fontDirs("cfg"); len(got) != 1 || got[0] != filepath.Join("cfg", "fonts") {
		t.Fatalf("without TENNIARB_FONTS: %q", got)
	}
}
