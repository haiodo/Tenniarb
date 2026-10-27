//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"

	"github.com/haiodo/tenniarb/go/model"
)

var fixture, cfgDir, serviceDir string

// TestMain builds the app with the QA server and starts it on a copy of Example.tenn. Windows open
// only with GOWT_GUI_TEST=1.
func TestMain(m *testing.M) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "tenniarbqa")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	data, err := os.ReadFile("../../web/fixtures/Example.tenn")
	if err != nil {
		panic(err)
	}
	fixture = filepath.Join(dir, "Example.tenn")
	if err := os.WriteFile(fixture, data, 0o644); err != nil {
		panic(err)
	}
	// The application reads its settings here, not from the user's config directory. The quick panel
	// would open after every click on an item, so the editor tests start with it off.
	cfg := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(`{"quickPanel": false}`), 0o644); err != nil {
		panic(err)
	}
	os.Setenv("TENNIARB_CONFIG_DIR", cfg)
	cfgDir = cfg
	// The app connects to tenniarb-service here, never to the user's; the socket path must stay short.
	serviceDir = filepath.Join(dir, "svc")
	if err := os.Mkdir(serviceDir, 0o700); err != nil {
		panic(err)
	}
	os.Setenv("TENNIARB_SERVICE_DIR", serviceDir)
	stub := filepath.Join(dir, "embed-stub.js")
	if err := os.WriteFile(stub, []byte("/*embed stub*/"), 0o644); err != nil {
		panic(err)
	}
	os.Setenv("TENNIARB_EMBED", stub)
	bin := filepath.Join(dir, "tenniarb")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-tags", "gowtqa", "-o", bin, "../cmd/tenniarb").CombinedOutput(); err != nil {
		println(string(out))
		os.Exit(1)
	}
	qatest.Main(m, bin, fixture)
}

type viewerState struct {
	Path    string
	Error   string
	Element []string
	Zoom    float64
	OriginX float64
	OriginY float64
	Canvas  []int
}

