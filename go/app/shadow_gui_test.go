//go:build darwin || linux || windows

package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// TestShadowFieldsFitTheValueColumn: X, Y and blur of a shadow are three fields where the value column of
// the inspector holds two, and squeezing all three into one row left X and Y thirteen points wide and the
// blur nothing at all, outside the panel. Each has room now.
func TestShadowFieldsFitTheValueColumn(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "shadow")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../docs/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "Example.tenn")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	app.Answer(qa.Dialog{Kind: "message", Reply: "No"})
	app.Answer(qa.Dialog{Kind: "file", Reply: file})
	menu(app, "Open...")
	app.State("command", "name", "close-others")
	app.Settle()
	app.Find(qa.Text("Basic steps").In(qa.ID("outline"))).Click()
	app.Settle()
	app.Find(qa.Text("Big yellow item").In(qa.ID("items"))).Click()
	app.Settle()

	boxes := map[string][4]int{}
	var walk func([]qa.Node)
	walk = func(list []qa.Node) {
		for _, n := range list {
			if strings.Contains(n.ID, "shadow-") {
				boxes[n.ID] = n.Bounds
			}
			walk(n.Children)
		}
	}
	walk(app.Dump())
	for _, id := range []string{"insp-shadow-0-x", "insp-shadow-0-y", "insp-shadow-0-blur", "insp-shadow-0-color"} {
		b, ok := boxes[id]
		if !ok {
			t.Fatalf("%s is not there; the inspector shows %v", id, keysOf(boxes))
		}
		if b[2] < 30 {
			t.Errorf("%s is %d points wide: %v", id, b[2], b)
		}
	}
}

// keysOf names the shadow controls a dump held, for a message about the ones that are missing.
func keysOf(m map[string][4]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, fmt.Sprintf("%s%v", k, m[k]))
	}
	return out
}
