//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

type inspState struct {
	Selection []string
	Screen    map[string][]float64
	Props     string
	Sidebar   bool
	Inspector bool
	Dirty     bool
	Outline   []string
}

func insp(t *testing.T, app *qatest.Client) inspState {
	t.Helper()
	var s inspState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestInspector drives the inspector with real input: edits through its fields reach the canvas, the
// saved file and the undo history.
func TestInspector(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "inspector")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../web/fixtures/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "Example.tenn")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	read := func() string { b, _ := os.ReadFile(file); return string(b) }
	field := func(id string) *qatest.Element { return app.Find(qa.ID(id)) }
	text := func(id string) string { return field(id).Node().Value }
	visible := func(id string) bool { return field(id).Exists() && field(id).Node().Visible }
	enter := func(id, v string) {
		field(id).Type(v)
		field(id).Key("Enter")
		time.Sleep(300 * time.Millisecond)
		app.Settle()
	}
	canvas := app.Find(qa.ID("canvas"))

	app.Answer(qa.Dialog{Kind: "message", Reply: "No"})
	app.Answer(qa.Dialog{Kind: "file", Reply: file})
	menu(app, "Open...")
	app.State("command", "name", "close-others")
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Click()
	app.Settle()
	time.Sleep(300 * time.Millisecond)
	app.Settle()

	// nothing selected: the element
	if text("insp-name") != "Basic steps" || !visible("insp-def-color") || visible("insp-color") {
		t.Fatalf("element sections: name %q", text("insp-name"))
	}
	app.Shot("insp-1-element-light")

	// one item: its properties; a field edit changes the item
	s := insp(t, app)
	x, y := center(s.Screen["Item"])
	canvas.ClickAt(x, y)
	app.Settle()
	if text("insp-name") != "Item" || !visible("insp-color") || visible("insp-def-color") {
		t.Fatalf("item sections: name %q", text("insp-name"))
	}
	if got := text("insp-color"); got != "green" {
		t.Fatalf("color field %q", got)
	}
	app.Shot("insp-2-item-light")
	enter("insp-color", "red")
	enter("insp-font-size", "30")
	if p := insp(t, app).Props; !strings.Contains(p, "color red") || !strings.Contains(p, "font-size 30") {
		t.Fatalf("props after the edit: %q", p)
	}
	app.Key("Mod+S")
	if got := read(); !strings.Contains(got, "font-size 30") {
		t.Fatalf("saved file lacks the edit")
	}
	app.Shot("insp-3-edited-light")
	// undo: the canvas takes the focus, the menu command works again
	canvas.Focus()
	app.Key("Mod+Z")
	app.Settle()
	if p := insp(t, app).Props; strings.Contains(p, "font-size 30") || !strings.Contains(p, "color red") {
		t.Fatalf("undo of the font size: %q", p)
	}
	app.Key("Mod+Z")
	app.Settle()
	if got := text("insp-color"); got != "green" {
		t.Fatalf("color after undo %q", got)
	}

	// several items: mixed values show as such, an edit applies to all
	canvas.Focus()
	app.Key("Mod+Shift+A")
	app.Settle()
	if n := len(insp(t, app).Selection); n < 3 {
		t.Fatalf("select all items: %d", n)
	}
	if visible("insp-name") {
		t.Fatal("the name field must be hidden for several items")
	}
	if got := field("insp-color").Node(); got.Value != "" {
		t.Fatalf("mixed colors show %q", got.Value)
	}
	app.Shot("insp-4-multi-light")
	enter("insp-font-size", "14")
	app.Key("Mod+S")
	if n := strings.Count(read(), "font-size 14"); n < 3 {
		t.Fatalf("font-size 14 in %d items", n)
	}
	canvas.Focus()
	app.Key("Mod+Z")
	app.Settle()

	// the body of an item, as text of its own
	canvas.ClickAt(5, 5)
	canvas.ClickAt(x, y)
	app.Settle()
	field("insp-body").Type("hello body")
	time.Sleep(900 * time.Millisecond)
	app.Settle()
	app.Key("Mod+S")
	if got := read(); !strings.Contains(got, "hello body") {
		t.Fatalf("saved file lacks the body")
	}
	app.Shot("insp-5-body-light")
	canvas.Focus()
	app.Key("Mod+Z")
	app.Settle()
	app.Key("Mod+S")
	if strings.Contains(read(), "hello body") {
		t.Fatal("undo did not take the body back")
	}

	// the element's own defaults, edited with nothing selected
	canvas.ClickAt(5, 5)
	app.Settle()
	if !visible("insp-def-color") {
		t.Fatal("element section missing after the selection was cleared")
	}
	enter("insp-def-border", "blue")
	app.Key("Mod+S")
	if got := read(); !strings.Contains(got, "border-color blue") {
		t.Fatalf("saved file lacks the element default:\n%s", got)
	}

	// Goto Item through the toolbar search, sidebar and inspector toggles
	field("goto-field").Type("'Ctrl'")
	app.Settle()
	if sel := insp(t, app).Selection; len(sel) != 1 || sel[0] != "hold 'Ctrl'" {
		t.Fatalf("toolbar search selected %v", sel)
	}
	app.State("command", "name", "toolbar", "item", "sidebar")
	app.Settle()
	if s = insp(t, app); s.Sidebar {
		t.Fatal("sidebar still shown")
	}
	app.Shot("insp-6-no-sidebar-light")
	app.State("command", "name", "toolbar", "item", "sidebar")
	app.State("command", "name", "toolbar", "item", "inspector")
	app.Settle()
	if s = insp(t, app); !s.Sidebar || s.Inspector {
		t.Fatalf("toggles: %+v", s)
	}
	app.State("command", "name", "toolbar", "item", "inspector")
	app.Settle()

	// a link
	canvas.ClickAt(5, 5)
	field("goto-field").Type("Ctrl")
	app.Settle()

	app.State("command", "name", "appearance", "mode", "dark")
	time.Sleep(500 * time.Millisecond)
	app.Settle()
	app.Shot("insp-7-item-dark")
	canvas.ClickAt(5, 5)
	app.Settle()
	app.Shot("insp-8-element-dark")
	app.State("command", "name", "appearance", "mode", "light")
	app.Settle()
}
