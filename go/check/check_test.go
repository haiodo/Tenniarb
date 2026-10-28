package check

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestExitCodes(t *testing.T) {
	td := "../scene/testdata/"
	tests := []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"clean", []string{td + "slots.tenn"}, ExitOK, "0 errors"},
		{"warnings only", []string{td + "nested_warn.tenn"}, ExitOK, ": warning: "},
		{"errors", []string{td + "layout_errors.tenn"}, ExitErrors, ": error: "},
		{"mount cycle", []string{td + "cycle_a.tenn"}, ExitErrors, "import cycle"},
		{"no files", nil, ExitUsage, ""},
		{"missing file", []string{td + "nope.tenn"}, ExitUsage, ""},
		{"bad flag", []string{"-nope", td + "slots.tenn"}, ExitUsage, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, _ := run(tt.args...)
			if code != tt.code || !strings.Contains(out, tt.out) {
				t.Errorf("exit %d, want %d; output:\n%s", code, tt.code, out)
			}
		})
	}
}

func TestJSONShape(t *testing.T) {
	f := "../scene/testdata/layout_errors.tenn"
	code, out, _ := run("-json", f)
	var ds []map[string]any
	if err := json.Unmarshal([]byte(out), &ds); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != ExitErrors || len(ds) == 0 {
		t.Fatalf("exit %d, %d diagnostics", code, len(ds))
	}
	for _, k := range []string{"file", "line", "col", "severity", "message"} {
		if _, ok := ds[0][k]; !ok {
			t.Errorf("no %q in %v", k, ds[0])
		}
	}
	if ds[0]["file"] != filepath.ToSlash(f) || ds[0]["severity"] != "error" {
		t.Errorf("first diagnostic %v", ds[0])
	}
	if _, out, _ = run("-json", "../scene/testdata/slots.tenn"); strings.TrimSpace(out) != "[]" {
		t.Errorf("clean file prints %q, want []", out)
	}
}

func TestEntryContext(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"design.tenn":      "element \"ui\" --from-file \"ui.tenn\"\nelement \"p\" --from-file \"apps/part.tenn\"\n",
		"ui.tenn":          "item Button { width 40; height 20 }\n",
		"apps/part.tenn":   "element \"Screen\" { use ui.Button as b }\n",
		"apps/orphan.tenn": "element \"Screen\" { use ui.Button as b }\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	part := filepath.Join(dir, "apps/part.tenn")
	for name, args := range map[string][]string{
		"found":    {"-root", dir, part},
		"explicit": {"-root", dir, "-entry", filepath.Join(dir, "design.tenn"), part},
		"entry":    {"-root", dir, filepath.Join(dir, "design.tenn")},
	} {
		if code, out, _ := run(args...); code != ExitOK {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
	}
	_, _, errOut := run("-root", dir, part)
	if !strings.Contains(errOut, "in the context of design.tenn") {
		t.Errorf("stderr %q", errOut)
	}
	if code, out, _ := run("-root", dir, filepath.Join(dir, "apps/orphan.tenn")); code != ExitErrors || !strings.Contains(out, "is not defined here or above") {
		t.Errorf("orphan: exit %d\n%s", code, out)
	}
	if code, _, errOut := run("-root", dir, "-entry", filepath.Join(dir, "ui.tenn"), part); code != ExitUsage || !strings.Contains(errOut, "does not mount") {
		t.Errorf("wrong entry: exit %d %q", code, errOut)
	}
}
