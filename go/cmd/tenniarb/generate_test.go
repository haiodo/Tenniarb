package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateSource is a screen whose code is the template of the only use below it, wrapped by a template
// of its own.
const generateSource = `element "Chip" {
    kind Component
    export let label = "Chip"
    template %{<span>${label}</span>}
}

element "Screen" {
    template %{<main><slot/></main>}
    item col {
        auto-layout column
        use Chip as a { label "Hi" }
    }
}
`

// TestGenerate: the code of the uses below an element from their templates, the element's own template or
// -template around them, and the exit codes of check: 0 done, 1 errors, 2 usage or an unreadable file.
func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "screen.tenn")
	if err := os.WriteFile(file, []byte(generateSource), 0o600); err != nil {
		t.Fatal(err)
	}
	wrap := filepath.Join(dir, "wrap.tpl")
	if err := os.WriteFile(wrap, []byte("<x>\n<slot/>\n</x>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args   []string
		code   int
		out    string
		errOut string
	}{
		{args: []string{file, "Screen"}, out: "<main><span>Hi</span></main>\n"},
		{args: []string{file, "Screen.col"}, out: "<span>Hi</span>\n"},
		{args: []string{file, "Screen.col.a"}, out: "<span>Hi</span>\n"},
		{args: []string{"-template", "<x><slot/></x>", file, "Screen"}, out: "<x><span>Hi</span></x>\n"},
		{args: []string{"-template", "@" + wrap, file, "Screen"}, out: "<x>\n<span>Hi</span>\n</x>\n"},
		{args: []string{"-template", "@" + filepath.Join(dir, "no.tpl"), file, "Screen"}, code: 2, errOut: "no.tpl"},
		{args: []string{file}, code: 2, errOut: "usage: tenniarb generate"},
		{args: []string{file, "Nowhere"}, code: 1, errOut: `"Nowhere" is not defined`},
		{args: []string{filepath.Join(dir, "missing.tenn"), "Screen"}, code: 2, errOut: "missing.tenn"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		if code := generate(c.args, &out, &errOut); code != c.code || out.String() != c.out ||
			!strings.Contains(errOut.String(), c.errOut) {
			t.Errorf("%v: code %d, stdout %q, stderr %q; want code %d, stdout %q, stderr %q",
				c.args, code, out.String(), errOut.String(), c.code, c.out, c.errOut)
		}
	}
}
