//go:build darwin || linux || windows

package app_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// menuTitles lists the titles of the menu items of the dump, submenus included.
func menuTitles(nodes []qa.Node) []string {
	var out []string
	var walk func(ns []qa.Node)
	walk = func(ns []qa.Node) {
		for _, n := range ns {
			if n.Class == "MenuItem" {
				out = append(out, n.Text)
			}
			walk(n.Children)
		}
	}
	walk(nodes)
	return out
}

// openCopy opens a copy of Example.tenn on the element Basic steps and closes the other windows.
func openCopy(t *testing.T, app *qatest.Client) string {
	t.Helper()
	return openText(t, app, "", "Basic steps")
}

// openText opens a file with text (the copy of Example.tenn when empty) and selects the outline row row.
func openText(t *testing.T, app *qatest.Client, text, row string) string {
	t.Helper()
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "parity")
	if err != nil {
		t.Fatal(err)
	}
	data := mustRead(t, "../../web/fixtures/Example.tenn")
	file := filepath.Join(dir, "Example.tenn")
	if text != "" {
		data = []byte(text)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	app.Answer(qa.Dialog{Kind: "message", Reply: "No"})
	app.Answer(qa.Dialog{Kind: "file", Reply: file})
	menu(app, "Open...")
	app.State("command", "name", "close-others")
	app.Find(qa.Text(row).In(qa.Class("Tree"))).Click()
	app.Settle()
	// the later tests start from the window that shows the fixture
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		orig := filepath.Join(filepath.Dir(file), "Restore.tenn")
		if err := os.WriteFile(orig, mustRead(t, "../../web/fixtures/Example.tenn"), 0o644); err != nil {
			t.Error(err)
			return
		}
		app.Answer(qa.Dialog{Kind: "message", Reply: "No"})
		app.Answer(qa.Dialog{Kind: "file", Reply: orig})
		menu(app, "Open...")
		app.State("command", "name", "close-others")
	})
	return file
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParityMenus(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	openCopy(t, app)
	have := menuTitles(app.Dump())
	t.Logf("menus: %q", have)
	for _, want := range []string{"New", "Open...", "Open Recent", "Save", "Save As...", "Reload", "Close Window", "Undo", "Redo", "Cut", "Copy", "Paste",
		"Delete", "Duplicate", "Select All", "Select All Items", "Select All Links", "Select None", "Paste as Item", "Paste as Item Set", "Inherit",
		"Show Edit Box", "Edit Title", "Edit Body", "Edit Value", "New Item", "New Linked Item", "Linked Styled Item",
		"Goto Item...", "Zoom In", "Zoom Out", "Reset Zoom", "Zoom to Fit", "Tenniarb Help"} {
		if !slices.Contains(have, want) {
			t.Errorf("no menu item %q", want)
		}
	}
}

type parityState struct {
	Path      string
	Dirty     bool
	Inline    bool
	Popover   bool
	Selection []string
	Items     map[string][]float64
	Screen    map[string][]float64
	Links     []string
	ItemCount int
	Props     string
	Zoom      float64
	OriginX   float64
	OriginY   float64
	Outline   []string
	Element   []string
	Canvas    []int
}

