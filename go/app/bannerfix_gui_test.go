//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// fixableDoc is a scene whose title is written as bare words - a layout makes the title one value, and a
// bare one is several: the canvas names the mistake, and the window offers to quote them, since a reader of
// the picture has no terminal to run `check -fix` in.
const fixableDoc = `element "Screen" {
  item A { auto-layout column; title Gap and padding }
}
`

// A banner that names a diagnostic the fixer repairs carries a Fix button; pressing it quotes the words as
// one step, the document is read again, and the banner goes away with the button.
func TestBannerFixQuotesTheTitle(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	file := openText(t, app, fixableDoc, "Screen")

	diag := stateBanner(t, app)
	if !strings.Contains(diag, "title takes one value") {
		t.Fatalf("the banner does not name the diagnostic: %q", diag)
	}
	if !nodeVisible(app, "banner-fix") {
		t.Fatal("no Fix button for a diagnostic the fixer repairs")
	}

	app.Find(qa.ID("banner-fix")).Click()
	app.Settle()

	if got := stateBanner(t, app); strings.Contains(got, "title takes one value") {
		t.Fatalf("the diagnostic is still there: %q", got)
	}
	if nodeVisible(app, "banner-fix") {
		t.Error("the Fix button stayed after the text was fixed")
	}

	// The fix is a change of the document: Save writes the quotes into the file.
	menu(app, "Save")
	app.Settle()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `title "Gap and padding"`) {
		t.Errorf("the file was not fixed:\n%s", data)
	}
}

// stateBanner is the text the window shows in its banner: the parse and scene errors of the document.
func stateBanner(t *testing.T, app *qatest.Client) string {
	t.Helper()
	var st struct {
		Banner string `json:"banner"`
	}
	if err := json.Unmarshal(app.State("state"), &st); err != nil {
		t.Fatal(err)
	}
	return st.Banner
}

// nodeText is the text a node of the window with that id holds.
func nodeText(app *qatest.Client, id string) string {
	var out string
	var walk func([]qa.Node)
	walk = func(list []qa.Node) {
		for _, n := range list {
			if n.ID == id {
				out = n.Text
			}
			walk(n.Children)
		}
	}
	walk(app.Dump())
	return out
}

// nodeVisible reports whether the window has that node and shows it.
func nodeVisible(app *qatest.Client, id string) bool {
	visible := false
	var walk func([]qa.Node)
	walk = func(list []qa.Node) {
		for _, n := range list {
			if n.ID == id {
				visible = n.Visible
			}
			walk(n.Children)
		}
	}
	walk(app.Dump())
	return visible
}
