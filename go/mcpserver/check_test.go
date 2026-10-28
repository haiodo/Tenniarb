package mcpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/check"
)

// tenniarb check must report what MCP validate reports for the same file and root.
func TestCheckMatchesValidate(t *testing.T) {
	scenes, _ := filepath.Glob("../scene/testdata/*.tenn")
	classic, _ := filepath.Glob("../../web/fixtures/Example.tenn")
	files := append(scenes, classic...)
	if len(scenes) == 0 || len(classic) == 0 {
		t.Fatal("fixtures not found")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			dir, base := filepath.Split(mustAbs(t, f))
			s, err := New(dir)
			if err != nil {
				t.Fatal(err)
			}
			text, err := s.validate(base)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, ln := range strings.Split(text, "\n")[1:] {
				if ln != "" && !strings.Contains(ln, " errors, ") && !strings.Contains(ln, "imported files not listed") {
					want = append(want, ln)
				}
			}
			var out, errOut bytes.Buffer
			code := check.Run([]string{"-json", "-root", dir, mustAbs(t, f)}, &out, &errOut)
			var ds []check.Diag
			if err := json.Unmarshal(out.Bytes(), &ds); err != nil {
				t.Fatalf("json: %v\n%s", err, out.String())
			}
			var got []string
			nerr := 0
			for _, d := range ds {
				rel, _ := filepath.Rel(dir, mustAbs(t, d.File))
				rel = filepath.ToSlash(rel)
				if d.Severity == "warning" {
					if rel == base { // MCP lists only warnings of the file itself
						got = append(got, fmt.Sprintf("%s:%d:%d: warning: %s", rel, d.Line, d.Col, d.Message))
					}
					continue
				}
				nerr++
				got = append(got, fmt.Sprintf("%s:%d:%d: %s", rel, d.Line, d.Col, d.Message))
			}
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("check:\n%s\nvalidate:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if (nerr > 0) != (code == check.ExitErrors) || errOut.Len() > 0 {
				t.Errorf("exit %d with %d errors, stderr %q", code, nerr, errOut.String())
			}
		})
	}
}

func mustAbs(t *testing.T, p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
