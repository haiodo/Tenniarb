//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// ownComponentsDoc is a document that defines a component kit inline and reuses two of its items, so the
// browser offers them in two groups.
const ownComponentsDoc = `element Kit {
    item Icons {
        item Plus { width 20; height 20; color #ff3b30 }
        item Minus { width 20; height 20; color #34c759 }
    }
    item Chrome {
        item Badge { width 20; height 20; color #0a84ff }
    }
}
element Screen {
    item Frame {
        width 600; height 400; color #ffffff
        use Kit.Icons.Plus as plus
        use Kit.Chrome.Badge as badge
    }
}
element Board {
    item Frame { width 600; height 400; color #ffffff }
}
`

// TestComponentsPanelDrag: the Items pane in its component mode groups the components the document reuses
// by branch, and a drag of a row onto the canvas writes the use there.
func TestComponentsPanelDrag(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	// The QA drag of a GTK tree row never reaches the app listeners under X11.
	if runtime.GOOS == "linux" {
		t.Skip("the drop wrote no use under X11: see .work/gowt-task-linux-qatest-drag.md")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "components")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "components.tenn")
	if err := os.WriteFile(path, []byte(ownComponentsDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(path)
	a.State("command", "name", "close-others")

	// The Board element is the canvas the drop goes to.
	a.Find(qa.Text("Board").In(qa.ID("outline"))).Click()
	a.Settle()
	a.Find(qa.ID("items-mode")).Click()
	a.Settle()
	time.Sleep(300 * time.Millisecond)
	a.Settle()

	labels := h.state().Items
	for _, want := range []string{"Icons", "Plus", "Chrome", "Badge"} {
		if !slices.Contains(labels, want) {
			t.Fatalf("the pane lists %q, want %q in it", labels, want)
		}
	}
	a.ShotOf("components-panel", qa.ID("items"))

	// A press on a component row selects it and leaves the canvas selection alone: the drag below needs the
	// row still selected.
	plus := a.Find(qa.Text("Plus").In(qa.ID("items")))
	plus.Click()
	a.Settle()
	if st := h.state(); st.ItemSelected != "Plus" {
		t.Fatalf("the row of a component is lost on a click: selected %q", st.ItemSelected)
	}

	before := h.text()
	plus.DragDropToAt(a.Find(qa.ID("canvas")), 300, 200)
	a.Settle()
	time.Sleep(300 * time.Millisecond)
	a.Settle()
	after := h.text()
	if strings.Count(after, "use Kit.Icons.Plus as ") <= strings.Count(before, "use Kit.Icons.Plus as ") {
		t.Fatalf("the drop wrote no use:\n%s", after)
	}
	if st := h.state(); st.Error != "" {
		t.Fatalf("the drop left a diagnostic: %s", st.Error)
	}
	a.ShotOf("components-dropped", qa.ID("canvas"))

	// The row menu of a component inserts it; the menu of a canvas node has no place there.
	badge := a.Find(qa.Text("Badge").In(qa.ID("items")))
	b := h.state().ItemRows["Badge"].Box
	badge.RightClickAt(float64(b[0]+b[2]/2), float64(b[1]+b[3]/2))
	a.Settle()
	// "Insert" exists only on a component row: no other menu item reads exactly that.
	a.Find(qa.Text("Insert")).Click()
	a.Settle()
	time.Sleep(300 * time.Millisecond)
	a.Settle()
	if text := h.text(); !strings.Contains(text, "use Kit.Chrome.Badge as ") {
		t.Fatalf("Insert wrote no use:\n%s", text)
	}

	// The picker shows the same groups and previews.
	a.Find(menuBarItem(h.activeTitle(), "Element", "Insert Component...")).Click()
	a.Settle()
	time.Sleep(300 * time.Millisecond)
	a.Settle()
	a.ShotOf("component-picker", qa.ID("component-list"))
	a.Find(qa.ID("component-filter")).Key("Esc")
	a.Settle()
}
