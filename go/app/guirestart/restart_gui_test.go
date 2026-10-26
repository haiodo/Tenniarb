//go:build darwin || linux || windows

// Package guirestart tests what survives a restart of the application: untitled documents, Open
// Recent. It starts a new process per test on one config directory.
package guirestart_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

var cfgDir string

func TestMain(m *testing.M) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "tenniarbrestart")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	cfgDir = filepath.Join(dir, "config")
	os.Setenv("TENNIARB_CONFIG_DIR", cfgDir)
	bin := filepath.Join(dir, "tenniarb")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-tags", "gowtqa", "-o", bin, "../../cmd/tenniarb").CombinedOutput(); err != nil {
		println(string(out))
		os.Exit(1)
	}
	qatest.RestartPerTest = true
	qatest.Main(m, bin)
}

type state struct {
	Windows  int
	Untitled bool
	Dirty    bool
	Outline  []string
	Stash    string
	Title    string
	Recent   []string
}

func get(t *testing.T, app *qatest.Client) state {
	t.Helper()
	var s state
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func skip(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
}

func inWindow(sel qa.Selector, title string) qa.Selector {
	return sel.In(qa.Selector{Class: "Shell", Text: title})
}

var saved string

// First launch: a new untitled document. Save it, open another, edit it, and quit at once: the
// quit does not ask, the edits are kept for the next launch.
func TestRestart1Quit(t *testing.T) {
	skip(t)
	app := qatest.App(t)
	s := get(t, app)
	if s.Windows != 1 || !s.Untitled || s.Dirty || s.Title != "Untitled" || len(s.Outline) != 1 {
		t.Fatalf("first launch: %+v", s)
	}
	saved = filepath.Join(t.TempDir(), "First.tenn")
	app.Answer(qa.Dialog{Kind: "file", Reply: saved})
	app.Find(qa.Text("Save As...")).Click()
	if s = get(t, app); s.Untitled || len(s.Recent) != 1 {
		t.Fatalf("Save As: %+v", s)
	}
	app.Find(qa.Text("New")).Click()
	app.Find(inWindow(qa.Text("New Top-Level Element"), "Untitled")).Click()
	s = get(t, app)
	if s.Windows != 2 || !s.Dirty || len(s.Outline) != 2 || s.Stash == "" {
		t.Fatalf("edit: %+v", s)
	}
	stash := filepath.Join(cfgDir, "untitled", s.Stash+".tenn")
	if _, err := os.Stat(stash); err == nil {
		t.Fatal("stashed before anything asked for it")
	}
	// the application exits inside the call, so the answer may never come
	app.Call("state", map[string]any{"name": "command", "args": map[string]any{"name": "quit"}})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stash); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Quit did not keep the untitled document: %v", listUntitled())
}

// Second launch: the document is back, edited, and Open Recent remembers the saved file. Closing it
// with Don't Save removes the stash.
func TestRestart2Restore(t *testing.T) {
	skip(t)
	app := qatest.App(t)
	s := get(t, app)
	if s.Windows != 1 || !s.Untitled || !s.Dirty || len(s.Outline) != 2 || len(s.Recent) != 1 || s.Recent[0] != saved {
		t.Fatalf("restored: %+v", s)
	}
	stash := filepath.Join(cfgDir, "untitled", s.Stash+".tenn")
	if _, err := os.Stat(stash); err != nil {
		t.Fatal(err)
	}
	app.Find(qa.Text("New")).Click()
	app.Answer(qa.Dialog{Kind: "message", Reply: "No"})
	app.Find(inWindow(qa.Text("Close Window"), "Untitled - Edited")).Click()
	if asked := app.Dialogs(); len(asked) == 0 || asked[len(asked)-1].Kind != "message" {
		t.Fatalf("no question on close: %+v", asked)
	}
	if s = get(t, app); s.Windows != 1 || s.Dirty {
		t.Fatalf("after Don't Save: %+v", s)
	}
	if _, err := os.Stat(stash); err == nil {
		t.Fatalf("the stash stays after Don't Save: %v, was %s", listUntitled(), s.Stash)
	}
}

// Third launch: nothing to restore.
func TestRestart3Clean(t *testing.T) {
	skip(t)
	app := qatest.App(t)
	if s := get(t, app); s.Windows != 1 || !s.Untitled || s.Dirty || len(s.Outline) != 1 {
		t.Fatalf("clean launch: %+v", s)
	}
	if strings.Contains(strings.Join(listUntitled(), ","), ".tenn") {
		t.Fatalf("stash left: %v", listUntitled())
	}
}

func listUntitled() []string {
	ents, _ := os.ReadDir(filepath.Join(cfgDir, "untitled"))
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}
