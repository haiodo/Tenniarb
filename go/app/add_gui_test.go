//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// TestAddFrame is Edit > Add > Frame in an empty screen: a frame with a size lands on the canvas,
// selected, and the item list takes its name.
func TestAddFrame(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "screen.tenn")
	if err := os.WriteFile(file, []byte("element \"Screen\" { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start on the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Screen").In(qa.ID("outline"))).Click()
	a.Settle()
	menu(a, "Frame")
	a.Settle()
	scene := string(a.State("scene"))
	if !strings.Contains(scene, `"Frame"`) {
		t.Fatalf("no frame on the canvas: %s", scene)
	}
	if n := h.scene()["Frame"]; n.Box[2] <= 0 || n.Box[3] <= 0 {
		t.Fatalf("the frame has no size: %+v", n)
	}
	if st := h.state(); !st.CanUndo || len(st.Nodes) != 1 || st.ItemSelected != "Frame" {
		t.Fatalf("state after Add > Frame: %+v", st)
	}
}
