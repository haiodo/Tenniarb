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

// TestSceneDragCoveredItem: a press inside the selected item moves it, even when a child covers all of
// it — the item an outline or a link selects must be draggable from anywhere on its picture.
func TestSceneDragCoveredItem(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "covered.tenn")
	text := "element Screen {\n" +
		"\titem Panel { pos 0 0; width 240; height 120; auto-layout abs\n" +
		"\t\titem Fill { pos 0 0; width fill; height fill; color #cccccc }\n\t}\n}\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	defer h.open(fixture)
	h.open(file)
	a.State("command", "name", "close-others")
	a.Settle()
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Panel"})
	a.Settle()

	nodes := h.scene()
	var panel sceneNode
	for path, n := range nodes {
		if strings.HasSuffix(path, "Panel") || path == "Panel" {
			panel = n
		}
	}
	if panel.Box == nil {
		t.Fatalf("no Panel in %d nodes", len(nodes))
	}
	b := panel.Box
	x, y := b[0]+b[2]/2, b[1]+b[3]/2
	a.Find(qa.ID("canvas")).DragTo(x, y, x+40, y+20)
	a.Settle()

	got := h.text()
	if got == text || !strings.Contains(got, "item Panel {") {
		t.Fatalf("the drag changed nothing:\n%s", got)
	}
	if st := h.state(); len(st.Nodes) != 1 || st.Nodes[0] != "Panel" {
		t.Fatalf("the drag left %v selected", st.Nodes)
	}
	// the child stays where it was: the item moved, not the part inside it
	if !strings.Contains(got, "item Fill { pos 0 0;") {
		t.Fatalf("the child moved:\n%s", got)
	}
}

// TestSceneDragUnderLayer: an item under a transparent full-size layer takes the click, and the selected
// item drags from anywhere on its box, even where a layer above covers it. A transparent layer swallows
// nothing: a toolbar laid over a window would otherwise take every press of the element.
func TestSceneDragUnderLayer(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "layer.tenn")
	text := "element Screen {\n" +
		"\tauto-layout abs\n" +
		"\titem Under { pos 0 0; width 100; height 60; color #cccccc }\n" +
		"\titem Layer { pos -10 -10; auto-layout abs; width 300; height 300 }\n" +
		"}\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.Settle()
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{})
	a.Settle()

	under := h.scene()["Under"].Box
	x, y := under[0]+under[2]/2, under[1]+under[3]/2
	a.Find(qa.ID("canvas")).ClickAt(x, y)
	a.Settle()
	if st := h.state(); len(st.Nodes) != 1 || st.Nodes[0] != "Under" {
		t.Fatalf("a click through the layer selected %v, want Under", st.Nodes)
	}
	a.Find(qa.ID("canvas")).DragTo(x, y, x+30, y+15)
	restWait(a)
	if got := h.text(); !strings.Contains(got, "item Under { pos 30 -15;") {
		t.Fatalf("Under did not move:\n%s", got)
	}
}
