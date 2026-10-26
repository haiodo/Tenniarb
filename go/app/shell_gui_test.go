//go:build darwin || linux || windows

package app_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

type shellState struct {
	Path      string
	Dirty     bool
	Title     string
	Windows   int
	Recent    []string
	Untitled  bool
	Stash     string
	PrefsOpen bool
	HelpOpen  bool
	Frame     []int
	Scale     float64
	Prefs     struct {
		QuickPanel  bool
		ExpandLevel int
	}
	Outline []string
}

func shell(t *testing.T, app *qatest.Client) shellState {
	t.Helper()
	var s shellState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// inWindow picks a control of the window with the title (menu items exist once per window).
func inWindow(sel qa.Selector, title string) qa.Selector {
	return sel.In(qa.Selector{Class: "Shell", Text: title})
}

func TestShell(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	dir := t.TempDir()
	app.State("command", "name", "close-others")
	was := shell(t, app)
	docTitle := was.Title
	wait := func() { time.Sleep(2600 * time.Millisecond); app.Settle() }

	// File > New: a window of its own with an untitled document
	menu(app, "New")
	s := shell(t, app)
	if s.Windows != 2 || !s.Untitled || s.Dirty || s.Stash == "" || s.Title != "Untitled" {
		t.Fatalf("New: %+v", s)
	}
	t.Logf("export scale %v", s.Scale)
	app.ShotTitle("30-new", s.Title)
	stash := filepath.Join(cfgDir, "untitled", s.Stash+".tenn")

	// an edit is kept in the stash 2 s later; the title says it is edited meanwhile
	app.Find(inWindow(qa.Text("New Top-Level Element"), s.Title)).Click()
	if s = shell(t, app); !s.Dirty || s.Title != "Untitled - Edited" {
		t.Fatalf("edit: %+v", s)
	}
	if _, err := os.Stat(stash); err == nil {
		t.Fatal("stashed before the autosave delay")
	}
	wait()
	data, err := os.ReadFile(stash)
	if err != nil || !strings.Contains(string(data), "Unnamed diagram") {
		t.Fatalf("stash: %v %q", err, data)
	}
	if s = shell(t, app); !s.Dirty || s.Path != "" {
		t.Fatalf("an untitled document stays edited after the autosave: %+v", s)
	}

	// Save As: the file replaces the stash, and the file is in Open Recent
	saved := filepath.Join(dir, "Saved shell.tenn")
	app.Answer(qa.Dialog{Kind: "file", Reply: saved})
	app.Find(inWindow(qa.Text("Save As..."), s.Title)).Click()
	s = shell(t, app)
	if s.Path != saved || s.Dirty || s.Untitled || len(s.Recent) == 0 || s.Recent[0] != saved {
		t.Fatalf("Save As: %+v", s)
	}
	if _, err := os.Stat(stash); err == nil {
		t.Fatal("the stash stays after Save")
	}
	if b, _ := os.ReadFile(saved); !strings.Contains(string(b), "Unnamed diagram") {
		t.Fatalf("saved file: %q", b)
	}

	// edit the file document: written 2 s later without a question
	app.Find(inWindow(qa.Text("New Top-Level Element"), s.Title)).Click()
	if !shell(t, app).Dirty {
		t.Fatal("edit did not mark the document")
	}
	wait()
	if s = shell(t, app); s.Dirty || strings.Count(readFile(t, saved), "element ") < 3 {
		t.Fatalf("autosave of a file: dirty=%v\n%s", s.Dirty, readFile(t, saved))
	}
	app.Shot("31-autosaved")

	// closing the saved window remembers its frame; the other window stays
	frame := s.Frame
	app.Find(inWindow(qa.Text("Close Window"), s.Title)).Click()
	frames := readFile(t, filepath.Join(cfgDir, "windows.json"))
	if !strings.Contains(frames, "Saved shell.tenn") {
		t.Fatalf("windows.json: %s", frames)
	}
	if s = shell(t, app); s.Windows != 1 || s.Title != docTitle {
		t.Fatalf("after close: %+v", s)
	}

	// export the element on the canvas (the window is the only one)
	if s.Path == "" {
		t.Fatalf("no document window: %+v", s)
	}
	app.Find(qa.Text("Basic steps").In(qa.Class("Tree"))).Click()
	for _, c := range []struct{ item, name, want string }{
		{"Export as PNG...", "out.png", "\x89PNG"},
		{"Export as SVG...", "out.svg", "<svg"},
		{"Export as HTML...", "out.html", "data:image/png;base64,"},
		{"Export as interactive HTML...", "live.html", "/*embed stub*/"},
	} {
		out := filepath.Join(dir, c.name)
		app.Answer(qa.Dialog{Kind: "file", Reply: out})
		menu(app, c.item)
		if got := readFile(t, out); !strings.Contains(got, c.want) {
			t.Fatalf("%s: %.200q", c.name, got)
		}
	}
	if keep := os.Getenv("GOWT_QA_ARTIFACTS"); keep != "" {
		for _, n := range []string{"out.png", "out.svg"} {
			os.WriteFile(filepath.Join(keep, "export-"+n), []byte(readFile(t, filepath.Join(dir, n))), 0o644)
		}
	}
	png := []byte(readFile(t, filepath.Join(dir, "out.png")))
	if !bytes.HasPrefix(png, []byte("\x89PNG")) || len(png) < 2000 {
		t.Fatalf("png of %d bytes", len(png))
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "out.svg")), "Basic editing functions") {
		t.Fatal("svg lacks the diagram text")
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "live.html")), `data-element="Documentation/Basic steps"`) {
		t.Fatal("interactive html lacks the element")
	}

	// Open Recent: the entry opens the file in a new window at the remembered size
	menu(app, "Saved shell.tenn")
	s = shell(t, app)
	if s.Windows != 2 || s.Path != saved {
		t.Fatalf("Open Recent: %+v", s)
	}
	if len(frame) != 4 || s.Frame[2] != frame[2] || s.Frame[3] != frame[3] {
		t.Fatalf("frame %v was %v", s.Frame, frame)
	}
	app.Shot("32-recent")
	// the same file again focuses the window
	app.Answer(qa.Dialog{Kind: "file", Reply: saved})
	app.Find(inWindow(qa.Text("Open..."), s.Title)).Click()
	if s = shell(t, app); s.Windows != 2 {
		t.Fatalf("the same file opened a second window: %+v", s)
	}

	// a file handed over by the system (Finder, "Open With") opens in a window of its own
	other := filepath.Join(dir, "Handed over.tenn")
	os.WriteFile(other, []byte("element \"Handed\" {\n}\n"), 0o644)
	app.State("command", "name", "open-document", "path", other)
	if s = shell(t, app); s.Windows != 3 || s.Path != other {
		t.Fatalf("OpenDocument: %+v", s)
	}
	app.State("command", "name", "close-others")
	if s = shell(t, app); s.Windows != 1 {
		t.Fatalf("after close-others: %+v", s)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAbout(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	app.State("command", "name", "close-others")
	app.Answer(qa.Dialog{Kind: "message", Reply: "OK"})
	app.State("command", "name", "about")
	d := app.Dialogs()
	if len(d) != 1 || !strings.Contains(d[0].Message, "Version ") || !strings.Contains(d[0].Message, "Andrey Sobolev") {
		t.Fatalf("about: %+v", d)
	}
}

func TestPrefsAndHelp(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	app.State("command", "name", "close-others")
	app.State("command", "name", "preferences")
	if s := shell(t, app); !s.PrefsOpen || s.Prefs.QuickPanel {
		t.Fatalf("prefs window: %+v", s)
	}
	app.ShotOf("33-prefs", qa.ID("quick-panel"))
	level := shell(t, app).Prefs.ExpandLevel
	app.Find(qa.ID("expand-level")).Type("3")
	if s := shell(t, app); s.Prefs.ExpandLevel != 3 {
		t.Fatalf("expand level typed: %+v", s.Prefs)
	}
	app.Find(qa.ID("expand-level")).Key("Up")
	if s := shell(t, app); s.Prefs.ExpandLevel != 4 {
		t.Fatalf("expand level stepped: %+v", s.Prefs)
	}
	if got := readFile(t, filepath.Join(cfgDir, "settings.json")); !strings.Contains(got, `"expandLevel": 4`) {
		t.Fatalf("settings.json: %s", got)
	}
	app.Find(qa.ID("expand-level")).Type(strconv.Itoa(level))
	app.Find(qa.ID("quick-panel")).Check(true)
	app.Find(qa.ID("background")).Type("#102030")
	if s := shell(t, app); !s.Prefs.QuickPanel {
		t.Fatalf("quick panel setting: %+v", s)
	}
	saved := readFile(t, filepath.Join(cfgDir, "settings.json"))
	if !strings.Contains(saved, `"quickPanel": true`) || !strings.Contains(saved, `"#102030"`) {
		t.Fatalf("settings.json: %s", saved)
	}
	app.Find(qa.ID("background-reset")).Click()
	if got := readFile(t, filepath.Join(cfgDir, "settings.json")); !strings.Contains(got, "#e7e9ebff") {
		t.Fatalf("reset: %s", got)
	}
	// the clicks of the editor tests must not open the quick panel: put the setting back
	app.Find(qa.ID("quick-panel")).Check(false)
	if got := readFile(t, filepath.Join(cfgDir, "settings.json")); !strings.Contains(got, `"quickPanel": false`) {
		t.Fatalf("restore: %s", got)
	}
	app.Find(qa.Selector{Class: "Shell", Text: "Preferences"}).Close()
	if s := shell(t, app); s.PrefsOpen {
		t.Fatal("the window stays registered after close")
	}

	menu(app, "Tenniarb Help")
	if s := shell(t, app); !s.HelpOpen {
		t.Fatalf("help: %+v", s)
	}
	time.Sleep(1500 * time.Millisecond) // the page loads
	app.ShotTitle("34-help", "Tenniarb Help")
	app.Find(qa.Selector{Class: "Shell", Text: "Tenniarb Help"}).Close()
	if s := shell(t, app); s.HelpOpen {
		t.Fatal("help window stays registered after close")
	}

	// Clear Menu empties Open Recent
	if s := shell(t, app); len(s.Recent) == 0 {
		t.Fatalf("recent list is empty: %+v", s)
	}
	menu(app, "Clear Menu")
	if s := shell(t, app); !slices.Equal(s.Recent, nil) {
		t.Fatalf("Clear Menu: %v", s.Recent)
	}
}
