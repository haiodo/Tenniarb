package editor

import (
	"testing"
)

// The preparation of a body: escaped newlines expanded, the common indent taken away, the empty first and
// last line dropped, a tab as four blanks, and a blank line kept as a blank of its own.
func TestPrepareBodyText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"one\ntwo", "one\ntwo"},
		{"  indented\n  lines", "indented\nlines"},
		{"\n body\n", "body"},
		{"a\\nb", "a\nb"},
		{"tab\there", "tab    here"},
		{"\n\n body\n tail \n", " \nbody\ntail "},
		{"", ""},
	} {
		if got := prepareBodyText(c.in); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}
