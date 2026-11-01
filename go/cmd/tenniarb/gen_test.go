package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genSource is a component with a template and one use that sets its parameter.
const genSource = `element "Chip" {
    kind Component
    export let label = "Chip"
    template %{<span>${label}</span>}
}

element "Screen" {
    item col {
        auto-layout column
        use Chip as a { label "Hi" }
    }
}
`

// TestGen: the text of the template on stdout, the values of -set and of the block of a use applied, and
// the exit codes of check: 0 done, 1 errors, 2 usage or an unreadable file.
func TestGen(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "chip.tenn")
	if err := os.WriteFile(file, []byte(genSource), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args   []string
		code   int
		out    string
		errOut string
	}{
		{args: []string{file, "Chip"}, out: "<span>Chip</span>\n"},
		{args: []string{"-set", "label=Hi", file, "Chip"}, out: "<span>Hi</span>\n"},
		{args: []string{file, "Screen.col.a"}, out: "<span>Hi</span>\n"},
		{args: []string{"-set", "nope=1", file, "Chip"}, code: 1, errOut: `unknown parameter "nope"`},
		{args: []string{file, "Nowhere"}, code: 1, errOut: `"Nowhere" is not defined`},
		{args: []string{file}, code: 2, errOut: "usage: tenniarb gen"},
		{args: []string{file, "Chip", "extra"}, code: 2, errOut: "usage: tenniarb gen"},
		{args: []string{"-set", "label", file, "Chip"}, code: 2, errOut: "want name=value"},
		{args: []string{filepath.Join(dir, "missing.tenn"), "Chip"}, code: 2, errOut: "no such file"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		if code := gen(c.args, &out, &errOut); code != c.code || out.String() != c.out ||
			!strings.Contains(errOut.String(), c.errOut) {
			t.Errorf("%v: code %d, stdout %q, stderr %q; want code %d, stdout %q, stderr %q",
				c.args, code, out.String(), errOut.String(), c.code, c.out, c.errOut)
		}
	}
}
