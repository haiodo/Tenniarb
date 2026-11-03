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

// TestImportPen drives File > Import Pen File: a .tenn is written next to the .pen, it is opened, and
// an import without notes says nothing.
func TestImportPen(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "penimport", "testdata", "components.pen"))
	if err != nil {
		t.Fatal(err)
	}
	pen, tenn := filepath.Join(dir, "components.pen"), filepath.Join(dir, "components.tenn")
	if err := os.WriteFile(pen, src, 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start on the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	// a new window for the import to open in place of it, alone, so the File menu is the one to click
	menu(a, "New")
	a.Settle()
	a.State("command", "name", "close-others")

	a.Answer(qa.Dialog{Kind: "file", Reply: pen})
	a.Find(qa.Text("Import Pen File...").In(qa.Selector{Class: "MenuItem", Text: "File"})).Click()
	var s shellState
	a.Eventually(10*time.Second, func() (any, bool) {
		_, err := os.Stat(tenn)
		s = shell(t, a)
		return s.Path, err == nil && s.Path == tenn
	})
	if s.Dirty || s.Untitled {
		t.Fatalf("after the import: %+v", s)
	}
	if text, err := os.ReadFile(tenn); err != nil || len(text) == 0 {
		t.Fatalf("%s: %v", tenn, err)
	}
	for _, d := range a.Dialogs() {
		if d.Kind == "message" {
			t.Fatalf("a clean import asked: %q", d.Message)
		}
		if d.Kind == "file" && !d.Multi || d.Kind == "file" && !strings.Contains(strings.Join(d.Filters, " "), "*.pen") {
			t.Fatalf("the file dialog: %+v", d)
		}
	}
}
