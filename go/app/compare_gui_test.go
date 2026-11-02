//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// compareState is the comparison of a window, as the QA state reports it.
type compareState struct {
	On      bool   `json:"on"`
	Rev     string `json:"rev"`
	Repo    string `json:"repo"`
	About   string `json:"about"`
	Markers []string
	Nodes   []struct {
		Path   string `json:"path"`
		Kind   string `json:"kind"`
		Status string `json:"status"`
		Props  int    `json:"props"`
	} `json:"nodes"`
}

func compareOf(t *testing.T, app *qatest.Client) compareState {
	t.Helper()
	var s struct{ Compare compareState }
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s.Compare
}

// statusOf is the status of a node of the comparison, "" when it is not listed.
func (c compareState) statusOf(path string) string {
	for _, n := range c.Nodes {
		if n.Path == path {
			return n.Status
		}
	}
	return ""
}

// hasRow reports whether the outline holds a row with that text, at any depth.
func hasRow(outline []string, text string) bool {
	for _, o := range outline {
		if o == text || strings.HasSuffix(o, "/"+text) {
			return true
		}
	}
	return false
}

// gitRun runs git in dir and fails the test on an error. Only the fixture repository is written to.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestCompareGit compares a file with its last commit, lists what changed in the outline and reverts a
// node as one undo step.
func TestCompareGit(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	wait := func() { time.Sleep(300 * time.Millisecond); a.Settle() }

	const committed = `element "Card" {
  width 240
  item Header { auto-layout row; title "Header" }
  item Body { auto-layout row; title "Body" }
}
`
	const working = `element "Card" {
  width 200
  item Header { auto-layout row; title "Title" }
  item Footer { auto-layout row; title "Footer" }
}
`
	dir := t.TempDir()
	file := filepath.Join(dir, "Card.tenn")
	if err := os.WriteFile(file, []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "add", "Card.tenn")
	gitRun(t, dir, "commit", "-m", "the first version")
	if err := os.WriteFile(file, []byte(working), 0o644); err != nil {
		t.Fatal(err)
	}

	h.open(file)
	a.State("command", "name", "close-others")
	a.State("select", "path", file, "element", []string{"Card"})
	wait()
	if c := compareOf(t, a); c.On {
		t.Fatalf("a comparison is on before it was asked for: %+v", c)
	}

	menu(a, "Compare with Git...")
	wait()
	c := compareOf(t, a)
	if !c.On || c.Rev != "HEAD" || c.Repo == "" {
		t.Fatalf("compare state %+v", c)
	}
	for path, want := range map[string]string{
		"Card":        "changed",
		"Card.Header": "changed",
		"Card.Footer": "added",
		"Card.Body":   "removed",
	} {
		if got := c.statusOf(path); got != want {
			t.Fatalf("%s is %q, want %q (%+v)", path, got, want, c.Nodes)
		}
	}
	if !slices.Contains(c.Markers, "Card.Header") || !slices.Contains(c.Markers, "Card.Footer") {
		t.Fatalf("the canvas marks %v", c.Markers)
	}

	// the outline lists the elements with the changed nodes and the properties of a changed one
	s := panels(t, a)
	for _, row := range []string{"Card  changed  4", "Header  changed", "Footer  added", "Body  removed", "width  240 -> 200"} {
		if !hasRow(s.Outline, row) {
			t.Fatalf("no row %q in %v", row, s.Outline)
		}
	}
	a.Shot("30-compare-git")

	// Revert to HEAD from the context menu of the row: the node is written as the commit has it
	row := s.Rows["Header  changed"]
	tree := a.Find(qa.ID("outline"))
	a.Find(qa.Text("Header  changed").In(qa.ID("outline"))).Click()
	wait()
	var sel struct{ OutlineSelected string }
	_ = json.Unmarshal(a.State("state"), &sel)
	if sel.OutlineSelected != "Header  changed" {
		t.Fatalf("the row is not selected: %q", sel.OutlineSelected)
	}
	tree.RightClickAt(float64(row[0]+row[2]/2), float64(row[1]+row[3]/2))
	a.Find(qa.Text("Revert to HEAD").In(qa.ID("outline"))).Click()
	wait()
	text := h.text()
	if !strings.Contains(text, `title "Header"`) || strings.Contains(text, `title "Title"`) {
		t.Fatalf("the revert did not write the old node:\n%s", text)
	}
	if s := panels(t, a); !s.CanUndo {
		t.Fatal("the revert left no undo step")
	}
	if c := compareOf(t, a); c.statusOf("Card.Header") != "" {
		t.Fatalf("the node is still listed as changed after the revert: %+v", c.Nodes)
	}
	a.Shot("31-compare-reverted")

	// one undo puts the edit back, and the comparison lists the node again
	a.Key("Mod+Z")
	wait()
	if text := h.text(); !strings.Contains(text, `title "Title"`) {
		t.Fatalf("one undo did not bring the edit back:\n%s", text)
	}
	if c := compareOf(t, a); c.statusOf("Card.Header") != "changed" {
		t.Fatalf("the comparison did not follow the undo: %+v", c.Nodes)
	}

	// the item is on the canvas and the frame of the change is drawn for it
	app := a
	if m := compareOf(t, app).Markers; len(m) == 0 {
		t.Fatal("no frame is drawn over the changed nodes")
	}

	// a file outside a repository has nothing to compare with: the item is disabled
	menu(a, "Compare with Git...") // the same item turns the comparison off
	wait()
	if c := compareOf(t, a); c.On {
		t.Fatalf("the comparison is still on: %+v", c)
	}
	if s := panels(t, a); !hasRow(s.Outline, "Card") || hasRow(s.Outline, "Header  changed") {
		t.Fatalf("the ordinary outline did not come back: %v", s.Outline)
	}

	plain := t.TempDir()
	other := filepath.Join(plain, "Plain.tenn")
	if err := os.WriteFile(other, []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	h.open(other)
	a.State("command", "name", "close-others")
	wait()
	item := a.Find(qa.Selector{Class: "MenuItem", Text: "Compare with Git..."})
	if item.Exists() && item.Node().Enabled {
		t.Fatal("Compare with Git is enabled for a file outside a repository")
	}
	if c := compareOf(t, a); c.On {
		t.Fatalf("a comparison survived the open of another document: %+v", c)
	}
}
