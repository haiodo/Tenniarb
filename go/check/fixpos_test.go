package check

import (
	"testing"

	"github.com/haiodo/tenn/go"
)

// FixTitles rounds a pos value with more than two decimals, so a scene imported from a `.pen` reads a
// point a reader can use: `pos -666.55078125 142.05078125` as `pos -666.55 142.05`. It changes only the
// numbers that are too long: an expression, an integer or a value already short stays as it is.
func TestFixPos(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		n              int
	}{
		{
			name: "long values of one pos",
			in:   "element \"E\" {\n  item A { pos -666.55078125 142.05078125 }\n}\n",
			want: "element \"E\" {\n  item A { pos -666.55 142.05 }\n}\n",
			n:    2,
		},
		{
			name: "three pos in one file",
			in:   "element \"E\" {\n  item A { pos -666.55078125 142 }\n  item B { pos 12 63.385 }\n  item C { pos -46.269999999999996 0 }\n}\n",
			want: "element \"E\" {\n  item A { pos -666.55 142 }\n  item B { pos 12 63.39 }\n  item C { pos -46.27 0 }\n}\n",
			n:    3,
		},
		{
			name: "short values stay as they are",
			in:   "element \"E\" {\n  item A { pos 0 0 }\n  item B { pos -555.0 -189.0 }\n  item C { pos 12.5 3 }\n}\n",
			want: "element \"E\" {\n  item A { pos 0 0 }\n  item B { pos -555.0 -189.0 }\n  item C { pos 12.5 3 }\n}\n",
			n:    0,
		},
		{
			name: "negative and float noise",
			in:   "element \"E\" {\n  item A { pos 63.385 -46.269999999999996 }\n}\n",
			want: "element \"E\" {\n  item A { pos 63.39 -46.27 }\n}\n",
			n:    2,
		},
		{
			name: "a value that rounds to a whole number keeps the style of a double",
			in:   "element \"E\" {\n  item A { pos -183.0004 900.999 }\n}\n",
			want: "element \"E\" {\n  item A { pos -183.0 901.0 }\n}\n",
			n:    2,
		},
		{
			name: "an expression is not a number and is left alone",
			in:   "element \"E\" {\n  item A { pos ${x} -46.269999999999996; color red }\n}\n",
			want: "element \"E\" {\n  item A { pos ${x} -46.27; color red }\n}\n",
			n:    1,
		},
		{
			name: "only expressions",
			in:   "element \"E\" {\n  item A { pos ${x} ${y} }\n}\n",
			want: "element \"E\" {\n  item A { pos ${x} ${y} }\n}\n",
			n:    0,
		},
		{
			name: "a body that mentions pos is not a command",
			in:   "element \"E\" {\n  item A { body %{pos 12.345 6.789} }\n}\n",
			want: "element \"E\" {\n  item A { body %{pos 12.345 6.789} }\n}\n",
			n:    0,
		},
		{
			name: "a title and a pos in one run",
			in:   "element \"E\" {\n  item A { title Gap and padding; pos -666.55078125 142.05 }\n}\n",
			want: "element \"E\" {\n  item A { title \"Gap and padding\"; pos -666.55 142.05 }\n}\n",
			n:    2,
		},
		{
			name: "a byte order mark is kept and does not move the numbers",
			in:   "\xef\xbb\xbfelement \"E\" {\n  item A { pos -1.2345 2 }\n}\n",
			want: "\xef\xbb\xbfelement \"E\" {\n  item A { pos -1.23 2 }\n}\n",
			n:    1,
		},
		{
			name: "a text that does not parse stays as it is",
			in:   "element \"E\" {\n  item A { pos -666.55078125 142.05\n",
			want: "element \"E\" {\n  item A { pos -666.55078125 142.05\n",
			n:    0,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, n := FixTitles(c.in)
			if got != c.want {
				t.Errorf("text:\n%s\nwant:\n%s", got, c.want)
			}
			if n != c.n {
				t.Errorf("%d places changed, want %d", n, c.n)
			}
			// The fix is a fix: it changes nothing the second time.
			if again, n2 := FixTitles(got); again != got || n2 != 0 {
				t.Errorf("a second run changed the text again: %d", n2)
			}
		})
	}
}

// roundPos is the rule on its own: a pos command of the tree, rounded, and the rest of the text left to
// the other rules. A title of the same text keeps its words, and an expression of a pos is not a number.
func TestRoundPos(t *testing.T) {
	cases := []struct {
		in, want string
		n        int
	}{
		{`element "E" { item A { pos -666.55078125 142.05078125 } }`,
			`element "E" { item A { pos -666.55 142.05 } }`, 2},
		{`element "E" { item A { pos 0 0 } }`,
			`element "E" { item A { pos 0 0 } }`, 0},
		{`element "E" { item A { pos ${x} 3.14159 } }`,
			`element "E" { item A { pos ${x} 3.14 } }`, 1},
		{`element "E" { item A { title Gap and padding; pos 1.2345 2 } }`,
			`element "E" { item A { title Gap and padding; pos 1.23 2 } }`, 1},
	}
	for _, c := range cases {
		root, parser := tenn.Parse(c.in)
		if parser.Errors.HasErrors() {
			t.Fatalf("%q: %v", c.in, parser.Errors)
		}
		got, n := roundPos(c.in, root)
		if n != c.n {
			t.Errorf("%q: fixed %d, want %d", c.in, n, c.n)
		}
		if got != c.want {
			t.Errorf("%q:\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}
