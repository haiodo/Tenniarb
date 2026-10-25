package penimport

import (
	"os"
	"path/filepath"
	"testing"
)

// The real design files change, so the test asserts only that they convert, that go/scene accepts
// the result and that most of the boxes agree; the numbers go to the log.
func TestRealFiles(t *testing.T) {
	dir := os.Getenv("PEN_DESIGN_DIR")
	if dir == "" {
		t.Skip("set PEN_DESIGN_DIR to the folder with fusio-1621.pen, fusio-1613.pen and ui-kit.lib.pen")
	}
	files := []string{"fusio-1621.pen", "fusio-1613.pen"}
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f)
		if _, err := os.Stat(p); err != nil {
			t.Skipf("no %s", p)
		}
		paths = append(paths, p)
	}
	res, err := Convert(paths, Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := res.Write(out); err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Files {
		rep, err := Compare(f, out, f.Name != "ui-kit.lib")
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Problems) > 0 {
			t.Fatalf("%s: %v", f.Name, rep.Problems)
		}
		compared, within := 0, 0
		for _, s := range rep.Screens {
			compared += s.Compared
			within += s.Within
		}
		t.Logf("%s: %d screens, %d boxes, %d within 0.5 (%d warnings, %d unsupported kinds)", f.Name, len(rep.Screens), compared, within, len(f.Warnings), len(f.Unsupported))
		if compared > 0 && float64(within) < 0.9*float64(compared) {
			t.Errorf("%s: only %d of %d boxes agree", f.Name, within, compared)
		}
	}
}
