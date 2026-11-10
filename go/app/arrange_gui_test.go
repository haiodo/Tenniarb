//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// The Arrange commands of the menu move the selected item through the drawing order: the item list of the
// panel shows the order the items are drawn in, and the last one is on top.
func TestArrangeMovesTheItemInTheOrder(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	openText(t, app, arrangeDoc, "Screen")
	canvas := app.Find(qa.ID("canvas"))

	order := func() []string {
		var st struct {
			ItemList []string `json:"itemList"`
		}
		if err := json.Unmarshal(app.State("state"), &st); err != nil {
			t.Fatal(err)
		}
		return st.ItemList
	}
	before := order()
	if len(before) < 3 {
		t.Fatalf("the document shows %v", before)
	}
	// The first item of the list is at the bottom; Raise takes it one place up.
	x, y := center(par(t, app).Screen[before[0]])
	canvas.ClickAt(x, y)
	app.Settle()
	menu(app, "Raise")
	app.Settle()
	if got := order(); !slices.Equal(got[:2], []string{before[1], before[0]}) {
		t.Fatalf("raise gave %v, was %v", got, before)
	}
	menu(app, "Send to Back")
	app.Settle()
	if got := order(); got[0] != before[0] {
		t.Fatalf("send to back gave %v, was %v", got, before)
	}
}

// arrangeDoc is a classic diagram of three items, the last one drawn on top.
const arrangeDoc = `element "Screen" {
  item A { pos 0 0; width 80; height 40 }
  item B { pos 120 0; width 80; height 40 }
  item C { pos 240 0; width 80; height 40 }
}
`
