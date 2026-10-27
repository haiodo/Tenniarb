//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"

	"github.com/haiodo/tenniarb/go/app"
	"github.com/haiodo/tenniarb/go/penimport"
	"github.com/haiodo/tenniarb/go/scene"
)

// sceneFiles puts a screen and its library into dir: fusio-1621 and ui-kit.lib from $TENNIARB_PEN_OUT
// when set, else penimport/testdata/imports.pen converted.
func sceneFiles(t *testing.T, dir string) (screen, lib string) {
	if src := os.Getenv("TENNIARB_PEN_OUT"); src != "" {
		for _, f := range []string{"fusio-1621.tenn", "ui-kit.lib.tenn"} {
			b, err := os.ReadFile(filepath.Join(src, f))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(dir, "fusio-1621.tenn"), filepath.Join(dir, "ui-kit.lib.tenn")
	}
	res, err := penimport.Convert([]string{"../penimport/testdata/imports.pen"}, penimport.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Write(dir); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "imports.tenn"), filepath.Join(dir, "lib.tenn")
}

type sceneNode struct {
	Box  []float64
	Kind string
	Kids int
}

type docState struct {
	Title     string
	Version   int
	Builds    int
	SceneMode bool
	Nodes     []string
	Dirty     bool
	Error     string
	Kind      string
}

type sceneState struct {
	SceneMode bool
	Kind      string
	Nodes     []string
	Picked    string
	Error     string
	CanUndo   bool
}

// TestScene opens a screen made by penimport and works on it with real input: select on the canvas,
// edit in the inspector, undo, edit the library in a second window, new screen, insert a component.
func TestScene(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	qa := qaHelpers{t: t, app: qatest.App(t)}
	a := qa.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "scene")
	if err != nil {
		t.Fatal(err)
	}
	screen, lib := sceneFiles(t, dir)
	// the later tests start from a clean window with the fixture, as the parity tests leave it
	t.Cleanup(func() {
		orig := filepath.Join(dir, "Restore.tenn")
		if err := os.WriteFile(orig, mustRead(t, "../../web/fixtures/Example.tenn"), 0o644); err != nil {
			t.Error(err)
			return
		}
		qa.open(orig)
		a.State("command", "name", "close-others")
	})
	screenTitle, libTitle := filepath.Base(screen), filepath.Base(lib)

	qa.open(screen)
	a.State("command", "name", "close-others")
	st := qa.state()
	if !st.SceneMode || st.Kind != "scene" || st.Error != "" {
		t.Fatalf("open: %+v", st)
	}
	before := qa.text()
	a.ShotTitle("01-screen", qa.title(screen))

	// a click on the biggest top node selects it or the deepest node written in the file under the click
	nodes := qa.scene()
	top, area := "", 0.0
	for p, n := range nodes {
		if !strings.Contains(p, ".") && n.Box[2]*n.Box[3] > area {
			top, area = p, n.Box[2]*n.Box[3]
		}
	}
	if top == "" {
		t.Fatalf("no node at the top among %d nodes", len(nodes))
	}
	b := nodes[top].Box
	x, y := b[0]+b[2]*0.5, b[1]+math.Min(b[3]*0.5, 200)
	canvas := func() *qatest.Element { return a.Find(inWindow(qaSel("canvas"), qa.title(screen))) }
	canvas().ClickAt(x, y)
	st = qa.state()
	if len(st.Nodes) != 1 || !strings.HasPrefix(st.Nodes[0]+".", top+".") {
		t.Fatalf("click at %v,%v in %s: %+v", x, y, top, st)
	}
	picked := st.Nodes[0]
	if r := qa.scene()[picked].Box; x < r[0] || y < r[1] || x > r[0]+r[2] || y > r[1]+r[3] {
		t.Fatalf("selected %s at %v is not under the click %v,%v", picked, r, x, y)
	}
	a.ShotTitle("02-selected", qa.title(screen))
	a.State("select", "path", screen, "nodes", []string{top})
	a.Find(inWindow(qa.menuItem("Zoom to Selection"), qa.title(screen))).Click()
	a.Settle()
	time.Sleep(300 * time.Millisecond)
	a.ShotTitle("02-zoomed-"+top, qa.title(screen))
	a.State("select", "path", screen, "nodes", []string{picked})
	a.Settle()

	// a property in the inspector is a text edit, one undo step
	qa.enter("scene-color", "#ff0000")
	after := qa.text()
	if after == before || !strings.Contains(after, "color #ff0000") || len(after)-len(before) > 40 {
		t.Fatalf("color edit changed %d bytes", len(after)-len(before))
	}
	if st = qa.state(); !st.CanUndo || len(st.Nodes) != 1 || st.Nodes[0] != picked {
		t.Fatalf("after edit: %+v", st)
	}
	a.ShotTitle("03-colored", qa.title(screen))
	canvas().Focus()
	a.Key("Mod+Z")
	a.Settle()
	if qa.text() != before {
		t.Fatal("undo did not give the text back")
	}
	a.ShotTitle("04-undone", qa.title(screen))

	// the library in a second window: its unsaved edit redraws the screen
	tree, _ := scene.Load(dir, screenTitle)
	use := firstUse(tree.File(screenTitle))
	if use == nil {
		t.Fatal("the screen uses no component")
	}
	compPath := use.Path[strings.Index(use.Path, ".")+1:]
	qa.open(lib)
	a.State("select", "path", lib, "nodes", []string{compPath})
	a.Settle()
	builds := qa.doc(screen).Builds
	a.ShotTitle("05-library", qa.title(lib))
	qa.enter("scene-color", "#ffd0d0")
	if d := qa.doc(screen); d.Builds <= builds || d.Dirty {
		t.Fatalf("the screen did not follow the library: %+v (builds before %d)", d, builds)
	}
	if d := qa.doc(lib); !d.Dirty {
		t.Fatalf("library: %+v", d)
	}
	// the screen window shows the instance of that component whole
	a.State("select", "path", screen, "nodes", []string{app.NodePath(tree.File(screenTitle), use)})
	a.State("activate", "path", screen)
	a.Find(inWindow(qa.menuItem("Zoom to Selection"), qa.title(screen))).Click()
	a.Settle()
	time.Sleep(300 * time.Millisecond)
	a.ShotTitle("06-screen-after-library", qa.title(screen))
	a.ShotTitle("06-library-edited", qa.title(lib))

	// New Screen and Insert Component in the screen window
	a.State("activate", "path", screen)
	a.Find(inWindow(qa.menuItem("Phone 390 x 844"), qa.title(screen))).Click()
	a.Settle()
	st = qa.state()
	if len(st.Nodes) != 1 || !strings.HasPrefix(st.Nodes[0], "Screen") || !strings.Contains(qa.text(), "width 390; height 844") {
		t.Fatalf("new screen: %+v", st)
	}
	frame := st.Nodes[0]
	a.ShotTitle("07-new-screen", qa.title(screen))

	outline := inWindow(qa.class("Tree"), qa.title(screen))
	libLabel := ""
	for _, r := range a.Find(outline).Node().Children {
		if strings.HasSuffix(r.Text, " - "+libTitle) {
			libLabel = r.Text
		}
	}
	libRow := a.Find(qa.textIn(libLabel, outline))
	libRow.Expand()
	a.Settle()
	cat := qa.childRows(libRow)
	if len(cat) == 0 {
		t.Fatal("the library row has no rows")
	}
	catRow := a.Find(qa.textIn(cat[0], outline))
	catRow.Expand()
	a.Settle()
	comps := qa.childRows(catRow)
	if len(comps) == 0 {
		t.Fatalf("%s has no rows", cat[0])
	}
	comp := comps[len(comps)-1]
	a.Find(qa.textIn(comp, outline)).Click()
	a.Settle()
	if st = qa.state(); st.Picked == "" {
		t.Fatalf("pick %s: %+v", comp, st)
	}
	// the outline took the selection: put the frame back on the canvas
	a.State("select", "path", screen, "nodes", []string{frame})
	a.Settle()
	a.Find(inWindow(qa.menuItem("Insert Component"), qa.title(screen))).Click()
	a.Settle()
	st = qa.state()
	if len(st.Nodes) != 1 || !strings.HasPrefix(st.Nodes[0], frame+".") || !strings.Contains(qa.text(), "use "+st.Picked+" as ") || st.Error != "" {
		t.Fatalf("insert %s: %+v", st.Picked, st)
	}
	a.ShotTitle("08-inserted", qa.title(screen))
}

