//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"testing"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// treeWidthDoc holds the long names of the foundation's PlatformDesign.tenn: they used to be cut at the
// fixed 220 points of the name column while the panel was wider.
const treeWidthDoc = `element "FUSIO-1620 Глобальный профиль" {
  item "Настройка доски - Быстрые фильтры" { width 10; height 10 }
}
element "2 Пространства - пусто" {
}
`

const (
	treeWidthSidebar = 360
	treeWidthLongest = "FUSIO-1620 Глобальный профиль"
	treeWidthItem    = "Настройка доски - Быстрые фильтры"
)

// treeWidthState is the QA state with the boxes of both trees and their columns.
type treeWidthState struct {
	sceneState
	Rows          map[string][]int `json:"rows"`
	OutlineCols   []int            `json:"outlineCols"`
	OutlineClient int              `json:"outlineClient"`
	ItemCols      []int            `json:"itemCols"`
	ItemClient    int              `json:"itemClient"`
}

func treeWidthRead(t *testing.T, a *qatest.Client) (s treeWidthState) {
	t.Helper()
	if err := json.Unmarshal(a.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// fits reports whether the row box ends before the right edge of its column, which the native tree cuts
// at: a box that runs to the edge was ellipsized.
func fits(box []int, col int) bool { return box[0]+box[2] < col }

// TestTreesNameWidth: the name column of the outline and of the Items pane fills the panel, on the open
// and again after a sidebar toggle and a theme switch; a long name is whole, not cut at 220 points.
func TestTreesNameWidth(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	restDoc(t, h, "Width.tenn", treeWidthDoc)
	a.State("command", "name", "sidebar-width", "width", treeWidthSidebar)
	restWait(a)
	slack := g.Scaled(18)

	// the outline: one column, as wide as the panel, and both long names whole
	st := treeWidthRead(t, a)
	if len(st.OutlineCols) != 1 || st.OutlineCols[0] < st.OutlineClient-slack-2 {
		t.Fatalf("outline columns %v of %d points", st.OutlineCols, st.OutlineClient)
	}
	for _, name := range []string{treeWidthLongest, "2 Пространства - пусто"} {
		box, ok := st.Rows[name]
		if !ok {
			t.Fatalf("no row %q among %v", name, st.Outline)
		}
		if !fits(box, st.OutlineCols[0]) {
			t.Fatalf("row %q is cut: box %v, column %d", name, box, st.OutlineCols[0])
		}
	}

	// a sidebar toggle and a theme switch resize the tree: the name column follows both
	a.State("command", "name", "toolbar", "item", "sidebar")
	restWait(a)
	a.State("command", "name", "toolbar", "item", "sidebar")
	a.State("command", "name", "appearance", "mode", "dark")
	restWait(a)
	a.State("command", "name", "appearance", "mode", "light")
	restWait(a)
	st = treeWidthRead(t, a)
	if st.OutlineCols[0] < st.OutlineClient-slack-2 {
		t.Fatalf("after a toggle: columns %v of %d points", st.OutlineCols, st.OutlineClient)
	}
	if box := st.Rows[treeWidthLongest]; !fits(box, st.OutlineCols[0]) {
		t.Fatalf("after a toggle the row is cut: box %v, column %d", box, st.OutlineCols[0])
	}

	// the Items pane: the name takes all but the narrow "+" column, the item name is whole
	a.Find(qa.Text(treeWidthLongest).In(qa.ID("outline"))).Click()
	restWait(a)
	st = treeWidthRead(t, a)
	if len(st.ItemCols) != 2 || st.ItemCols[0] < st.ItemClient-g.Scaled(24)-slack-2 {
		t.Fatalf("item columns %v of %d points", st.ItemCols, st.ItemClient)
	}
	if box, ok := st.ItemRows[treeWidthItem]; !ok || !fits(box.Box, st.ItemCols[0]) {
		t.Fatalf("item row %q is cut: %+v of column %d", treeWidthItem, box, st.ItemCols[0])
	}
}
