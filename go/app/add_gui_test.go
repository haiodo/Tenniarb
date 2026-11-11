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

// TestAddFrame is Edit > Add > Frame in an empty screen: a frame with a size lands on the canvas,
// selected, and the item list takes its name.
func TestAddFrame(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "screen.tenn")
	if err := os.WriteFile(file, []byte("element \"Screen\" { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start on the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Screen").In(qa.ID("outline"))).Click()
	a.Settle()
	menu(a, "Frame")
	a.Settle()
	scene := string(a.State("scene"))
	if !strings.Contains(scene, `"Frame"`) {
		t.Fatalf("no frame on the canvas: %s", scene)
	}
	if n := h.scene()["Frame"]; n.Box[2] <= 0 || n.Box[3] <= 0 {
		t.Fatalf("the frame has no size: %+v", n)
	}
	if st := h.state(); !st.CanUndo || len(st.Nodes) != 1 || st.ItemSelected != "Frame" {
		t.Fatalf("state after Add > Frame: %+v", st)
	}
}

// addEntry picks an entry of the Add menu of the Items pane: the menu bar carries items of the same
// titles, so the click is aimed inside the menu of the button.
func addEntry(t *testing.T, a *qatest.Client, label string) {
	t.Helper()
	a.Find(qa.Selector{Class: "MenuItem", Text: label}.In(qa.ID("items-add"))).Click()
	a.Settle()
}

// TestItemsAddClassic: the "+" of the Items pane on a classic diagram offers the classic items, not the
// scene kinds that silently did nothing there, and New Item adds a row.
func TestItemsAddClassic(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	h.open(fixture)
	a.Settle()
	if h.state().SceneMode {
		t.Fatal("the fixture is not a classic diagram")
	}
	// the document itself is on the canvas at open, and Add is off until an element is selected
	a.Find(qa.Text("Basic steps").In(qa.ID("outline"))).Click()
	a.Settle()
	before := len(h.state().Items)
	a.Find(qa.ID("items-add")).Click()
	a.Settle()
	addEntry(t, a, "New Item")
	if got := len(h.state().Items); got != before+1 {
		t.Fatalf("New Item from the + of the Items pane: %d rows, want %d", got, before+1)
	}
}

// TestItemsAddUntitled: the "+" of the Items pane on a fresh window, the way a reader meets it, adds a
// row every time; the same window the report came from.
func TestItemsAddUntitled(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	menu(a, "New")
	a.Settle()
	a.State("command", "name", "close-others")
	a.Settle()
	if h.state().SceneMode {
		t.Fatal("File > New opened a scene, not a classic diagram")
	}
	a.Find(qa.Text("Unnamed diagram").In(qa.ID("outline"))).Click()
	a.Settle()
	for i := 1; i <= 2; i++ {
		a.Find(qa.ID("items-add")).Click()
		a.Settle()
		addEntry(t, a, "New Item")
		if got := len(h.state().Items); got != i {
			t.Fatalf("after %d clicks of New Item the list has %d rows: %v", i, got, h.state().Items)
		}
	}
}

// TestItemsAddRowHover: the "+" that appears on a row on hover adds into that row on one click, without
// selecting the row first — the way a reader presses it.
func TestItemsAddRowHover(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := restDoc(t, h, "Hover.tenn", moveDoc)
	tree := a.Find(qa.ID("items"))
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{})
	restWait(a)
	row, ok := readRest(t, a).ItemRows["A"]
	if !ok {
		t.Fatalf("no row A")
	}
	tree.Pointer(qa.PointerEvent{Kind: "move", X: float64(row.Box[0] + 20), Y: float64(row.Box[1] + row.Box[3]/2)})
	restWait(a)
	s := readRest(t, a)
	if len(s.ItemsAdd) != 2 {
		t.Fatalf("no + on the row A: %+v", s.ItemsAdd)
	}
	tree.ClickAt(float64(s.ItemsAdd[0]), float64(s.ItemsAdd[1]))
	restWait(a)
	if got := h.text(); !strings.Contains(got, "item Frame") {
		t.Fatalf("the row + wrote nothing:\n%s", got)
	}
	if s := readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "A.Frame" {
		t.Fatalf("the row + added to %v, want A.Frame", s.Nodes)
	}
}

// TestItemsAddFrameDrags: a Frame added to the element gets a place of its own and a fill, so it shows
// on the canvas and drags; a bare `item Frame` landed at 0,0, drew nothing and could not be moved.
func TestItemsAddFrameDrags(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	text := "element Screen {\n\tauto-layout abs\n\titem A { pos 0 0; width 80; height 40; color #cccccc; item T { title A } }\n}\n"
	file := filepath.Join(t.TempDir(), "frame.tenn")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{})
	a.Settle()

	a.Find(qa.ID("items-add")).Click()
	a.Settle()
	addEntry(t, a, "Frame")
	a.Key("Esc") // the new node opens for renaming
	a.Settle()

	got := h.text()
	if !strings.Contains(got, "item Frame { pos ") {
		t.Fatalf("the frame has no place of its own:\n%s", got)
	}
	if strings.Contains(got, "item Frame { pos 0 0;") {
		t.Fatalf("the frame landed on the node already there:\n%s", got)
	}
	var box []float64
	for path, n := range h.scene() {
		if strings.HasSuffix(path, "Frame") {
			box = n.Box
		}
	}
	if len(box) != 4 || box[2] <= 0 || box[3] <= 0 {
		t.Fatalf("the frame is not drawn: %v", box)
	}
	// the drag starts on the node the URL, the outline or a click selects
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Frame"})
	a.Settle()
	x, y := box[0]+box[2]/2, box[1]+box[3]/2
	a.Find(qa.ID("canvas")).DragTo(x, y, x+40, y+20)
	restWait(a)
	if after := h.text(); after == got {
		t.Fatalf("the frame did not drag:\n%s", after)
	}
}

// TestItemsAddDocumentRoot: the canvas at open holds the document itself, which is a diagram of items, so
// Add stays on and adds into it — the root is where a classic report starts.
func TestItemsAddDocumentRoot(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	h.open(fixture)
	a.Settle()
	if h.state().SceneMode {
		t.Fatal("the fixture is not a classic diagram")
	}
	// the document at open: no element chosen, the root of the file on the canvas
	a.State("select", "path", fixture, "nodes", []string{})
	a.Settle()
	if b := a.Find(qa.ID("items-add")); !b.Node().Enabled {
		t.Fatalf("Add is off on the document itself: %s", a.State("state"))
	}
}