func state(t *testing.T, app *qatest.Client) viewerState {
	t.Helper()
	var s viewerState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestViewer(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	app.Settle()
	app.Shot("viewer-open")

	s := state(t, app)
	if s.Path != fixture || len(s.Element) != 0 {
		t.Fatalf("open state %+v", s)
	}

	app.Find(qa.Class("TreeItem").In(qa.Class("Tree"))).Nodes() // outline is populated
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Click()
	s = state(t, app)
	if len(s.Element) != 2 || s.Element[1] != "Basic steps" {
		t.Fatalf("after select %+v", s)
	}
	app.Shot("viewer-basic-steps")

	z := s.Zoom
	app.State("zoom", "factor", 2.0)
	app.State("pan", "dx", 40.0, "dy", 20.0)
	s2 := state(t, app)
	if s2.Zoom != z*2 {
		t.Fatalf("zoom %v -> %v", z, s2.Zoom)
	}
	app.Shot("viewer-zoomed")

	later := time.Now().Add(time.Hour)
	os.Chtimes(fixture, later, later)
	time.Sleep(1500 * time.Millisecond)
	app.Settle()
	s3 := state(t, app)
	if len(s3.Element) != 2 || s3.Zoom != s2.Zoom || s3.OriginX != s2.OriginX {
		t.Fatalf("reload lost selection or viewport: %+v vs %+v", s3, s2)
	}

	// a broken file shows the error and keeps the diagram
	if err := os.WriteFile(fixture, []byte("element \"A\" {"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	app.Settle()
	s4 := state(t, app)
	if s4.Error == "" || len(s4.Element) != 2 {
		t.Fatalf("parse error state %+v", s4)
	}
	app.Shot("viewer-parse-error")

	// File > Open with the scripted file dialog
	other := filepath.Join(filepath.Dir(fixture), "Other.tenn")
	if err := os.WriteFile(other, []byte("element \"Solo\" {\n item \"One\" { pos 0 0 }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.Answer(qa.Dialog{Kind: "file", Reply: other})
	menu(app, "Open...")
	app.State("command", "name", "close-others") // Open makes a window of its own
	if s5 := state(t, app); s5.Path != other || s5.Error != "" {
		t.Fatalf("after open %+v", s5)
	}
	app.Find(qa.Text("Solo")).Click()
	time.Sleep(300 * time.Millisecond)
	app.Shot("viewer-other-file")
}

type editorState struct {
	Path      string
	Dirty     bool
	Title     string
	Inline    bool
	Popover   bool
	CanUndo   bool
	CanRedo   bool
	Selection []string
	Items     map[string][]float64
	Links     []string
	ItemCount int
	Screen    map[string][]float64
	Zoom      float64
	OriginX   float64
	OriginY   float64
	Element   []string
	Outline   []string
}

func editor(t *testing.T, app *qatest.Client) editorState {
	t.Helper()
	var s editorState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func center(r []float64) (x, y float64) { return r[0] + r[2]/2, r[1] + r[3]/2 }

// itemNames lists the items of an element of a parsed document.
func itemNames(t *testing.T, text string, path ...string) map[string]*model.DiagramItem {
	t.Helper()
	m := model.ReadTenn(text)
	if m == nil {
		t.Fatalf("saved text does not parse:\n%s", text)
	}
	e := &m.Element
	for _, name := range path {
		var next *model.Element
		for _, c := range e.Elements {
			if c.Name == name {
				next = c
			}
		}
		if next == nil {
			t.Fatalf("no element %q in the saved text", name)
		}
		e = next
	}
	out := map[string]*model.DiagramItem{}
	for _, i := range e.Items {
		out[i.Name] = i
	}
	return out
}

// linksOf lists the links of an element of a parsed document as "source -> target".
func linksOf(t *testing.T, text string, path ...string) []string {
	t.Helper()
	m := model.ReadTenn(text)
	if m == nil {
		t.Fatalf("saved text does not parse:\n%s", text)
	}
	e := &m.Element
	for _, name := range path {
		i := slices.IndexFunc(e.Elements, func(c *model.Element) bool { return c.Name == name })
		if i < 0 {
			t.Fatalf("no element %q in the saved text", name)
		}
		e = e.Elements[i]
	}
	var out []string
	for _, i := range e.Items {
		if i.IsLink() && i.Source != nil && i.Target != nil {
			out = append(out, i.Source.Name+" -> "+i.Target.Name)
		}
	}
	return out
}

func TestEditor(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "editor")
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
	shot := func(name string) {
		app.Shot(name)
	}
	element := []string{"Documentation", "Basic steps"}
	canvas := app.Find(qa.ID("canvas"))
	inline := qa.Class("Text").In(qa.ID("canvas"))

	app.Answer(qa.Dialog{Kind: "file", Reply: file})
	menu(app, "Open...")
	app.State("command", "name", "close-others")
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Click()
	app.Settle()
	s := editor(t, app)
	if s.Path != file || s.Dirty || s.CanUndo || len(s.Items) == 0 {
		t.Fatalf("open: %+v", s)
	}
	shot("01-open")
	opened := s

	// gestures reach the camera through the view's router
	z0 := state(t, app)
	canvas.Pinch(qa.PinchEvent{X: 400, Y: 300, Magnification: 0.5, Phase: "changed"})
	z1 := state(t, app)
	if z1.Zoom <= z0.Zoom {
		t.Fatalf("pinch: zoom %v -> %v", z0.Zoom, z1.Zoom)
	}
	canvas.ScrollEvent(qa.ScrollEvent{X: 400, Y: 300, DX: 30, DY: 20, Precise: true})
	if z2 := state(t, app); z2.OriginX == z1.OriginX && z2.OriginY == z1.OriginY {
		t.Fatalf("scroll did not pan: %+v", z2)
	}
	app.State("fit")
	app.Settle()
	if s = editor(t, app); !sameScreen(opened.Screen, s.Screen) {
		t.Fatalf("fit does not restore the opened view: %v vs %v", opened.Screen, s.Screen)
	}

	// select by a click
	x, y := center(s.Screen["Item"])
	still := s
	canvas.ClickAt(x, y)
	if s = editor(t, app); len(s.Selection) != 1 || s.Selection[0] != "Item" || s.Dirty {
		t.Fatalf("click did not select the item: %+v", s)
	}
	// a selection moves nothing: the camera and every item stay where they were on screen
	if s.Zoom != still.Zoom || s.OriginX != still.OriginX || s.OriginY != still.OriginY || !sameScreen(still.Screen, s.Screen) {
		t.Fatalf("click moved the content: camera %v,%v,%v -> %v,%v,%v\nscreen %v\n   -> %v",
			still.Zoom, still.OriginX, still.OriginY, s.Zoom, s.OriginX, s.OriginY, still.Screen, s.Screen)
	}
	shot("02-selected")

	// drag it: the view points moved are the scene units moved times the zoom
	const dx, dy = 140.0, -70.0
	item := s.Items["Item"]
	canvas.DragTo(x, y, x+dx*s.Zoom, y-dy*s.Zoom)
	s = editor(t, app)
	moved := s.Items["Item"]
	if !s.Dirty || !strings.Contains(s.Title, "Edited") || !s.CanUndo || !approx(moved[0], item[0]+dx) || !approx(moved[1], item[1]+dy) {
		t.Fatalf("drag: dirty=%v title=%q undo=%v before %v after %v", s.Dirty, s.Title, s.CanUndo, item, moved)
	}
	shot("04-dragged")

	// Tab adds a linked item
	before := len(s.Items)
	app.Key("Tab")
	s = editor(t, app)
	if len(s.Items) != before+1 || len(s.Selection) != 1 || !strings.HasPrefix(s.Selection[0], "Untitled") {
		t.Fatalf("Tab: %d items, selection %v", len(s.Items), s.Selection)
	}
	fresh := s.Selection[0]
	shot("05-added")

	// Enter opens the inline editor over it; Escape drops the typed text
	app.Key("Enter")
	if s = editor(t, app); !s.Inline {
		t.Fatalf("Enter did not open the inline editor: %+v", s)
	}
	app.Find(inline).Type("Dropped")
	app.Key("Esc")
	if s = editor(t, app); s.Inline || s.Items[fresh] == nil || s.Items["Dropped"] != nil {
		t.Fatalf("Escape: inline=%v items=%v", s.Inline, keys(s.Items))
	}

	// Enter again, type a name, Enter commits
	app.Key("Enter")
	app.Find(inline).Type("Renamed item")
	shot("06-inline-typed")
	app.Key("Enter")
	s = editor(t, app)
	if s.Inline || s.Items["Renamed item"] == nil || s.Items[fresh] != nil {
		t.Fatalf("rename: inline=%v items=%v", s.Inline, keys(s.Items))
	}
	shot("07-renamed")

	// undo the rename, then redo it
	app.Key("Mod+Z")
	if s = editor(t, app); s.Items[fresh] == nil || s.Items["Renamed item"] != nil || !s.CanRedo {
		t.Fatalf("undo: %v", keys(s.Items))
	}
	shot("08-undone")
	app.Key("Mod+Shift+Z")
	if s = editor(t, app); s.Items["Renamed item"] == nil || s.Items[fresh] != nil {
		t.Fatalf("redo: %v", keys(s.Items))
	}
	shot("09-redone")

	// a double click opens the editor over the item under the pointer
	x, y = center(s.Screen["Renamed item"])
	canvas.DoubleClickAt(x, y)
	if s = editor(t, app); !s.Inline {
		t.Fatalf("double click did not open the inline editor: %+v", s)
	}
	app.Key("Esc")

	// Cmd+D duplicates the selection, Delete removes the copy
	if s = editor(t, app); s.Inline {
		t.Fatalf("Escape left the inline editor open: %+v", s)
	}
	count := s.ItemCount
	app.Key("Mod+D") // the links that end at the item are cloned with it
	if s = editor(t, app); s.ItemCount <= count {
		t.Fatalf("duplicate: %d items, was %d", s.ItemCount, count)
	}
	app.Key("Delete")
	if s = editor(t, app); s.ItemCount != count {
		t.Fatalf("delete: %d items, was %d", s.ItemCount, count)
	}

	// Ctrl-drag from one item to another draws a link
	links := len(s.Links)
	fx, fy := center(s.Screen["Item"])
	tx, ty := center(s.Screen["New Item"])
	canvas.Pointer(qa.PointerEvent{Kind: "drag", X: fx, Y: fy, ToX: tx, ToY: ty, Mods: "Ctrl"})
	if s = editor(t, app); len(s.Links) != links+1 || !slices.Contains(s.Links, "Item -> New Item") {
		t.Fatalf("ctrl-drag: links %v, were %d", s.Links, links)
	}
	shot("10-linked")

	// save: the file is the new text, written atomically
	if strings.Contains(read(), "Renamed item") {
		t.Fatal("the file changed before Save")
	}
	app.Key("Mod+S")
	s = editor(t, app)
	if s.Dirty || strings.Contains(s.Title, "Edited") {
		t.Fatalf("still dirty after save: %+v", s)
	}
	saved := itemNames(t, read(), element...)
	was := itemNames(t, string(data), element...)
	if saved["Renamed item"] == nil || saved["Item"] == nil || !approx(saved["Item"].X, was["Item"].X+dx) || !approx(saved["Item"].Y, was["Item"].Y+dy) {
		t.Fatalf("saved file: Item %+v", saved["Item"])
	}
	for name := range was {
		if saved[name] == nil {
			t.Errorf("item %q lost in the saved file", name)
		}
	}
	for _, l := range []string{"Item -> Renamed item", "Item -> New Item"} {
		if !slices.Contains(linksOf(t, read(), element...), l) {
			t.Errorf("the link %s is not in the saved file", l)
		}
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("temp files left in %s: %v", dir, ents)
	}
	shot("11-saved")

	// the context menu of the canvas: a right click on an item, then an entry
	x, y = center(s.Screen["Renamed item"])
	count = s.ItemCount
	canvas.RightClickAt(x, y)
	app.Find(qa.Text("Duplicate").In(qa.ID("canvas"))).Click()
	if s = editor(t, app); s.ItemCount <= count || !s.Dirty {
		t.Fatalf("canvas menu Duplicate: %d items, was %d", s.ItemCount, count)
	}
	app.Key("Mod+Z")
	if s = editor(t, app); s.ItemCount != count {
		t.Fatalf("undo of the menu Duplicate: %d items, was %d", s.ItemCount, count)
	}

	// the context menu of the outline
	rows := len(app.Find(qa.Class("TreeItem")).Nodes())
	tree := app.Find(qa.Class("Tree"))
	tree.RightClickAt(40, 12)
	app.Find(qa.Text("New element").In(qa.Class("Tree"))).Click()
	if got := len(app.Find(qa.Class("TreeItem")).Nodes()); got != rows+1 {
		t.Fatalf("outline menu New element: %d rows, was %d", got, rows)
	}
	app.Key("Mod+Z")
	if got := len(app.Find(qa.Class("TreeItem")).Nodes()); got != rows {
		t.Fatalf("undo of New element: %d rows, was %d", got, rows)
	}
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Click()
	s = editor(t, app)

	// a change on disk while the document has edits: keep the edits, the file stays, Save asks
	canvas.ClickAt(center(s.Screen["Renamed item"]))
	app.Key("Right")
	if s = editor(t, app); !s.Dirty {
		t.Fatal("arrow key did not edit")
	}
	app.Answer(qa.Dialog{Kind: "confirm", Reply: false})
	other := strings.Replace(read(), "Renamed item", "Changed outside", 1)
	if err := os.WriteFile(file, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	app.Settle()
	s = editor(t, app)
	if !s.Dirty || s.Items["Renamed item"] == nil {
		t.Fatalf("reload clobbered the edits: dirty=%v items=%v", s.Dirty, keys(s.Items))
	}
	app.Answer(qa.Dialog{Kind: "confirm", Reply: true})
	app.Key("Mod+S")
	if s = editor(t, app); s.Dirty || !strings.Contains(read(), "Renamed item") {
		t.Fatalf("overwrite save: dirty=%v", s.Dirty)
	}

	// the quick style panel opens over a single selected item
	x, y = center(s.Screen["Item"])
	canvas.ClickAt(x, y)
	menu(app, "Quick Style Panel")
	if s = editor(t, app); !s.Popover {
		t.Fatalf("quick panel not shown: %+v", s)
	}
	app.Shot("12-quick-panel")
	// Esc closes the panel
	app.Key("Esc")
	if s = editor(t, app); s.Popover {
		t.Fatalf("the panel stayed open after Esc: %+v", s)
	}

	// rename a row of the outline: activating it opens a name box over the row
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Activate()
	if s = editor(t, app); !s.Popover {
		t.Fatalf("rename box not shown: %+v", s)
	}
	app.Shot("13-rename")
	app.Find(qa.Class("Text").In(qa.Class("Popover"))).Type("Steps")
	app.Key("Enter")
	if s = editor(t, app); s.Popover || !slices.Contains(s.Outline, "Documentation/Steps") {
		t.Fatalf("rename: %+v", s)
	}
	app.Key("Mod+Z")

	// autosave: an edit is written 2 s after the last change, nobody asks
	onDisk := read()
	canvas.ClickAt(center(editor(t, app).Screen["Renamed item"]))
	app.Key("Left")
	if s = editor(t, app); !s.Dirty {
		t.Fatal("arrow key did not edit")
	}
	time.Sleep(2600 * time.Millisecond)
	app.Settle()
	if s = editor(t, app); s.Dirty || read() == onDisk {
		t.Fatalf("autosave: dirty=%v, file changed=%v", s.Dirty, read() != onDisk)
	}

	// closing a window with edits writes them first, and the window goes; another window stays
	app.Key("Right")
	onDisk = read()
	menu(app, "New")
	if s = editor(t, app); s.Path != "" {
		t.Fatalf("New did not make an untitled window: %+v", s)
	}
	app.Find(qa.Text("Close Window").In(qa.Text("Example.tenn - Edited"))).Click()
	if read() == onDisk {
		t.Fatal("closing did not save the edit")
	}
	if s = editor(t, app); s.Path != "" {
		t.Fatalf("window state after close: %+v", s)
	}
}

func gotoAsked(t *testing.T, app *qatest.Client) int {
	t.Helper()
	var s struct{ GotoAsked int }
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s.GotoAsked
}

// menu clicks a menu item; a toolbar button with the same title is a ToolItem, not picked.
func menu(app *qatest.Client, title string) {
	app.Find(qa.Selector{Class: "MenuItem", Text: title}).Click()
}

func sameScreen(a, b map[string][]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for name, ra := range a {
		rb := b[name]
		if len(rb) != len(ra) {
			return false
		}
		for i := range ra {
			if math.Abs(ra[i]-rb[i]) > 0.01 {
				return false
			}
		}
	}
	return true
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func keys(m map[string][]float64) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

type panelState struct {
	Dirty       bool
	CanUndo     bool
	Props       string
	PropsValues map[string]string
	PropsStyles int
	Selection   []string
	Items       map[string][]float64
	Screen      map[string][]float64
	Outline     []string
	Rows        map[string][]int
	Element     []string
}

func panels(t *testing.T, app *qatest.Client) panelState {
	t.Helper()
	var s panelState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestPanels drives the properties panel, Goto Item and the outline drag with real input.
func TestPanels(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "panels")
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
	canvas := app.Find(qa.ID("canvas"))
	app.Find(qa.ID("insp-code")).Click() // the code section is closed at start
	props := app.Find(qa.Class("StyledText"))
	wait := func() { time.Sleep(700 * time.Millisecond); app.Settle() }

	app.Answer(qa.Dialog{Kind: "message", Reply: "No"}) // TestEditor left edits in the shared window
	app.Answer(qa.Dialog{Kind: "file", Reply: file})
	menu(app, "Open...")
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Click()
	app.Settle()

	// the panel shows the element until an item is selected, then the item
	s := panels(t, app)
	if !strings.Contains(s.Props, "name \"Basic steps\"") {
		t.Fatalf("element text: %q", s.Props)
	}
	x, y := center(s.Screen["Item"])
	canvas.ClickAt(x, y)
	s = panels(t, app)
	if !strings.Contains(s.Props, "color green") || s.PropsStyles == 0 {
		t.Fatalf("item text: %q styles %d", s.Props, s.PropsStyles)
	}
	app.Shot("20-props-item")

	// typing in the panel changes the item: a name with an emoji, a color, an expression
	props.Type("name \"Item \U0001F600\"\npos -555.0 -189.0\ncolor blue\nfont-size ${10 + 20}")
	wait()
	s = panels(t, app)
	if s.Items["Item \U0001F600"] == nil || !s.Dirty || !s.CanUndo {
		t.Fatalf("typed text did not reach the canvas: items %v dirty=%v", keys(s.Items), s.Dirty)
	}
	if s.PropsValues["3"] != "30" {
		t.Fatalf("expression value: %v", s.PropsValues)
	}
	app.Shot("21-props-typed")

	// undo and redo through the model, the panel follows
	app.Key("Mod+Z")
	s = panels(t, app)
	if s.Items["Item \U0001F600"] != nil || !strings.Contains(s.Props, "color green") {
		t.Fatalf("undo: items %v props %q", keys(s.Items), s.Props)
	}
	app.Key("Mod+Shift+Z")
	s = panels(t, app)
	if s.Items["Item \U0001F600"] == nil || !strings.Contains(s.Props, "color blue") {
		t.Fatalf("redo: items %v props %q", keys(s.Items), s.Props)
	}

	// a text that does not parse is marked and changes nothing
	props.Type("name \"Item \U0001F600\"\ncolor {")
	wait()
	s = panels(t, app)
	if s.Items["Item \U0001F600"] == nil || s.PropsStyles == 0 || !strings.Contains(s.Props, "color {") {
		t.Fatalf("broken text: items %v styles %d props %q", keys(s.Items), s.PropsStyles, s.Props)
	}
	app.Shot("22-props-error")

	// a click on another item replaces the text
	x, y = center(s.Screen["New Item"])
	canvas.ClickAt(x, y)
	if s = panels(t, app); !strings.Contains(s.Props, "name \"New Item\"") {
		t.Fatalf("selection did not update the panel: %q", s.Props)
	}

	// save: the file has the typed item
	app.Key("Mod+S")
	if got := itemNames(t, read(), "Documentation", "Basic steps"); got["Item \U0001F600"] == nil {
		t.Fatalf("saved file lacks the item typed in the panel: %v", read())
	}

	// Goto Item: Cmd+F focuses the toolbar field, typing selects and centers the first result, Enter
	// goes to the next one, Esc clears the field
	asked := gotoAsked(t, app)
	canvas.Key("Mod+F")
	if gotoAsked(t, app) != asked+1 {
		t.Fatal("Cmd+F did not reach Goto Item")
	}
	gotoField := app.Find(qa.ID("goto-field"))
	for i := 0; i < 20 && !gotoField.Node().Focused; i++ { // on macOS the focus comes with a layout of the toolbar
		time.Sleep(100 * time.Millisecond)
	}
	if !gotoField.Node().Focused {
		if runtime.GOOS != "darwin" {
			t.Fatalf("Cmd+F did not focus the Goto Item field: %+v", gotoField.Node())
		}
		t.Log("macOS: the Goto Item field is not focused after Cmd+F (gowt FocusSearch is asynchronous)")
	}
	gotoField.Type("'Ctrl'")
	s = panels(t, app)
	if len(s.Selection) != 1 || s.Selection[0] != "hold 'Ctrl'" {
		t.Fatalf("search did not select: %v", s.Selection)
	}
	cx, cy := center(s.Screen["hold 'Ctrl'"])
	if cv := state(t, app).Canvas; math.Abs(cx-float64(cv[0])/2) > 2 || math.Abs(cy-float64(cv[1])/2) > 2 {
		t.Fatalf("the found item is at %v,%v, the canvas is %v", cx, cy, cv)
	}
	app.Shot("23-goto")
	gotoField.Key("Enter")
	app.Settle()
	gotoField.Key("Esc")
	app.Settle()
	if v := gotoField.Node().Value; v != "" {
		t.Fatalf("Esc left %q in the field", v)
	}

	// drop "General info" on the top edge of "Basic steps": the order of the siblings flips
	s = panels(t, app)
	if !slices.Equal(s.Outline, []string{"Documentation", "Documentation/Basic steps", "Documentation/General info"}) {
		t.Fatalf("outline %v", s.Outline)
	}
	to := s.Rows["Basic steps"]
	tree := app.Find(qa.Class("Tree"))
	app.Find(qa.Text("General info").In(qa.Class("Tree"))).DragDropToAt(tree, to[0]+20, to[1]+1)
	wait()
	s = panels(t, app)
	if !slices.Equal(s.Outline, []string{"Documentation", "Documentation/General info", "Documentation/Basic steps"}) {
		t.Fatalf("drag did not move the element: %v", s.Outline)
	}
	app.Shot("24-outline-dragged")
	app.Key("Mod+Z")
	if s = panels(t, app); s.Outline[1] != "Documentation/Basic steps" {
		t.Fatalf("undo of the drag: %v", s.Outline)
	}
}