func par(t *testing.T, app *qatest.Client) parityState {
	t.Helper()
	var s parityState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

const parityDoc = `element "Parity" {
    item "A" {
        pos -200.0 0.0
        color green
    }
    item "B" {
        pos 0.0 0.0
    }
    item "C" {
        pos 200.0 0.0
    }
    item "D" {
        pos 0.0 -150.0
    }
    link "A" "B" {
        display arrow
    }
    link "B" "C" {
        display arrow
    }
}
`

// TestParityCanvas drives the keyboard, mouse and menu gestures of the canvas that the editor tests do not.
func TestParityCanvas(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	openText(t, app, parityDoc, "Parity")
	canvas := app.Find(qa.ID("canvas"))
	inline := qa.Class("Text").In(qa.ID("canvas"))
	var s parityState
	pick := func(name string) parityState {
		x, y := center(par(t, app).Screen[name])
		canvas.ClickAt(x, y)
		return par(t, app)
	}
	sel := func(want ...string) {
		t.Helper()
		got := slices.Clone(par(t, app).Selection)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("selection %q, want %q", got, want)
		}
	}

	// Select All, Select All Items / Links, Select None
	canvas.Focus()
	app.Key("Mod+A")
	sel("A", "B", "C", "D", "", "")
	menu(app, "Select None")
	sel()
	menu(app, "Select All Links")
	sel("", "")
	menu(app, "Select All Items")
	sel("A", "B", "C", "D")
	menu(app, "Select None")

	// Space opens the operation box over the selection; its text is applied
	pick("A")
	app.Key("Space")
	if !par(t, app).Popover {
		t.Fatal("Space did not open the operation box")
	}
	app.Find(qa.Class("Text").In(qa.Class("Popover"))).Type("color red")
	app.Key("Enter")
	if s = par(t, app); s.Popover || !strings.Contains(s.Props, "color red") {
		t.Fatalf("operation: popover=%v props %q", s.Popover, s.Props)
	}
	app.Key("Mod+Z")
	if s := par(t, app); !strings.Contains(s.Props, "color green") {
		t.Fatalf("undo of the operation: %q", s.Props)
	}

	// Shift+Enter edits the body, Opt+Enter the value; Esc leaves
	canvas.Focus()
	app.Key("Shift+Enter")
	if !par(t, app).Inline {
		t.Fatal("Shift+Enter did not open the body editor")
	}
	app.Find(inline).Type("line one")
	app.Key("Esc")
	canvas.Focus()
	app.Key("Alt+Enter")
	if !par(t, app).Inline {
		t.Fatal("Opt+Enter did not open the value editor")
	}
	app.Key("Esc")
	if s := par(t, app); s.Inline || s.Dirty && strings.Contains(s.Props, "line one") {
		t.Fatalf("Esc kept the text: %+v", s)
	}

	// arrows move by the grid step; Shift resizes from the edge, Cmd from the centre
	pick("B")
	b0 := par(t, app).Items["B"]
	app.Key("Right")
	if b1 := par(t, app).Items["B"]; b1[0] <= b0[0] || b1[1] != b0[1] {
		t.Fatalf("Right: %v -> %v", b0, b1)
	}
	app.Key("Left")
	app.Key("Shift+Right")
	if b1 := par(t, app).Items["B"]; b1[2] <= b0[2] || b1[0] != b0[0] {
		t.Fatalf("Shift+Right: %v -> %v", b0, b1)
	}
	app.Key("Shift+Left")
	app.Key("Mod+Right")
	if b1 := par(t, app).Items["B"]; b1[2] <= b0[2] || b1[0] >= b0[0] {
		t.Fatalf("Cmd+Right (from the centre): %v -> %v", b0, b1)
	}
	app.Key("Mod+Left")
	// the explicit width is the natural one minus the line inset, so it lands within a point of the start
	if b1 := par(t, app).Items["B"]; math.Abs(b1[2]-b0[2]) > 1 || math.Abs(b1[0]-b0[0]) > 1 {
		t.Fatalf("Cmd+Left: %v -> %v", b0, b1)
	}

	// x deletes with the links; Tab adds a linked item, Opt+Tab one with the style of the selected
	n0 := par(t, app).ItemCount
	pick("D")
	app.Key("x")
	if par(t, app).ItemCount != n0-1 {
		t.Fatalf("x: %d items, were %d", par(t, app).ItemCount, n0)
	}
	app.Key("Mod+Z")
	pick("A")
	app.Key("Alt+Tab")
	if s := par(t, app); s.ItemCount != n0+2 || !strings.Contains(s.Props, "color green") || len(s.Selection) != 1 || s.Selection[0] == "A" {
		t.Fatalf("Opt+Tab: %d items, selection %q props %q", s.ItemCount, s.Selection, s.Props)
	}
	app.Key("Mod+Z")
	pick("A")
	app.Key("Tab")
	if s := par(t, app); s.ItemCount != n0+2 || strings.Contains(s.Props, "color green") {
		t.Fatalf("Tab: %d items, props %q", s.ItemCount, s.Props)
	}
	app.Key("Mod+Z")

	// Shift+drag selects by a band
	s = par(t, app)
	a, c := s.Screen["A"], s.Screen["C"]
	canvas.Pointer(qa.PointerEvent{Kind: "drag", X: a[0] - 15, Y: a[1] - 25, ToX: c[0] + c[2] + 15, ToY: c[1] + c[3] + 25, Mods: "Shift"})
	sel("A", "B", "C", "", "")

	// Cmd+click toggles
	menu(app, "Select None")
	pick("A")
	bx, by := center(par(t, app).Screen["B"])
	canvas.Pointer(qa.PointerEvent{Kind: "click", X: bx, Y: by, Mods: "Mod"})
	sel("A", "B")
	canvas.Pointer(qa.PointerEvent{Kind: "click", X: bx, Y: by, Mods: "Mod"})
	sel("A")

	// Opt+drag moves the item with everything linked from it
	ax, ay := center(par(t, app).Screen["A"])
	before := par(t, app).Items
	canvas.Pointer(qa.PointerEvent{Kind: "drag", X: ax, Y: ay, ToX: ax, ToY: ay + 40*par(t, app).Zoom, Mods: "Alt"})
	after := par(t, app).Items
	for _, n := range []string{"A", "B", "C"} {
		if !approx(after[n][1], before[n][1]-40) {
			t.Errorf("Opt+drag: %s moved from %v to %v", n, before[n], after[n])
		}
	}
	if after["D"][1] != before["D"][1] {
		t.Errorf("Opt+drag moved D: %v", after["D"])
	}
	app.Key("Mod+Z")

	// the middle button pans, the empty place under the left button too
	z := par(t, app)
	canvas.Pointer(qa.PointerEvent{Kind: "drag", Button: 2, X: 5, Y: 5, ToX: 45, ToY: 25})
	if m := par(t, app); m.OriginX == z.OriginX && m.OriginY == z.OriginY {
		t.Fatal("the middle button did not pan")
	}
	z = par(t, app)
	canvas.DragTo(5, 5, 55, 35)
	if m := par(t, app); m.OriginX == z.OriginX && m.OriginY == z.OriginY {
		t.Fatal("the left button on empty space did not pan")
	}

	// View menu: zoom in and out, Reset Zoom (Cmd+0) and Zoom to Fit
	z = par(t, app)
	menu(app, "Zoom In")
	if m := par(t, app); m.Zoom <= z.Zoom {
		t.Fatalf("Zoom In: %v -> %v", z.Zoom, m.Zoom)
	}
	menu(app, "Zoom Out")
	if m := par(t, app); !approx(m.Zoom, z.Zoom) {
		t.Fatalf("Zoom Out: %v -> %v", z.Zoom, m.Zoom)
	}
	canvas.Focus()
	app.Key("Mod+0")
	if m := par(t, app); m.Zoom != 1 {
		t.Fatalf("Cmd+0: zoom %v", m.Zoom)
	}
	menu(app, "Zoom In")
	menu(app, "Zoom In")
	zoomed := par(t, app).Zoom
	app.Key("Mod+9")
	if m := par(t, app); m.Zoom >= zoomed {
		t.Fatalf("Cmd+9 (fit): zoom %v", m.Zoom)
	}
	// Cmd+scroll zooms about the pointer
	canvas.Focus()
	app.Key("Mod+0")
	z = par(t, app)
	canvas.ScrollEvent(qa.ScrollEvent{X: 300, Y: 300, DY: 40, Precise: true, Mods: "Mod"})
	if m := par(t, app); m.Zoom == z.Zoom {
		t.Fatalf("Cmd+scroll did not zoom: %v", m.Zoom)
	}

}

