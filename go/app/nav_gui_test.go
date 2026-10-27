//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"

	"github.com/haiodo/tenniarb/go/app"
	"github.com/haiodo/tenniarb/go/scene"
)

// navFiles copies fusio-1613 and ui-kit.lib from $TENNIARB_NAV_SRC into dir when it is set, else
// converts penimport/testdata/imports.pen there.
func navFiles(t *testing.T, dir string) (screen, lib string) {
	src := os.Getenv("TENNIARB_NAV_SRC")
	if src == "" {
		return sceneFiles(t, dir)
	}
	for _, f := range []string{"fusio-1613.tenn", "ui-kit.lib.tenn"} {
		if err := os.WriteFile(filepath.Join(dir, f), mustRead(t, filepath.Join(src, f)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "fusio-1613.tenn"), filepath.Join(dir, "ui-kit.lib.tenn")
}

func overlaps(a, b []float64) bool {
	return a[0] < b[0]+b[2] && b[0] < a[0]+a[2] && a[1] < b[1]+b[3] && b[1] < a[1]+a[3]
}

// TestNavigation drives both levels of the sidebar with real input: canvases in the outline, screens in
// Items; reveal, Cmd+D and undo, a component of the library canvas inserted into a frame.
func TestNavigation(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "nav")
	if err != nil {
		t.Fatal(err)
	}
	screen, lib := navFiles(t, dir)
	t.Cleanup(func() {
		orig := filepath.Join(dir, "Restore.tenn")
		if err := os.WriteFile(orig, mustRead(t, "../../web/fixtures/Example.tenn"), 0o644); err != nil {
			t.Error(err)
			return
		}
		h.open(orig)
		a.State("command", "name", "close-others")
	})
	h.open(screen)
	a.State("command", "name", "close-others")
	// the only window: its title changes with the edits and the autosave, so selectors do not name it
	win := func(sel qa.Selector) qa.Selector { return sel }

	// the outline: the file's own canvas and the library, no screens
	st := h.state()
	root := strings.TrimSuffix(filepath.Base(screen), ".tenn")
	if len(st.Outline) != 3 || st.Outline[0] != root || st.Outline[1] != "Libraries" ||
		!strings.HasSuffix(st.Outline[2], " - "+filepath.Base(lib)) || st.Error != "" {
		t.Fatalf("outline %q (error %q)", st.Outline, st.Error)
	}
	libLabel := strings.TrimPrefix(st.Outline[2], "Libraries/")
	alias := strings.TrimSuffix(libLabel, " - "+filepath.Base(lib))

	// the Items list: the top level nodes of the canvas
	tree, _ := scene.Load(dir, filepath.Base(screen))
	tops := app.SceneRows(tree.File(filepath.Base(screen)), "")
	var shown []string
	for _, it := range st.Items {
		if !strings.Contains(it, "/") {
			shown = append(shown, it)
		}
	}
	if len(shown) != len(tops) || len(tops) == 0 || shown[0] != tops[0].Label {
		t.Fatalf("items %q, want the %d top nodes", shown, len(tops))
	}
	a.ShotWindow("01-canvases-and-items", 0)

	// a click on a row of a screen out of view selects it and brings it in
	a.State("zoom", "factor", 8.0)
	a.Settle()
	st = h.state()
	w, hh := float64(st.Canvas[0]), float64(st.Canvas[1])
	view := []float64{0, 0, w, hh}
	nodes := h.scene()
	target, label := "", ""
	area := 0.0
	for _, r := range tops {
		// the biggest screen out of view: a page title is not one
		if n, ok := nodes[r.Path]; ok && !overlaps(n.Box, view) && n.Box[2]*n.Box[3] > area && strings.Count(strings.Join(st.Items, "\n"), r.Label) == 1 {
			target, label, area = r.Path, r.Label, n.Box[2]*n.Box[3]
		}
	}
	if target == "" {
		t.Fatal("no screen out of view at zoom 8")
	}
	a.Find(qa.Text(label).In(win(qa.ID("items")))).Click()
	a.Settle()
	st = h.state()
	b := h.scene()[target].Box
	if len(st.Nodes) != 1 || st.Nodes[0] != target || st.ItemSelected != label {
		t.Fatalf("click on %s: %+v", label, st)
	}
	if b[0] < -1 || b[1] < -1 || b[0]+b[2] > w+1 || b[1]+b[3] > hh+1 {
		t.Fatalf("%s at %v is not in the canvas %vx%v", target, b, w, hh)
	}
	a.ShotWindow("02-revealed", 0)

	// Cmd+D: a copy right of it, clear of the other screens, selected; one undo step
	before := h.text()
	a.Find(win(qa.ID("canvas"))).Focus()
	a.Key("Mod+D")
	a.Settle()
	time.Sleep(200 * time.Millisecond)
	st = h.state()
	if len(st.Nodes) != 1 || st.Nodes[0] == target || !st.CanUndo || st.Error != "" {
		t.Fatalf("Cmd+D: %+v", st)
	}
	cp := st.Nodes[0]
	nodes = h.scene()
	ob, cb := nodes[target].Box, nodes[cp].Box
	if cb[0] <= ob[0]+ob[2] || abs(cb[1]-ob[1]) > 1 || abs(cb[2]-ob[2]) > 1 || abs(cb[3]-ob[3]) > 1 {
		t.Fatalf("copy %s at %v, original at %v", cp, cb, ob)
	}
	for p, n := range nodes {
		if p != cp && !strings.Contains(p, ".") && overlaps(n.Box, cb) {
			t.Fatalf("the copy %v overlaps %s %v", cb, p, n.Box)
		}
	}
	if st.ItemSelected != label+" 2" || !strings.Contains(h.text(), label+" 2") {
		t.Fatalf("copy named %q, want %q", st.ItemSelected, label+" 2")
	}
	a.ShotWindow("03-duplicated", 0)
	a.Key("Mod+Z")
	a.Settle()
	if h.text() != before {
		t.Fatal("undo did not take the copy away")
	}
	a.ShotWindow("04-undone", 0)

	// a component from the library canvas into a new frame
	a.Find(win(h.menuItem("Phone 390 x 844"))).Click()
	a.Settle()
	st = h.state()
	if len(st.Nodes) != 1 {
		t.Fatalf("new screen: %+v", st)
	}
	frame := st.Nodes[0]
	a.Find(qa.Text(libLabel).In(win(qa.ID("outline")))).Click()
	a.Settle()
	st = h.state()
	if st.Lib == "" || len(st.Items) == 0 {
		t.Fatalf("library canvas: %+v", st)
	}
	a.ShotWindow("05-library-canvas", 0)
	cat := ""
	for _, it := range st.Items {
		if r := st.ItemRows[it]; !strings.Contains(it, "/") && r.Kids {
			cat = it
			break
		}
	}
	a.Find(qa.Text(cat).In(win(qa.ID("items")))).Expand()
	a.Settle()
	st = h.state()
	comp := ""
	for _, it := range st.Items {
		name := strings.TrimPrefix(it, cat+"/")
		// names may hold a slash: the child rows are the ones the category row was opened for
		if name != it && st.ItemRows[name].Kind == "item" && st.ItemRows[name].Kids && strings.Count("\n"+strings.Join(st.Items, "\n")+"\n", "/"+name+"\n") == 1 {
			comp = name
			break
		}
	}
	if comp == "" {
		t.Fatalf("no component under %s: %q", cat, st.Items)
	}
	row := a.Find(qa.Text(comp).In(win(qa.ID("items"))))
	row.Click()
	a.Settle()
	box := h.state().ItemRows[comp].Box
	a.Find(win(qa.ID("items"))).RightClickAt(float64(box[0]+box[2]/2), float64(box[1]+box[3]/2))
	a.Settle()
	a.Find(qa.Text("Insert Component").In(win(qa.ID("items")))).Click()
	a.Settle()
	time.Sleep(200 * time.Millisecond)
	st = h.state()
	if st.Lib != "" || len(st.Nodes) != 1 || !strings.HasPrefix(st.Nodes[0], frame+".") || st.Error != "" ||
		!strings.Contains(h.text(), "use "+alias+"."+scene.NormalizeName(cat)+".") {
		t.Fatalf("insert %s from the library into %s: %+v", comp, frame, st)
	}
	a.ShotWindow("06-inserted", 0)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
