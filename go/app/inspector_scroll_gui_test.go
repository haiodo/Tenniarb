//go:build darwin || linux || windows

package app_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// inspScroll is the inspector pane and the container it scrolls, as the window has them now. gowt
// scrolls a Composite it makes itself, so the content is the one Composite child of the pane: the
// scroll bars beside it are ScrollBar controls.
func inspScroll(t *testing.T, app *qatest.Client) (pane, content qa.Node) {
	t.Helper()
	var find func(nodes []qa.Node) *qa.Node
	find = func(nodes []qa.Node) *qa.Node {
		for i := range nodes {
			if nodes[i].ID == "insp-scroll" {
				return &nodes[i]
			}
			if n := find(nodes[i].Children); n != nil {
				return n
			}
		}
		return nil
	}
	p := find(app.Dump())
	if p == nil {
		t.Fatal("no insp-scroll control in the window: is the inspector shown?")
	}
	for _, c := range p.Children {
		if c.Class == "Composite" {
			return *p, c
		}
	}
	t.Fatalf("the inspector pane scrolls no content, only %d children", len(p.Children))
	return
}

// inspFields lists the controls of the inspector, the pane excluded.
func inspFields(nodes []qa.Node) []qa.Node {
	var out []qa.Node
	for _, n := range nodes {
		if strings.HasPrefix(n.ID, "insp-") && n.Visible && n.ID != "insp-scroll" {
			out = append(out, n)
		}
		out = append(out, inspFields(n.Children)...)
	}
	return out
}

// TestInspectorScrolls keeps both halves of the inspector layout, which pull against each other: the
// content is taller than the panel and scrolls vertically, and no control is wider than the panel.
// 8e826d5 clamped the width and wrote 0 for the height, so the measured height was dropped and the
// long inspector stopped scrolling.
func TestInspectorScrolls(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	openText(t, app, "", "General info")

	// The item with the most properties in the fixture: pos, image, title, display and layer, so its
	// inspector is the longest one Example.tenn can show.
	rect, ok := insp(t, app).Screen["Main Window"]
	if !ok {
		t.Fatal("no Main Window on the canvas of General info")
	}
	app.Find(qa.ID("canvas")).ClickAt(center(rect))
	app.Settle()
	if s := insp(t, app); len(s.Selection) != 1 || s.Selection[0] != "Main Window" {
		t.Fatalf("click on the item selected %v", s.Selection)
	}

	pane, content := inspScroll(t, app)
	t.Logf("pane %v, content %v", pane.Bounds, content.Bounds)
	if content.Bounds[3] <= pane.Bounds[3] {
		t.Fatalf("the inspector content is %d points high in a %d pane: nothing is left to scroll",
			content.Bounds[3], pane.Bounds[3])
	}

	// The width is the half 8e826d5 fixed: the content asks for a little more than the panel - the rows
	// carry a width of their own now - so the pane holds it to its own width, and no control sticks out
	// of the window where it is cut.
	if content.Bounds[2] > pane.Bounds[2] {
		t.Fatalf("the inspector content is %d points wide in a %d pane", content.Bounds[2], pane.Bounds[2])
	}
	var over []string
	left, right := pane.Bounds[0], pane.Bounds[0]+pane.Bounds[2]
	for _, n := range inspFields(app.Dump()) {
		if n.Bounds[0] < left-1 || n.Bounds[0]+n.Bounds[2] > right+1 {
			over = append(over, fmt.Sprintf("%s %v", n.ID, n.Bounds))
		}
	}
	if len(over) > 0 {
		t.Fatalf("controls pass the inspector pane %d..%d: %s", left, right, strings.Join(over, ", "))
	}

	// Scrolling is geometry, not a picture: the content moves up in the window and the pane keeps it.
	before := content.Bounds[1]
	app.Find(qa.ID("insp-scroll")).ScrollBy(0, 200)
	app.Settle()
	_, after := inspScroll(t, app)
	if moved := before - after.Bounds[1]; moved < 100 {
		t.Fatalf("the inspector moved %d points of the 200 asked for: content %d high, pane %d",
			moved, after.Bounds[3], pane.Bounds[3])
	}
	// The bottom of the content is reachable: a scroll past the end stops there, and the top is off
	// screen, so the whole inspector can be seen.
	app.Find(qa.ID("insp-scroll")).ScrollBy(0, 10000)
	app.Settle()
	_, bottom := inspScroll(t, app)
	if seen := bottom.Bounds[1] + bottom.Bounds[3]; seen < pane.Bounds[1]+pane.Bounds[3] {
		t.Fatalf("the last row sits %d points above the bottom of the pane", pane.Bounds[1]+pane.Bounds[3]-seen)
	}
	if bottom.Bounds[1] >= pane.Bounds[1] {
		t.Fatalf("the content did not move up: %v in the pane %v", bottom.Bounds, pane.Bounds)
	}
}
