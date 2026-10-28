//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/haiodo/gowt/qa/qatest"
)

type indicatorState struct {
	Indicators []struct {
		X, Y, Size float64
		Count      int
	}
}

func indicators(t *testing.T, app *qatest.Client) indicatorState {
	t.Helper()
	var s indicatorState
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIndicatorsGUI(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	openCopy(t, app)
	app.State("fit")
	app.Settle()
	if s := indicators(t, app); len(s.Indicators) != 0 {
		t.Fatalf("everything is in view after fit: %+v", s.Indicators)
	}
	app.Shot("indicators-none")

	// Pan the content out of the view to the left: dots on the right edge, one for each row of items.
	app.State("pan", "dx", -4000.0, "dy", 0.0)
	app.Settle()
	s := indicators(t, app)
	if len(s.Indicators) == 0 {
		t.Fatal("items are off screen: dots expected")
	}
	n := 0
	for _, d := range s.Indicators {
		n += d.Count
	}
	if n != editor(t, app).ItemCount {
		t.Errorf("dots stand for %d items, the diagram has %d", n, editor(t, app).ItemCount)
	}
	app.Shot("indicators-panned")

	app.State("fit")
	app.Settle()
	if s := indicators(t, app); len(s.Indicators) != 0 {
		t.Errorf("fit brings the items back: %+v", s.Indicators)
	}
}
