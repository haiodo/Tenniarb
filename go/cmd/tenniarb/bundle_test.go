package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bundleFiles is the main file and its libraries:
//
//	kit.lib.tenn mounts parts/button.tenn, and lib.tenn mounts kit.lib.tenn.
const (
	bundleMain = `// the screen
element "kit" --from-file "lib.tenn"
element "Screen" {
  use kit.Parts.Button as ok
}
`
	bundleLib = `// the library
element Parts --from-file "parts/button.tenn"
`
	bundleParts = `element Button {
  item t { title "ok" }
}
`
)

func writeBundle(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBundle: a mount becomes a block with the content of the file, a mount of a mount too, and the
// text around them stays as written.
func TestBundle(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, map[string]string{
		"main.tenn":         bundleMain,
		"lib.tenn":          bundleLib,
		"parts/button.tenn": bundleParts,
	})
	main := filepath.Join(dir, "main.tenn")
	want := `// the screen
element "kit" {
  // the library
  element Parts {
    element Button {
      item t { title "ok" }
    }
  }
}
element "Screen" {
  use kit.Parts.Button as ok
}
`
	var out, errOut bytes.Buffer
	if code := bundleCmd([]string{main}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
	// Bundling the result changes nothing: it has no mounts left.
	once := filepath.Join(dir, "main.bundle.tenn")
	if err := os.WriteFile(once, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var again, errOut2 bytes.Buffer
	if code := bundleCmd([]string{once}, &again, &errOut2); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut2.String())
	}
	if again.String() != out.String() {
		t.Errorf("not idempotent:\n%s", again.String())
	}
}

// TestBundleLiteral: the bytes of a multi-line template are the text it generates, so the lines of
// it keep their indentation when the file moves into a block.
func TestBundleLiteral(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, map[string]string{
		"main.tenn": "element \"x\" --from-file \"lib.tenn\"\n",
		"lib.tenn":  "element T {\n  template %{<p>\ntext</p>%}\n}\n",
	})
	var out, errOut bytes.Buffer
	if code := bundleCmd([]string{filepath.Join(dir, "main.tenn")}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	want := "element \"x\" {\n  element T {\n    template %{<p>\ntext</p>%}\n  }\n}\n"
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

// TestBundleProblems: a cycle and a file that cannot be read are errors with the paths; with -o the
// result goes to the file.
func TestBundleProblems(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, map[string]string{
		"cycle.tenn":    "element \"a\" --from-file \"b.tenn\"\n",
		"b.tenn":        "element \"b\" --from-file \"cycle.tenn\"\n",
		"missing.tenn":  "element \"x\" --from-file \"nowhere.tenn\"\n",
		"comments.tenn": "// keep me\n\nelement \"x\" --from-file \"lib.tenn\"\n",
		"lib.tenn":      "element X { item a { title b } }\n",
	})
	cases := []struct {
		file, errOut string
		code         int
	}{
		{"cycle.tenn", "import cycle: cycle.tenn imports b.tenn, b.tenn imports cycle.tenn", 1},
		{"missing.tenn", "nowhere.tenn", 1},
		{"comments.tenn", "", 0},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		code := bundleCmd([]string{filepath.Join(dir, c.file)}, &out, &errOut)
		if code != c.code || !strings.Contains(errOut.String(), c.errOut) {
			t.Errorf("%s: exit %d, stderr %q", c.file, code, errOut.String())
		}
	}
	// The blank line and the comment before the mount are kept.
	var out, errOut bytes.Buffer
	if code := bundleCmd([]string{filepath.Join(dir, "comments.tenn")}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "// keep me\n\nelement \"x\" {\n  element X { item a { title b } }\n}\n") {
		t.Errorf("got %q", out.String())
	}
	// -o writes the file.
	dst := filepath.Join(dir, "out.tenn")
	if code := bundleCmd([]string{"-o", dst, filepath.Join(dir, "comments.tenn")}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != out.String() {
		t.Errorf("-o wrote %q, %v", string(b), err)
	}
}
