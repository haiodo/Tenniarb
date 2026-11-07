// Package fontset registers the font files a document ships with, so `font-family` can name a face the
// system does not have.
package fontset

import (
	"os"
	"path/filepath"

	"github.com/haiodo/gowt/gfx/offscreen"
)

// Env names font files or directories to register for this run, separated as PATH is.
const Env = "TENNIARB_FONTS"

// Dirs are where the faces come from: the fonts folder of the config directory, then what Env names.
// Nothing is installed into the system: a document copied to another machine asks for the same fonts
// there.
func Dirs(configDir string) []string {
	out := []string{filepath.Join(configDir, "fonts")}
	for _, p := range filepath.SplitList(os.Getenv(Env)) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Register makes those files usable by name. It runs before the first text is measured: a family
// looked up earlier keeps the fallback the backend picked for it.
func Register(configDir string) int {
	n := 0
	for _, p := range Dirs(configDir) {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if k, err := offscreen.RegisterFonts(p); err == nil {
			n += k
		}
	}
	return n
}
