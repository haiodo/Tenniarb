//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa/qatest"

	"github.com/haiodo/tenniarb/go/app"
)

// TestAgentEditAutosaved: an edit that came through the document API is written by the autosave of the
// agent edits, after a pause that a running agent cannot postpone; the human's Save has nothing left.
func TestAgentEditAutosaved(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	data, err := os.ReadFile("../../web/fixtures/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "Example.tenn")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	h.open(file)
	a.State("command", "name", "close-others")
	a.Settle()
	defer h.open(fixture)

	text := string(data)
	item := strings.Index(text, `item "Big yellow item"`)
	at := item + strings.Index(text[item:], "font-size 30") + len("font-size ")
	a.State("edit", "path", file, "start", at, "end", at+2, "new", "24")
	a.Settle()
	if !editor(t, a).Dirty {
		t.Fatal("the agent edit is not shown as unsaved")
	}

	time.Sleep(app.AgentAutosaveEvery + 2*time.Second)
	a.Settle()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "font-size 24") {
		t.Fatalf("the autosave did not write the agent edit:\n%s", b)
	}
	if editor(t, a).Dirty {
		t.Fatal("the document still shows the written edit as unsaved")
	}
}
