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
// TestPropsRejectsDocumentNodes keeps an item line out of the properties: applied, it would nest an
// item and take the colour, the text and the place of the item it was typed into.
func TestPropsRejectsDocumentNodes(t *testing.T) {
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
	a.Find(qa.Text("Big yellow item").In(qa.ID("items"))).Click()
	a.Settle()
	before := props()
	if before == "" {
		t.Fatal("nothing in the pane")
	}

	// a new item line: the pane keeps it, the item keeps its properties
	draft := before + "\nitem\n"
	a.Find(qa.ID("props")).Type(draft)
	time.Sleep(500 * time.Millisecond)
	a.Settle()
	a.Find(qa.Text("Big yellow item").In(qa.ID("items"))).Click()
	a.Settle()
	if got := props(); got != draft {
		t.Fatalf("the draft went away: %q", got)
	}
	a.Find(qa.ID("canvas")).ClickAt(700, 700)
	a.Settle()
	a.Find(qa.Text("Big yellow item").In(qa.ID("items"))).Click()
	a.Settle()
	if got := props(); got != before {
		t.Fatalf("the item changed to %q, want %q", got, before)
	}
}

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

const sceneKidDoc = `element "Screen" {
    item "Card" {
        width 100
        height 50
        item "Group" { width 10; height 10; item "Inner" { width 1; height 1 } }
    }
    link "Card" "Card"
}
`

// TestSceneCodeKeepsTypedKid keeps a kid typed in full in the Code section: its properties stay on
// screen to keep writing them, while the kids already there stay stubs.
func TestSceneCodeKeepsTypedKid(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "screen.tenn")
	if err := os.WriteFile(file, []byte(sceneKidDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	props := func() string { return a.Find(qa.ID("props")).Node().Value }
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Screen").In(qa.ID("outline"))).Click()
	a.Settle()
	a.Find(qa.Text("Card").In(qa.ID("items"))).Click()
	a.Settle()
	before := props()
	if !strings.Contains(before, "item \"Group\"\n") {
		t.Fatalf("the kid with kids is not a stub: %q", before)
	}

	draft := strings.Replace(before, "    item \"Group\"", "    item \"New\" { width 5 }\n    item \"Group\"", 1)
	a.Find(qa.ID("props")).Type(draft)
	time.Sleep(500 * time.Millisecond)
	a.Settle()
	got := props()
	if !strings.Contains(got, "item \"New\" { width 5 }") {
		t.Fatalf("the new kid was stubbed: %q", got)
	}
	if !strings.Contains(got, "item \"Group\"\n") {
		t.Fatalf("the kid already there is shown: %q", got)
	}
	if scene := string(a.State("scene")); !strings.Contains(scene, "Card.New") {
		t.Fatalf("the item is not in the scene: %s", scene)
	}
}
