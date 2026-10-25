package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const two = `element A {
    item X { pos 0 0 }
    item Y { pos 100 0 }
    link X Y
}
element B {
    item Z
}
element Empty {
}
`

func runCLI(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tenn")
	if err := os.WriteFile(in, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := run(append(args, in), &stderr)
	return code, dir, stderr.String()
}

func TestSVGOfFirstElementWithItems(t *testing.T) {
	out := filepath.Join(t.TempDir(), "o.svg")
	code, _, stderr := runCLI(t, two, "-o", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	data, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(data), "<svg ") || !strings.Contains(string(data), "<text") {
		t.Errorf("svg: %.80s", data)
	}
}

func TestSVGBackground(t *testing.T) {
	out := filepath.Join(t.TempDir(), "o.svg")
	if code, _, e := runCLI(t, two, "-bg", "white", "-o", out); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `</defs><rect x="0" y="0"`) || !strings.Contains(string(data), `fill="rgb(255,255,255)"`) {
		t.Errorf("no background rect: %.300s", data)
	}
}

func TestPNGScaleAndBackground(t *testing.T) {
	dir := t.TempDir()
	p1, p2 := filepath.Join(dir, "1.png"), filepath.Join(dir, "2.png")
	if code, _, e := runCLI(t, two, "-scale", "1", "-element", "B", "-bg", "#e7e9eb", "-o", p1); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if code, _, e := runCLI(t, two, "-scale", "2", "-element", "B", "-o", p2); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	a, _ := os.ReadFile(p1)
	b, _ := os.ReadFile(p2)
	if !bytes.HasPrefix(a, []byte("\x89PNG")) || !bytes.HasPrefix(b, []byte("\x89PNG")) {
		t.Fatal("not PNG")
	}
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "o.svg")
	if code, _, e := runCLI(t, two); code != 1 || !strings.Contains(e, "usage:") {
		t.Errorf("no -o: %d %q", code, e)
	}
	if code, _, _ := runCLI(t, two, "-o", filepath.Join(dir, "o.txt")); code != 1 {
		t.Errorf("bad extension: %d", code)
	}
	if code, _, _ := runCLI(t, two, "-o", out, "-scale", "0"); code != 1 {
		t.Errorf("bad scale: %d", code)
	}
	if code := run([]string{"-o", out, filepath.Join(dir, "missing.tenn")}, &bytes.Buffer{}); code != 1 {
		t.Errorf("missing input: %d", code)
	}
	if code, _, e := runCLI(t, "element A {\n  item X {", "-o", out); code != 2 || !strings.Contains(e, "in.tenn:") {
		t.Errorf("parse error: %d %q", code, e)
	}
	if code, _, e := runCLI(t, two, "-o", out, "-element", "Nope"); code != 3 || !strings.Contains(e, "element not found") {
		t.Errorf("unknown element: %d %q", code, e)
	}
	if code, _, _ := runCLI(t, "element E {\n}\n", "-o", out); code != 3 {
		t.Errorf("nothing to render: %d", code)
	}
}

func TestExpressionsEvaluatedUnlessNoEval(t *testing.T) {
	src := "element A {\n    item X {\n        pos 0 0\n        title ${\"ab\" + \"cd\"}\n    }\n}\n"
	for _, tc := range []struct {
		args []string
		want string
	}{{nil, ">abcd<"}, {[]string{"-no-eval"}, "&#34;ab&#34; + &#34;cd&#34;"}} {
		out := filepath.Join(t.TempDir(), "o.svg")
		if code, _, e := runCLI(t, src, append(tc.args, "-o", out)...); code != 0 {
			t.Fatalf("exit %d: %s", code, e)
		}
		data, _ := os.ReadFile(out)
		if !strings.Contains(string(data), tc.want) {
			t.Errorf("%v: want %q in %s", tc.args, tc.want, data)
		}
	}
}
