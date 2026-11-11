//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa/qatest"
)

// marksText is a screen of one card with a title, so a click, an agent select and an agent edit name
// different nodes of one element.
const marksText = `item "Card" {
    pos 0 0
    width 200
    height 120
    item "Title" { pos 20 90; width 100; height 20 }
}
`

// markState is one mark of the canvas as the QA state lists it.
type markState struct {
	Path string
	Kind string
	Left int64 // milliseconds left
}

type marksState struct {
	sceneState
	Dirty bool
	Marks []markState
}

func marks(t *testing.T, app *qatest.Client) (s marksState) {
	t.Helper()
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// markOf finds the mark of a node of a kind; nil when the canvas has none.
func markOf(s marksState, path, kind string) *markState {
	for i := range s.Marks {
		if s.Marks[i].Path == path && s.Marks[i].Kind == kind {
			return &s.Marks[i]
		}
	}
	return nil
}

// TestAgentMarks drives what an agent does through the document API and reads what the canvas shows: a
// mark on the node it selects or edits, which goes by itself. The marks are a picture: the selection of
// the user and the undo stack stay as they were.
func TestAgentMarks(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "marks")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "marks.tenn")
	if err := os.WriteFile(file, []byte(marksText), 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start from a window with the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	if st := marks(t, a); !st.SceneMode || st.Kind != "scene" || st.Error != "" {
		t.Fatalf("open: %+v", st)
	}
	// the window title names the document with its marks of an unsaved change: look it up every time
	canvas := func() *qatest.Element { return a.Find(inWindow(qaSel("canvas"), h.title(file))) }
	// below the title of the card: a click there names the card itself
	spot := func() (float64, float64) {
		b := h.scene()["Card"].Box
		return b[0] + b[2]*0.7, b[1] + b[3]*0.9
	}

	// the user selects the card: no mark, nothing to undo
	x, y := spot()
	canvas().ClickAt(x, y)
	if st := marks(t, a); !slices.Equal(st.Nodes, []string{"Card"}) || len(st.Marks) != 0 || st.CanUndo {
		t.Fatalf("click: %+v", st)
	}

	// an agent select: the mark names the node, and the select is no undo step
	a.State("select", "path", file, "nodes", []string{"Card.Title"})
	st := marks(t, a)
	if m := markOf(st, "Card.Title", "select"); m == nil || m.Left <= 0 {
		t.Fatalf("select mark: %+v", st)
	}
	if !slices.Equal(st.Nodes, []string{"Card.Title"}) || st.CanUndo {
		t.Fatalf("select: %+v", st)
	}
	shotTitle(a, "01-select-mark", h.title(file))

	// the user selects the card again, the mark of the agent stays
	x, y = spot()
	canvas().ClickAt(x, y)
	if st = marks(t, a); !slices.Equal(st.Nodes, []string{"Card"}) || markOf(st, "Card.Title", "select") == nil {
		t.Fatalf("click over the mark: %+v", st)
	}

	// an agent edit: a mark on the node it changed, the selection of the user stays, one undo step
	text := h.text()
	at := strings.Index(text, "width 200")
	if at < 0 {
		t.Fatalf("no width in\n%s", text)
	}
	a.State("edit", "path", file, "start", at, "end", at+len("width 200"), "new", "width 260")
	st = marks(t, a)
	if m := markOf(st, "Card", "edit"); m == nil || m.Left <= 0 {
		t.Fatalf("edit mark: %+v", st)
	}
	if !slices.Equal(st.Nodes, []string{"Card"}) || !st.Dirty || !strings.Contains(h.text(), "width 260") {
		t.Fatalf("edit: %+v", st)
	}
	shotTitle(a, "02-edit-mark", h.title(file))
	// one undo step puts the text back: the marks made none
	canvas().Focus()
	a.Key("Mod+Z")
	if h.text() != text || marks(t, a).CanUndo {
		t.Fatalf("one undo: canUndo %v", marks(t, a).CanUndo)
	}

	// the window's own edit: a pulse on the node, no label
	x, y = spot()
	canvas().DragTo(x, y, x+40, y-20)
	st = marks(t, a)
	pulse := markOf(st, "Card", "pulse")
	if pulse == nil || pulse.Left <= 0 || pulse.Left > 1200 {
		t.Fatalf("pulse: %+v", st)
	}
	shotTitle(a, "03-pulse", h.title(file))
	a.Key("Mod+Z")

	// the marks go after their time, with no message from anyone
	a.Eventually(8*time.Second, func() (any, bool) {
		st := marks(t, a)
		return len(st.Marks), len(st.Marks) == 0
	})
	if st := marks(t, a); len(st.Marks) != 0 || st.CanUndo || !slices.Equal(st.Nodes, []string{"Card"}) {
		t.Fatalf("after the marks: %+v", st)
	}
}
