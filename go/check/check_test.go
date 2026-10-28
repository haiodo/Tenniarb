package check

import (
	"bytes"
	"encoding/json"
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