// pngFile writes a 2x2 picture for Attach image.
func pngFile(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	path := filepath.Join(t.TempDir(), "pic.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

// withClipboard runs f with the system clipboard text saved and restored (macOS only; the QA driver has
// no clipboard of its own, so the text of the user's clipboard is put back, other formats are lost).
func withClipboard(t *testing.T, f func(set func(string), get func() string)) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the clipboard is read with pbcopy and pbpaste")
	}
	old, _ := exec.Command("pbpaste").Output()
	defer func() {
		c := exec.Command("pbcopy")
		c.Stdin = bytes.NewReader(old)
		_ = c.Run()
	}()
	set := func(s string) {
		c := exec.Command("pbcopy")
		c.Stdin = strings.NewReader(s)
		if err := c.Run(); err != nil {
			t.Fatal(err)
		}
	}
	get := func() string { b, _ := exec.Command("pbpaste").Output(); return string(b) }
	f(set, get)
}

func TestParityContext(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	openText(t, app, parityDoc, "Parity")
	canvas := app.Find(qa.ID("canvas"))
	rightClick := func(name string) {
		x, y := center(par(t, app).Screen[name])
		canvas.RightClickAt(x, y)
	}
	entry := func(text string) { app.Find(qa.Text(text).In(qa.ID("canvas"))).Click() }

	// a quick style from the menu of one item
	rightClick("A")
	entry("15")
	if s := par(t, app); !strings.Contains(s.Props, "corner-radius 15") || len(s.Selection) != 1 {
		t.Fatalf("Quick Style > Corner Radius: selection %q props %q", s.Selection, s.Props)
	}
	app.Key("Mod+Z")

	// Align on a band selection
	s := par(t, app)
	a, c := s.Screen["A"], s.Screen["C"]
	canvas.Pointer(qa.PointerEvent{Kind: "drag", X: a[0] - 15, Y: a[1] - 25, ToX: c[0] + c[2] + 15, ToY: c[1] + c[3] + 25, Mods: "Shift"})
	rightClick("A")
	entry("Leading Edges")
	s = par(t, app)
	if !approx(s.Items["A"][0], s.Items["B"][0]) || !approx(s.Items["B"][0], s.Items["C"][0]) {
		t.Fatalf("Align > Leading Edges: %v %v %v", s.Items["A"], s.Items["B"], s.Items["C"])
	}
	app.Key("Mod+Z")

	// Order, Define new style, Attach image and Delete on one item
	canvas.ClickAt(5, 5)
	rightClick("D")
	entry("Move Forward")
	if s = par(t, app); !s.Dirty {
		t.Fatal("Order > Move Forward changed nothing")
	}
	app.Key("Mod+Z")
	rightClick("D")
	app.Answer(qa.Dialog{Kind: "file", Reply: pngFile(t)})
	entry("Attach image")
	if s = par(t, app); !strings.Contains(s.Props, "image") {
		t.Fatalf("Attach image: props %q", s.Props)
	}
	app.Key("Mod+Z")
	withClipboard(t, func(set func(string), get func() string) {
		rightClick("D")
		entry("Export text as html")
		if got := get(); !strings.Contains(got, "<div") {
			t.Fatalf("Export text as html: clipboard %q", got)
		}
	})
	rightClick("D")
	n := par(t, app).ItemCount
	entry("Delete")
	if s = par(t, app); s.ItemCount != n-1 {
		t.Fatalf("Delete: %d items, were %d", s.ItemCount, n)
	}
	app.Key("Mod+Z")

	// the empty place: New item at the click, Test layout, Enable shadows
	canvas.RightClickAt(5, 5)
	entry("New item")
	if s = par(t, app); s.ItemCount != n+1 {
		t.Fatalf("New item: %d items, were %d", s.ItemCount, n)
	}
	app.Key("Mod+Z")
	before := par(t, app).Items
	canvas.RightClickAt(5, 5)
	entry("Test layout")
	if after := par(t, app).Items; reflect.DeepEqual(before, after) {
		t.Fatal("Test layout moved nothing")
	}
	app.Key("Mod+Z")
	canvas.RightClickAt(5, 5)
	entry("Enable shadows")
	if s = par(t, app); !s.Dirty || !strings.Contains(s.Props, "shadow") {
		t.Fatalf("Enable shadows: dirty=%v props %q", s.Dirty, s.Props)
	}
	app.Key("Mod+Z")
	canvas.RightClickAt(5, 5)
	entry("Define new style")
	if s = par(t, app); !s.Dirty {
		t.Fatal("Define new style changed nothing")
	}
}

