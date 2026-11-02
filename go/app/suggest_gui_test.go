//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// suggestDoc is a screen with a look repeated by three nodes: Element > Suggest Improvements offers one
// style for it.
const suggestDoc = `element "Screen" {
  width 390; height 844
  item A { auto-layout row; item Inner { width 18; height 18; color #ffffff } }
  item B { auto-layout row; item Inner { width 18; height 18; color #ff0000 }; item Dot { width 2; height 2 } }
  item C { auto-layout row; item Inner { width 18; height 18; color #0000ff }; item Gap { width 4; height 4 } }
}
`

// suggestCards is three cards of one structure at other places: Element > Suggest Improvements offers
// one component for them, and applying it is one edit.
const suggestCards = `element "Screen" {
  width 390; height 844
  item CardOne {
    pos 10 10
    auto-layout column
    gap 8; padding 12; color #ffffff; width 120; height 80
    item Head { title "One"; font-size 18 }
  }
  item CardTwo {
    pos 150 10
    auto-layout column
    gap 8; padding 12; color #ff0000; width 120; height 80
    item Head { title "Two"; font-size 18 }
  }
  item CardThree {
    pos 10 110
    auto-layout column
    gap 8; padding 12; color #0000ff; width 120; height 80
    item Head { title "Three"; font-size 18 }
  }
}
`

// suggestState is what the Suggest Improvements list holds, as the QA state reports it.
type suggestState struct {
	Findings int
	Preview  string
	Version  int
}

func suggestion(t *testing.T, app *qatest.Client) suggestState {
	t.Helper()
	var s struct{ Suggest suggestState }
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s.Suggest
}

// TestSuggest is Element > Suggest Improvements on a screen with one repeated look: the list shows the
// finding, Preview says what changes, Apply writes the style as one edit, and one undo brings the file
// back.
func TestSuggest(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "Suggest.tenn")
	if err := os.WriteFile(file, []byte(suggestDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start on the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.State("select", "path", file, "element", []string{"Screen"})
	a.Settle()
	if !h.doc(file).SceneMode {
		t.Fatalf("the screen is not on the canvas: %+v", h.doc(file))
	}
	before := h.text()

	menu(a, "Suggest Improvements...")
	a.Settle()
	st := suggestion(t, a)
	if st.Findings == 0 {
		t.Fatal("no finding in the list")
	}
	a.ShotTitle("suggest-1-list", h.title(file))

	a.Find(qa.ID("suggest-preview-0")).Click()
	a.Settle()
	if st := suggestion(t, a); !strings.Contains(st.Preview, "use-style") || !strings.Contains(st.Preview, "lines") {
		t.Fatalf("preview %q", st.Preview)
	}

	listed := suggestion(t, a).Findings
	a.Find(qa.ID("suggest-apply-0")).Click()
	a.Settle()
	a.ShotTitle("suggest-2-applied", h.title(file))
	applied := h.text()
	if applied == before || !strings.Contains(applied, "use-style") || !strings.Contains(applied, "styles {") {
		t.Fatalf("after apply:\n%s", applied)
	}
	// the list is rebuilt from the new text: the look is named now, so that finding is gone
	if st := suggestion(t, a); st.Findings >= listed {
		t.Fatalf("the list was not rebuilt: %d findings, was %d", st.Findings, listed)
	}

	a.State("command", "name", "undo")
	a.Settle()
	if got := h.text(); got != before {
		t.Fatalf("the apply took more than one undo step:\n%s", got)
	}
}

// TestSuggestRepeat: a repeated structure is offered as a component and applied from the list: Preview
// shows the element and the uses, Apply writes them as one edit, and one undo brings the file back.
func TestSuggestRepeat(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "Repeat.tenn")
	if err := os.WriteFile(file, []byte(suggestCards), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.State("select", "path", file, "element", []string{"Screen"})
	a.Settle()
	before := h.text()

	menu(a, "Suggest Improvements...")
	a.Settle()
	if st := suggestion(t, a); st.Findings == 0 {
		t.Fatal("no finding in the list")
	}
	a.ShotTitle("suggest-repeat-1-list", h.title(file))

	a.Find(qa.ID("suggest-preview-0")).Click()
	a.Settle()
	if st := suggestion(t, a); !strings.Contains(st.Preview, `element "CardOne-2"`) || !strings.Contains(st.Preview, "use ") {
		t.Fatalf("preview %q", st.Preview)
	}

	a.Find(qa.ID("suggest-apply-0")).Click()
	a.Settle()
	a.ShotTitle("suggest-repeat-2-applied", h.title(file))
	applied := h.text()
	for _, want := range []string{`element "CardOne-2" {`, "export let color", "export let title"} {
		if !strings.Contains(applied, want) {
			t.Fatalf("missing %q after apply:\n%s", want, applied)
		}
	}
	if n := strings.Count(applied, `use "CardOne-2" as`); n != 3 {
		t.Fatalf("%d uses, want 3:\n%s", n, applied)
	}
	if st := suggestion(t, a); st.Findings != 0 {
		t.Fatalf("the list was not rebuilt: %d findings", st.Findings)
	}

	a.State("command", "name", "undo")
	a.Settle()
	if got := h.text(); got != before {
		t.Fatalf("the apply took more than one undo step:\n%s", got)
	}
}
