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

const (
	guiRenameKit  = "element Parts {\n  item Button { width 40; height 20 }\n}\n"
	guiRenameMain = "element kit --from-file kit.tenn\nelement Screen {\n  use kit.Parts.Button as b\n  item Card { item title { } }\n}\n"
)

// TestRenameInTrees renames from the outline and from the item list: the box opens on a double click,
// the dialog lists the references, Cmd+Z brings the files back.
func TestRenameInTrees(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "rename")
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"kit.tenn": guiRenameKit, "main.tenn": guiRenameMain} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h.open(filepath.Join(dir, "main.tenn"))
	a.State("command", "name", "close-others")

	// the mounted element Parts is renamed from the outline; two paths name it, the dialog asks
	a.Find(qa.Text("kit").In(qa.ID("outline"))).Click()
	a.Answer(qa.Dialog{Kind: "message", Reply: "Yes"})
	a.Find(qa.Text("Parts").In(qa.ID("outline"))).Activate()
	a.Settle()
	a.Find(qa.ID("rename-field")).Type("Pieces")
	a.Find(qa.ID("rename-field")).Key("Enter")
	time.Sleep(400 * time.Millisecond)
	a.Settle()
	if d := a.Dialogs(); len(d) == 0 || !strings.Contains(d[0].Message, "1 references") || !strings.Contains(d[0].Message, "kit.Parts.Button") {
		t.Fatalf("dialogs %+v", d)
	}
	if st := h.state(); st.Error != "" || !contains(st.Outline, "Pieces") {
		t.Fatalf("outline %q, error %q", st.Outline, st.Error)
	}

	// Esc drops the box and nothing changes
	a.Find(qa.Text("Pieces").In(qa.ID("outline"))).Activate()
	a.Find(qa.ID("rename-field")).Type("Nope")
	a.Find(qa.ID("rename-field")).Key("Esc")
	a.Settle()
	if st := h.state(); !contains(st.Outline, "Pieces") || contains(st.Outline, "Nope") {
		t.Fatalf("outline after Esc %q", st.Outline)
	}

	// an item row: no references, so no dialog
	a.Find(qa.Text("Screen").In(qa.ID("outline"))).Click()
	a.Find(qa.Text("Card").In(qa.ID("items"))).Activate()
	h.enterIn("rename-field", "Panel")
	if st := h.state(); st.Error != "" || !contains(st.Items, "Panel") {
		t.Fatalf("items %q, error %q", st.Items, st.Error)
	}

	// undo steps back over the item rename, then over the element rename and the paths together
	a.Key("Cmd+Z")
	a.Key("Cmd+Z")
	a.Settle()
	if st := h.state(); !contains(st.Outline, "Parts") || st.Error != "" {
		t.Fatalf("outline after undo %q, error %q", st.Outline, st.Error)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if strings.Contains(x, s) {
			return true
		}
	}
	return false
}