func TestParityDocument(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	file := openText(t, app, parityDoc, "Parity")
	canvas := app.Find(qa.ID("canvas"))
	tree := app.Find(qa.Class("Tree"))
	pick := func(name string) {
		x, y := center(par(t, app).Screen[name])
		canvas.ClickAt(x, y)
	}
	var s parityState

	// Edit > Inherit: a copy of the element inside it
	rows := len(par(t, app).Outline)
	menu(app, "Inherit")
	if s = par(t, app); len(s.Outline) != rows+1 {
		t.Fatalf("Inherit: outline %q, had %d rows", s.Outline, rows)
	}
	app.Key("Mod+Z")

	// the outline: Tab adds an item to the canvas and gives the focus back to it
	app.Find(qa.Text("Parity").In(qa.Class("Tree"))).Click()
	n := par(t, app).ItemCount
	tree.Focus()
	app.Key("Tab")
	if s = par(t, app); s.ItemCount != n+1 {
		t.Fatalf("Tab in the outline: %d items, were %d", s.ItemCount, n)
	}
	app.Key("Mod+Z")

	// Element menu: New Element, Move Up/Down, Duplicate Element, Delete Element, each one undo step
	rows = len(par(t, app).Outline)
	menu(app, "New Element")
	if s = par(t, app); len(s.Outline) != rows+1 {
		t.Fatalf("New Element: %q", s.Outline)
	}
	menu(app, "Duplicate Element")
	if s = par(t, app); len(s.Outline) != rows+2 {
		t.Fatalf("Duplicate Element: %q", s.Outline)
	}
	menu(app, "Delete Element")
	if s = par(t, app); len(s.Outline) != rows+1 {
		t.Fatalf("Delete Element: %q", s.Outline)
	}
	app.Key("Mod+Z")
	app.Key("Mod+Z")
	app.Key("Mod+Z")
	if s = par(t, app); len(s.Outline) != rows {
		t.Fatalf("undo of the element commands: %q", s.Outline)
	}

	// Reload (Revert to Saved) drops the edits after the question
	canvas.Focus()
	pick("A")
	app.Key("Right")
	if !par(t, app).Dirty {
		t.Fatal("Right did not edit")
	}
	app.Answer(qa.Dialog{Kind: "confirm", Reply: true})
	menu(app, "Reload")
	if s = par(t, app); s.Dirty {
		t.Fatal("Reload kept the edit")
	}
	if got, _ := os.ReadFile(file); !strings.Contains(string(got), "pos -200") {
		t.Fatalf("the file changed: %s", got)
	}

	// clipboard: Cmd+C and Cmd+V on items, Cmd+X, Paste as Item, Paste as Item Set, element copy and paste
	withClipboard(t, func(set func(string), get func() string) {
		canvas.Focus()
		pick("B")
		n := par(t, app).ItemCount
		app.Key("Mod+C")
		if got := get(); !strings.Contains(got, `item "B"`) {
			t.Fatalf("Cmd+C put %q on the clipboard", got)
		}
		app.Key("Mod+V")
		if s = par(t, app); s.ItemCount != n+1 {
			t.Fatalf("Cmd+V: %d items, were %d", s.ItemCount, n)
		}
		app.Key("Mod+X")
		if s = par(t, app); s.ItemCount != n {
			t.Fatalf("Cmd+X: %d items, were %d", s.ItemCount, n)
		}
		set("plain words")
		menu(app, "Paste as Item")
		if s = par(t, app); s.ItemCount != n+1 {
			t.Fatalf("Paste as Item: %d items, were %d", s.ItemCount, n)
		}
		set("one\ntwo\nthree")
		menu(app, "Paste as Item Set")
		if s = par(t, app); s.ItemCount != n+1+3+2 {
			t.Fatalf("Paste as Item Set: %d items, were %d", s.ItemCount, n+1)
		}
		app.Find(qa.Text("Parity").In(qa.Class("Tree"))).Click()
		tree.Focus()
		app.Key("Mod+C")
		if got := get(); !strings.Contains(got, `element "Parity"`) {
			t.Fatalf("Cmd+C in the outline put %q on the clipboard", got)
		}
		rows := len(par(t, app).Outline)
		app.Key("Mod+V")
		if s = par(t, app); len(s.Outline) != rows+1 {
			t.Fatalf("Cmd+V in the outline: %q", s.Outline)
		}
	})
}

