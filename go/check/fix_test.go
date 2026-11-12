package check

import (
	"testing"

	"github.com/haiodo/tenn/go"
)

// quoteTitles is the fix a reader of the error is told to make: the words of a title in one quoted text.
// It must leave every other title alone - one already quoted, one word, a body - and rewrite only the
// arguments, not the name of the command or the rest of the line.
func TestQuoteTitles(t *testing.T) {
	cases := []struct {
		in, want string
		n        int
	}{
		{`element "E" { item A { title Gap and padding } }`,
			`element "E" { item A { title "Gap and padding" } }`, 1},
		{`element "E" { item A { title "Gap and padding" } }`,
			`element "E" { item A { title "Gap and padding" } }`, 0},
		{`element "E" { item A { title One } }`,
			`element "E" { item A { title One } }`, 0},
		{"element \"E\" { item A { title %{two\nlines} } }",
			"element \"E\" { item A { title %{two\nlines} } }", 0},
		{`element "E" { use X as Y { title A B; color red } }`,
			`element "E" { use X as Y { title "A B"; color red } }`, 1},
		{`element "E" {
    item A { title Gap and padding }
    item B { title Also here }
}`,
			`element "E" {
    item A { title "Gap and padding" }
    item B { title "Also here" }
}`, 2},
	}
	for _, c := range cases {
		root, parser := tenn.Parse(c.in)
		if parser.Errors.HasErrors() {
			t.Fatalf("%q: %v", c.in, parser.Errors)
		}
		got, n := quoteTitles(c.in, root)
		if n != c.n {
			t.Errorf("%q: fixed %d, want %d", c.in, n, c.n)
		}
		if got != c.want {
			t.Errorf("%q:\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}
