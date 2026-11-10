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

// longestBase64 is the longest run of base64 characters in s: the data of an inline picture shows up
// as a long one, the stub of the Code section as short words.
func longestBase64(s string) int {
	is := func(c byte) bool {
		return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '='
	}
	best, n := 0, 0
	for i := 0; i < len(s); i++ {
		if is(s[i]) {
			n++
			if n > best {
				best = n
			}
		} else {
			n = 0
		}
	}
	return best
}

// TestCodeHidesClassicImageData opens the golden file with the classic screenshot: selecting its item
// shows a stub in the Code section instead of the base64, and opening alone does not dirty the file.
func TestCodeHidesClassicImageData(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	defer h.open(fixture)
	data, err := os.ReadFile("../../web/fixtures/Example.saved.tenn")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "Example.saved.tenn")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("General info").In(qa.ID("outline"))).Click()
	a.Settle()
	a.Find(qa.Text("Main Window").In(qa.ID("items"))).Click()
	a.Settle()
	props := a.Find(qa.ID("props")).Node().Value
	if props == "" {
		t.Fatal("the Code section is empty")
	}
	if !strings.Contains(props, "@(image data ") {
		t.Fatalf("no stub for the picture:\n%s", props)
	}
	if strings.Contains(props, "iVBORw0KGgo") || longestBase64(props) > 200 {
		t.Fatalf("the data is in the Code section (run %d):\n%s", longestBase64(props), props)
	}
	if st := editor(t, a); st.Dirty {
		t.Fatalf("opening the file dirtied it: %q", st.Title)
	}
}
