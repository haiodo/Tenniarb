package check

import "testing"

// FixTitles is the fix of the diagnostic a window shows: the words of a title written bare are quoted, so
// a reader who is not at a terminal has the same repair as `check -fix`.
func TestFixTitles(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		n              int
	}{
		{
			name: "bare words",
			in:   "element \"E\" {\n  item A { auto-layout column; title Gap and padding }\n}\n",
			want: "element \"E\" {\n  item A { auto-layout column; title \"Gap and padding\" }\n}\n",
			n:    1,
		},
		{
			name: "two titles",
			in:   "element \"E\" {\n  item A { auto-layout column; title One two }\n  item B { auto-layout column; title Three four }\n}\n",
			want: "element \"E\" {\n  item A { auto-layout column; title \"One two\" }\n  item B { auto-layout column; title \"Three four\" }\n}\n",
			n:    2,
		},
		{
			name: "already quoted",
			in:   "element \"E\" {\n  item A { title \"Gap and padding\" }\n}\n",
			want: "element \"E\" {\n  item A { title \"Gap and padding\" }\n}\n",
			n:    0,
		},
		{
			name: "one word needs no quote",
			in:   "element \"E\" {\n  item A { title Alone }\n}\n",
			want: "element \"E\" {\n  item A { title Alone }\n}\n",
			n:    0,
		},
		{
			name: "a text that does not parse stays as it is",
			in:   "element \"E\" {\n  item A { title Gap and padding\n",
			want: "element \"E\" {\n  item A { title Gap and padding\n",
			n:    0,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, n := FixTitles(c.in)
			if got != c.want {
				t.Errorf("text:\n%s\nwant:\n%s", got, c.want)
			}
			if n != c.n {
				t.Errorf("%d titles changed, want %d", n, c.n)
			}
			// The fix is a fix: it changes nothing the second time.
			if again, n2 := FixTitles(got); again != got || n2 != 0 {
				t.Errorf("a second run changed the text again: %d", n2)
			}
		})
	}
}

// Fixable names the diagnostics the fixer repairs: a reader without a terminal gets a Fix button for those
// and for nothing else.
func TestFixable(t *testing.T) {
	for _, c := range []struct {
		diagnostic string
		want       bool
	}{
		{`tenniarb.tenn:2922:78: error: title takes one value: quote the words, title "Gap and padding" (tenniarb check -fix does it)`, true},
		{`a.tenn:1:44: title takes one value: quote the words, title "One two"`, true},
		{`a.tenn:2:1: path f.C: nothing named "C" under "f"`, false},
		{`a.tenn:3:17: warning: kind "Person" is not one of component, screen, library, application`, false},
		{"", false},
	} {
		if got := Fixable(c.diagnostic); got != c.want {
			t.Errorf("%q: fixable %v, want %v", c.diagnostic, got, c.want)
		}
	}
}
