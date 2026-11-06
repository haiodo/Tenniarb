//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// moveDoc is a scene of nodes in file order: A, B and Flow, the last with its own kid C.
const moveDoc = `element "Screen" {
  item A { pos 400 0; width 100; height 40; title A }
  item B { pos 0 0; width 100; height 40; title B }
  item Flow { auto-layout column; gap 4; width 200; item C { width 10; height 10 } }
}
`

const libraryScreenDoc = `element "Screen" {
  item Board { auto-layout column; width 200; height 200 }
}
`

const libraryKitDoc = `element "Parts" {
  item Card { width 20; height 20; item label { title Card } }
}
`

// restState is the QA state plus what the banner and the hover "+" show.
type restState struct {
	sceneState
	Banner    string `json:"banner"`
	ItemsAdd  []int  `json:"itemsAdd"`
	ItemCount int    `json:"itemCount"`
	Dirty     bool   `json:"dirty"`
}

func readRest(t *testing.T, a *qatest.Client) (s restState) {
	t.Helper()
	if err := json.Unmarshal(a.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// restDoc writes text to name in a fresh folder, opens it and closes the other windows; the test
// leaves the shared window on the fixture, as it found it.
func restDoc(t *testing.T, h qaHelpers, name, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		h.app.State("command", "name", "close-others")
	})
	h.open(file)
	h.app.State("command", "name", "close-others")
	return file
}

// wait settles the application after the delay a change needs to reach the canvas.
func restWait(a *qatest.Client) { time.Sleep(300 * time.Millisecond); a.Settle() }

// order reports whether a comes before b in text.
func order(text, a, b string) bool {
	i, j := strings.Index(text, a), strings.Index(text, b)
	return i >= 0 && j >= 0 && i < j
}

// TestItemsAdd drives the "+" of the Items pane: the header one offers the Add menu, the one that
// appears on a row on hover adds into that row, not beside it.
func TestItemsAdd(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := restDoc(t, h, "Items.tenn", moveDoc)
	tree := a.Find(qa.ID("items"))
	before := h.text()

	// The header "+" is square at the ControlHeight hint, like a toolbar icon button.
	if b := a.Find(qa.ID("items-add")).Node().Bounds; b[2] != b[3] || b[2] > g.Spacing.ControlHeight+2 {
		t.Fatalf("items-add bounds %v, want a square within ControlHeight %d", b, g.Spacing.ControlHeight)
	}

	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{})
	restWait(a)

	// the header "+": the Add menu, a new node at the end of the element
	a.Find(qa.ID("items-add")).Click()
	a.Find(qa.Text("Frame").In(qa.ID("items-add"))).Click()
	restWait(a)
	a.Key("Esc") // the new node opens for renaming
	restWait(a)
	if got := h.text(); !strings.Contains(got, "item Frame") {
		t.Fatalf("the header +:\n%s", got)
	}
	if s := readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "Frame" {
		t.Fatalf("the header + selected %v", s.Nodes)
	}
	a.ShotTitle("items-1-header", h.title(file))
	a.Find(qa.ID("canvas")).Focus()
	a.Key("Mod+Z")
	restWait(a)
	if got := h.text(); got != before {
		t.Fatalf("the header + took more than one undo step:\n%s", got)
	}

	// the row "+" on hover: adds into the row under the pointer
	s := readRest(t, a)
	row, ok := s.ItemRows["A"]
	if !ok {
		t.Fatalf("no row A among %v", s.ItemRows)
	}
	// A click on a row selects it; the synthetic hover below moves only the pointer, so select the row
	// the way a click does before pressing its "+".
	a.Find(qa.Text("A").In(qa.ID("items"))).Click()
	restWait(a)
	tree.Pointer(qa.PointerEvent{Kind: "move", X: float64(row.Box[0] + 20), Y: float64(row.Box[1] + row.Box[3]/2)})
	restWait(a)
	if s = readRest(t, a); len(s.ItemsAdd) != 2 {
		t.Fatalf("no + on the row A: %+v", s)
	}
	tree.ClickAt(float64(s.ItemsAdd[0]), float64(s.ItemsAdd[1]))
	restWait(a)
	a.Key("Esc") // the new node opens for renaming
	restWait(a)
	if s = readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "A.Frame" {
		t.Fatalf("the row + added to %v, want A.Frame", s.Nodes)
	}
	if got := h.text(); !strings.Contains(got, "item Frame") {
		t.Fatalf("the row + wrote nothing:\n%s", got)
	}
	a.ShotOf("items-2-row", qa.ID("items")) // by control: the title lags the state on Linux
	a.Find(qa.ID("canvas")).Focus()
	a.Key("Mod+Z")
	restWait(a)
	if got := h.text(); got != before {
		t.Fatalf("the row + took more than one undo step:\n%s", got)
	}
}

