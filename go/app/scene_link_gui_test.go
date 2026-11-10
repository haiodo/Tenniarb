//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// TestSceneCtrlDragDrawsLink: Ctrl and a drag from one item to another writes `link A B` in the scene, as
// Swift did; a plain drag moves the item instead.
func TestSceneCtrlDragDrawsLink(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "link.tenn")
	text := "element Screen {\n" +
		"\titem A { pos 0 0; width 120; height 60; auto-layout abs; title \"A\" }\n" +
		"\titem B { pos 300 0; width 120; height 60; auto-layout abs; title \"B\" }\n}\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	defer h.open(fixture)
	h.open(file)
	a.State("command", "name", "close-others")
	a.Settle()
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"A"})
	a.Settle()

	boxes := h.scene()
	t.Logf("scene state: %d nodes, text %d bytes", len(boxes), len(h.text()))
	t.Logf("state: %+v", h.state())
	var from, to []float64
	var okFrom, okTo bool
	for path, n := range boxes {
		if n.Box == nil {
			continue
		}
		switch {
		case strings.HasSuffix(path, "A"):
			from, okFrom = n.Box, true
		case strings.HasSuffix(path, "B"):
			to, okTo = n.Box, true
		}
	}
	if !okFrom || !okTo {
		t.Fatalf("no A and B among %d nodes", len(boxes))
	}
	canvas := a.Find(qa.ID("canvas"))
	canvas.Pointer(qa.PointerEvent{
		Kind: "drag", Mods: "Ctrl",
		X: from[0] + from[2]/2, Y: from[1] + from[3]/2,
		ToX: to[0] + to[2]/2, ToY: to[1] + to[3]/2,
	})
	a.Settle()

	got := h.text()
	if !strings.Contains(got, "link A B") {
		t.Fatalf("no link in the file:\n%s", got)
	}
	if st := h.state(); len(st.Nodes) != 1 {
		t.Fatalf("the link is not selected: %v", st.Nodes)
	}

	// A plain drag moves the item and writes no link: the gesture is the Ctrl, not the drag.
	canvas.DragTo(from[0]+from[2]/2, from[1]+from[3]/2, from[0]+from[2]/2+60, from[1]+from[3]/2+30)
	a.Settle()
	if moved := h.text(); strings.Count(moved, "link ") != 1 {
		t.Fatalf("a plain drag wrote a link:\n%s", moved)
	}
}
