package app

import (
	"os"
	"path/filepath"

	"github.com/haiodo/gowt/gfx/offscreen"
)

// envFonts names font files or directories to register for this run, separated as PATH is.
const envFonts = "TENNIARB_FONTS"

// fontDirs are where the faces a document names by family come from: the fonts folder of the config
// directory, then what TENNIARB_FONTS names. A face stays out of the system: a document copied to
// another machine asks for the same fonts there.
func fontDirs(dir string) []string {
	out := []string{filepath.Join(dir, "fonts")}
	for _, p := range filepath.SplitList(os.Getenv(envFonts)) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// registerFonts makes those files usable by name. It runs before the first text is measured: a family
// looked up earlier keeps the fallback the backend picked for it.
func registerFonts(dir string) int {
	n := 0
	for _, p := range fontDirs(dir) {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if k, err := offscreen.RegisterFonts(p); err == nil {
			n += k
		}
	}
	return n
}