// TestSceneMoveNodes drives Cmd+Alt+arrows and the menu: a node changes places with its siblings,
// or becomes a kid of the one before or after it; pos goes in a flow, stays in an absolute parent.
func TestSceneMoveNodes(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := restDoc(t, h, "Move.tenn", moveDoc)
	canvas := a.Find(qa.ID("canvas"))
	before := h.text()

	selectNode := func(path string) {
		a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{path})
		restWait(a)
	}
	key := func(k string) {
		canvas.Focus()
		a.Key(k)
		restWait(a)
	}
	text := func() string { return h.text() }

	// up: B before A, one undo step, the selection follows the node
	selectNode("B")
	key("Mod+Alt+Up")
	if got := text(); !order(got, "item B {", "item A {") {
		t.Fatalf("Move Up:\n%s", got)
	}
	if s := readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "B" {
		t.Fatalf("Move Up selected %v", s.Nodes)
	}
	a.ShotTitle("move-1-up", h.title(file))
	key("Mod+Z")
	if got := text(); got != before {
		t.Fatalf("Move Up took more than one undo step:\n%s", got)
	}

	// down: B after Flow
	selectNode("B")
	key("Mod+Alt+Down")
	if got := text(); !order(got, "item Flow {", "item B {") {
		t.Fatalf("Move Down:\n%s", got)
	}
	key("Mod+Z")
	if got := text(); got != before {
		t.Fatalf("Move Down took more than one undo step:\n%s", got)
	}

	// right: B becomes the first kid of Flow, and its pos goes: Flow lays out in a column
	selectNode("B")
	key("Mod+Alt+Right")
	got := text()
	if !strings.Contains(got, "item B { width 100; height 40; title B }") {
		t.Fatalf("the pos of B stayed in a flow layout:\n%s", got)
	}
	if !order(got, "item B {", "item C {") {
		t.Fatalf("B is not the first kid of Flow:\n%s", got)
	}
	if s := readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "Flow.B" {
		t.Fatalf("Move Right selected %v", s.Nodes)
	}
	a.ShotTitle("move-2-into", h.title(file))
	key("Mod+Z")
	if got := text(); got != before {
		t.Fatalf("Move Right took more than one undo step:\n%s", got)
	}

	// right into an absolute item keeps the pos
	selectNode("A")
	key("Mod+Alt+Right")
	if got := text(); !strings.Contains(got, "pos 400 0") {
		t.Fatalf("the pos of A went with an absolute parent:\n%s", got)
	}
	if s := readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "B.A" {
		t.Fatalf("Move Right into B selected %v", s.Nodes)
	}
	key("Mod+Z")
	if got := text(); got != before {
		t.Fatalf("Move Right into B took more than one undo step:\n%s", got)
	}

	// left: B becomes the last kid of A, pos kept
	selectNode("B")
	key("Mod+Alt+Left")
	if got := text(); !strings.Contains(got, "pos 0 0") {
		t.Fatalf("the pos of B went with an absolute parent:\n%s", got)
	}
	if s := readRest(t, a); len(s.Nodes) != 1 || s.Nodes[0] != "A.B" {
		t.Fatalf("Move Left selected %v", s.Nodes)
	}
	key("Mod+Z")
	if got := text(); got != before {
		t.Fatalf("Move Left took more than one undo step:\n%s", got)
	}

	// Element > Move Down, and the same on the row menu of the Items pane
	selectNode("A")
	menu(a, "Move Down")
	restWait(a)
	if got := text(); !order(got, "item B {", "item A {") {
		t.Fatalf("Element > Move Down:\n%s", got)
	}
	key("Mod+Z")
	selectNode("A")
	row := readRest(t, a).ItemRows["A"]
	tree := a.Find(qa.ID("items"))
	tree.RightClickAt(float64(row.Box[0]+20), float64(row.Box[1]+row.Box[3]/2))
	restWait(a)
	a.Find(qa.Text("Move Down").In(qa.ID("items"))).Click()
	restWait(a)
	if got := text(); !order(got, "item B {", "item A {") {
		t.Fatalf("row menu Move Down:\n%s", got)
	}
	key("Mod+Z")
	if got := text(); got != before {
		t.Fatalf("the menu moves left a change:\n%s", got)
	}
}

// TestAddLibrary drives Element > Add Library...: the file is mounted as an element of the document,
// Insert Component offers what it holds, and a file outside the workspace root is refused with a
// clear message.
func TestAddLibrary(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir := t.TempDir()
	file := filepath.Join(dir, "App.tenn")
	lib := filepath.Join(dir, "lib", "Kit.tenn")
	if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(libraryScreenDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lib, []byte(libraryKitDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "Other.tenn")
	if err := os.WriteFile(outside, []byte(libraryKitDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	before := h.text()

	// a file outside the workspace root: a clear message, no line written
	a.Answer(qa.Dialog{Kind: "file", Reply: outside})
	menu(a, "Add Library...")
	restWait(a)
	if s := readRest(t, a); !strings.Contains(s.Banner, "outside the workspace root") {
		t.Fatalf("a library outside the root: %q", s.Banner)
	}
	if got := h.text(); got != before {
		t.Fatalf("the refused library changed the text:\n%s", got)
	}

	// the library in the folder of the document: one element line, relative to the document
	a.Answer(qa.Dialog{Kind: "file", Reply: lib})
	menu(a, "Add Library...")
	restWait(a)
	got := h.text()
	if !strings.Contains(got, `element "Kit" --from-file "lib/Kit.tenn"`) {
		t.Fatalf("Add Library wrote:\n%s", got)
	}
	if s := readRest(t, a); s.Banner != "" {
		t.Fatalf("Add Library: %q", s.Banner)
	}
	a.ShotTitle("library-1-added", h.title(file))
	a.Find(qa.ID("canvas")).Focus()
	a.Key("Mod+Z")
	restWait(a)
	if got := h.text(); got != before {
		t.Fatalf("Add Library took more than one undo step:\n%s", got)
	}
	a.Key("Mod+Shift+Z")
	restWait(a)

	// Insert Component sees the components of the mounted file
	a.Find(menuBarItem(h.activeTitle(), "Element", "Insert Component...")).Click()
	restWait(a)
	h.enterIn("component-filter", "Card")
	restWait(a)
	if got := h.text(); !strings.Contains(got, "use Kit.Parts.Card as Card") {
		t.Fatalf("Insert Component from the added library:\n%s", got)
	}
	a.ShotTitle("library-2-inserted", h.title(file))
}
