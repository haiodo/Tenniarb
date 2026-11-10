package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// check -fix rewrites the file, and the check that follows finds it clean: the whole way a reader of the
// error is meant to go. A title is quoted and a long pos is rounded in the same run.
func TestCheckFixesTitlesAndPos(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "e.tenn")
	const text = `element "E" {
    item A {
        title Gap and padding
        pos -666.55078125 142.05078125
    }
}
`
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"-fix", "-root", dir, file}, &out, &errOut); code != ExitOK {
		t.Fatalf("check -fix: %d\n%s%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "fixed 3 place(s)") {
		t.Errorf("no report of the fix: %s", errOut.String())
	}
	fixed, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`title "Gap and padding"`, "pos -666.55 142.05"} {
		if !strings.Contains(string(fixed), want) {
			t.Fatalf("%q is not fixed:\n%s", want, fixed)
		}
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"-root", dir, file}, &out, &errOut); code != ExitOK {
		t.Fatalf("check after the fix: %d\n%s%s", code, out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "title takes one value") {
		t.Fatalf("the error is still reported:\n%s", out.String())
	}
}
