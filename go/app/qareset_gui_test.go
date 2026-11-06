//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa/qatest"
)

// TestQAReset: the state a test starts from holds one window, on its file and without the edits of the
// test before; the windows of the tests before are closed, not piled up on the screen.
func TestQAReset(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "reset")
	if err != nil {
		t.Fatal(err)
	}
	screen, lib := sceneFiles(t, dir)
	type winState struct {
		Windows int
		Path    string
		Dirty   bool
	}
	var st winState
	if err := json.Unmarshal(a.State("state"), &st); err != nil {
		t.Fatal(err)
	}
	start := st.Windows
	h.open(screen)
	h.open(lib)
	if err := json.Unmarshal(a.State("state"), &st); err != nil {
		t.Fatal(err)
	}
	if st.Windows != start+2 {
		t.Fatalf("%d windows, %d before the two opens", st.Windows, start)
	}
	// An edit of the first document, so the reset has something to drop.
	a.State("edit", "path", screen, "start", 0, "end", 0, "new", "// edited\n")
	a.Settle()
	a.State("activate", "path", screen)
	a.Settle()
	if err := json.Unmarshal(a.State("state"), &st); err != nil {
		t.Fatal(err)
	}
	if st.Path != screen || !st.Dirty || st.Windows != start+2 {
		t.Fatalf("before the reset: %+v", st)
	}

	a.State("reset")
	a.Settle()
	time.Sleep(200 * time.Millisecond)
	a.Settle()
	if err := json.Unmarshal(a.State("state"), &st); err != nil {
		t.Fatal(err)
	}
	// The window the application opened at launch is the one kept, back on its file and without edits;
	// the documents the test opened are closed, the edited one with its edit.
	if st.Windows != 1 || st.Dirty || st.Path == screen || st.Path == lib {
		t.Fatalf("after the reset: %+v", st)
	}
	if got := h.text(); strings.HasPrefix(got, "// edited") {
		t.Fatalf("the kept file still shows the edit:\n%.40s", got)
	}
	for _, v := range a.Dump() {
		if v.Class == "Shell" && v.Text != "" {
			t.Logf("window %q", v.Text)
		}
	}
}