// pixelOf is the colour of a pixel of a PNG file, at the fractions of its width and height.
func pixelOf(t *testing.T, path string, fx, fy float64) (r, g, b uint32) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Size()
	r, g, b, _ = img.At(int(fx*float64(size.X)), int(fy*float64(size.Y))).RGBA()
	return r >> 8, g >> 8, b >> 8
}

func TestParityPrefs(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	file := openText(t, app, parityDoc, "Parity")
	canvas := app.Find(qa.ID("canvas"))
	app.State("command", "name", "preferences")

	// the canvas takes the colour of the settings
	app.Find(qa.ID("background")).Type("#102030")
	app.Settle()
	path := app.ShotTitle("parity-bg", "Example.tenn")
	if r, g, b := pixelOf(t, path, 0.4, 0.3); r > 0x20 || g < 0x18 || g > 0x28 || b < 0x28 || b > 0x38 {
		t.Errorf("the canvas corner is %02x%02x%02x, the setting is 102030", r, g, b)
	}
	app.Find(qa.ID("background-reset")).Click()

	// the quick panel opens after a click on an item and closes with Esc
	app.Find(qa.ID("quick-panel")).Check(true)
	canvas.Focus()
	x, y := center(par(t, app).Screen["A"])
	canvas.ClickAt(x, y)
	if !par(t, app).Popover {
		t.Error("the quick panel did not open after the click")
	}
	app.Key("Esc")
	app.Find(qa.ID("quick-panel")).Check(false)

	// Expand levels: with auto expand off a document opens collapsed
	app.Find(qa.ID("auto-expand")).Check(false)
	nested := "element \"Top\" {\n    element \"Inner\" {\n        item \"X\" {\n            pos 0.0 0.0\n        }\n    }\n}\n"
	other := filepath.Join(filepath.Dir(file), "Nested.tenn")
	if err := os.WriteFile(other, []byte(nested), 0o644); err != nil {
		t.Fatal(err)
	}
	app.Answer(qa.Dialog{Kind: "file", Reply: other})
	app.Find(inWindow(qa.Text("Open..."), "Example.tenn")).Click()
	app.State("command", "name", "close-others")
	app.Settle()
	if app.Find(qa.Text("Inner").In(qa.Class("Tree"))).Node().Visible {
		t.Error("auto expand off: the nested row is open")
	}
	app.Find(qa.ID("auto-expand")).Check(true)
	app.Find(qa.Selector{Class: "Shell", Text: "Preferences"}).Close()
}
