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
)

// TestPropsKeepsInvalidDraft keeps text that does not parse in the props pane: the user fixes it there,
// and it goes away only when the code changes the text.
func TestPropsKeepsInvalidDraft(t *testing.T) {
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
	props := func() string { return a.Find(qa.ID("props")).Node().Value }
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Basic steps").In(qa.ID("outline"))).Click()
	a.Settle()

	// a draft that does not parse stays after the debounce and after the focus leaves the pane
	draft := "element Broken {\n  item x { color\n}\n"
	a.Find(qa.ID("props")).Type(draft)
	time.Sleep(500 * time.Millisecond)
	a.Settle()
	a.Find(qa.ID("canvas")).ClickAt(700, 700)
	a.Settle()
	if got := props(); got != draft {
		t.Fatalf("the draft went away: %q", got)
	}
	if st := h.state(); !contains(st.Outline, "Basic steps") {
		t.Fatalf("the document went away: %q", st.Outline)
	}

	// the code changes the text: the draft makes way for it
	a.Find(qa.ID("insp-name")).Type("Steps")
	a.Find(qa.ID("insp-name")).Key("Enter")
	time.Sleep(400 * time.Millisecond)
	a.Settle()
	if got := props(); strings.Contains(got, "Broken") || !strings.Contains(got, "Steps") {
		t.Fatalf("the draft outlived the new text: %q", got)
	}
}