func qaSel(id string) qa.Selector { return qa.ID(id) }

// firstUse is the first use of a library component in the file, depth first.
func firstUse(n *scene.Node) *scene.Node {
	for _, c := range n.Content {
		if c.Kind == scene.KUse && strings.Contains(c.Path, ".") {
			return c
		}
		if u := firstUse(c); u != nil {
			return u
		}
	}
	return nil
}

type qaHelpers struct {
	t   *testing.T
	app *qatest.Client
}

func (h qaHelpers) open(path string) {
	h.app.State("command", "name", "open-document", "path", path)
	h.app.Settle()
	time.Sleep(300 * time.Millisecond)
	h.app.Settle()
}

func (h qaHelpers) state() (s sceneState) {
	h.t.Helper()
	if err := json.Unmarshal(h.app.State("state"), &s); err != nil {
		h.t.Fatal(err)
	}
	return s
}

func (h qaHelpers) doc(path string) (d docState) {
	h.t.Helper()
	if err := json.Unmarshal(h.app.State("doc", "path", path), &d); err != nil {
		h.t.Fatal(err)
	}
	return d
}

func (h qaHelpers) title(path string) string { return h.doc(path).Title }

func (h qaHelpers) text() (s string) {
	h.t.Helper()
	if err := json.Unmarshal(h.app.State("text"), &s); err != nil {
		h.t.Fatal(err)
	}
	return s
}

func (h qaHelpers) scene() (m map[string]sceneNode) {
	h.t.Helper()
	if err := json.Unmarshal(h.app.State("scene"), &m); err != nil {
		h.t.Fatal(err)
	}
	return m
}

// enter types into an inspector field of the active window and commits it with Enter.
func (h qaHelpers) enter(id, v string) {
	f := h.app.Find(qa.ID(id).In(qa.Selector{Class: "Shell", Text: h.activeTitle()}))
	f.Type(v)
	f.Key("Enter")
	time.Sleep(400 * time.Millisecond)
	h.app.Settle()
}

func (h qaHelpers) activeTitle() string {
	var s struct{ Title string }
	json.Unmarshal(h.app.State("state"), &s)
	return s.Title
}

func (h qaHelpers) menuItem(title string) qa.Selector {
	return qa.Selector{Class: "MenuItem", Text: title}
}

func (h qaHelpers) class(c string) qa.Selector { return qa.Class(c) }

func (h qaHelpers) textIn(text string, in qa.Selector) qa.Selector {
	return qa.Text(text).In(in)
}

// childRows are the texts of the rows under a row.
func (h qaHelpers) childRows(row *qatest.Element) []string {
	var out []string
	for _, c := range row.Node().Children {
		if c.Class == "TreeItem" && c.Text != "" {
			out = append(out, c.Text)
		}
	}
	return out
}
