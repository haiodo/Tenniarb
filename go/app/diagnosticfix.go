//go:build darwin || linux || windows

package app

import (
	"fmt"

	"github.com/haiodo/tenniarb/go/check"
)

// fixDiagnostics repairs what the banner reports, as one undo step: the titles the diagnostics name are
// quoted, and the document is read again from the new text, which clears the banner when nothing is left.
// A reader who is not at a terminal gets the fix here, in the window, instead of a command to run.
func (v *Viewer) fixDiagnostics() {
	if v.session == nil {
		return
	}
	fixed, n := check.FixTitles(v.text)
	if n == 0 {
		v.setNote("Nothing in this document can be fixed that way")
		return
	}
	if err := v.applyText(0, len(v.text), fixed); err != nil {
		v.setBanner(err.Error())
		return
	}
	v.setNote(fmt.Sprintf("Fixed %d %s", n, plural(n, "title", "titles")))
}

// plural picks the word of a count: one title, two titles.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
