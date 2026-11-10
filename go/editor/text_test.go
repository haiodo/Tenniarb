package editor

import (
	"testing"

	"github.com/haiodo/tenniarb/go/render"
)

// The editor's preparation of a body is the old renderer's: the text the user edits must be the text the
// picture showed, and the two copies become one when the old engine goes.
func TestPrepareBodyTextLikeTheOldOne(t *testing.T) {
	for _, s := range []string{
		"one\ntwo",
		"  indented\n  lines",
		"\n body\n",
		"a\\nb",
		"tab\there",
		"\n\n body\n tail \n",
		"",
	} {
		if got, want := prepareBodyText(s), render.PrepareBodyText(s); got != want {
			t.Errorf("%q: %q, the old renderer gave %q", s, got, want)
		}
	}
}
