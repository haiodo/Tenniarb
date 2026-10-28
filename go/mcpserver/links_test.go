package mcpserver

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/servicewire"
)

func TestScenePath(t *testing.T) {
	for _, c := range []struct {
		text, want string
	}{
		{"a.tenn", ""},
		{"a.tenn#Tracker.Boards/Screen.Kanban", "Tracker.Boards.Screen.Kanban"},
		{"a.tenn#Tracker 2", "Tracker_2"},
		{"a.tenn#Boards#1/Screen#2.Kanban", "Boards.Screen.Kanban"},
		{"a.tenn#Screen/login/Header.Title", "Screen.login"},
		{"a.tenn#/Screen", "Screen"},
	} {
		l, err := link.Parse(c.text)
		if err != nil {
			t.Fatal(err)
		}
		if got := scenePath(l); got != c.want {
			t.Errorf("%s: %q, want %q", c.text, got, c.want)
		}
	}
}

func TestSelectLink(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	repo := tempRoot(t, map[string]string{"design/a.tenn": liveDisk})
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:haiodo/foundation.git"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// The workspace is a folder of the repository; the link names the file from the repository root.
	root := filepath.Join(repo, "design")
	cs := session(t, dir, root, "v1", nil)
	fake, _ := connectFake(t, dir)
	abs := filepath.Join(root, "a.tenn")
	if err := fake.Open(abs, liveDisk); err != nil {
		t.Fatal(err)
	}

	mustOK(t, cs, "select", map[string]any{"file": "design/a.tenn#A/x", "reveal": true})
	if sel := fake.Selected(); len(sel) != 1 || sel[0].Path != abs || sel[0].Nodes[0] != "A.x" || !sel[0].Reveal {
		t.Fatalf("a plain link reached the app as %+v", sel)
	}
	url := "tenniarb://open?repo=github.com/haiodo/foundation&path=design/a.tenn&el=A&node=x"
	mustOK(t, cs, "select", map[string]any{"file": url})
	if sel := fake.Selected(); len(sel) != 2 || sel[1].Nodes[0] != "A.x" {
		t.Fatalf("a URL reached the app as %+v", sel)
	}
	mustFail(t, cs, "select", map[string]any{"file": "tenniarb://open?repo=github.com/other/repo&path=design/a.tenn"}, "is for the repository")
	mustFail(t, cs, "select", map[string]any{"file": "design/a.tenn#A/nope"}, "nothing named")
	mustFail(t, cs, "select", map[string]any{"file": "../outside.tenn#A"}, "stay inside")
	mustFail(t, cs, "select", map[string]any{"file": "other/a.tenn#A"}, "outside the workspace")

	fake.SetSelection(servicewire.Selection{Path: abs, Element: "A", Nodes: []string{"A.x"}})
	got := mustOK(t, cs, "selection", nil)
	for _, want := range []string{
		`"link": "` + url + `"`,
		`"link_text": "design/a.tenn#A/x"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("selection lacks %s:\n%s", want, got)
		}
	}
}
