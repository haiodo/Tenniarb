//go:build darwin || linux || windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/link"
)

const linkScene = `element "Tracker" {
  element "Boards" {
    item Screen { pos 0 0; auto-layout column; item Kanban { width 40; height 20 }; item Kanban { width 40; height 20 } }
  }
  element "Tracker 2" {
    item Other { pos 0 0; auto-layout column; item Card { width 40; height 20 } }
  }
}
`

func TestGotoLinkScene(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "p.tenn"), linkScene)
	cases := []struct {
		name    string
		l       link.Link
		element string
		nodes   []string
		missing string
	}{
		{"node", link.Link{Element: []string{"Tracker", "Boards"}, Node: "Screen.Kanban"}, "Boards", []string{"Screen.Kanban"}, ""},
		{"repeated name", link.Link{Element: []string{"Tracker", "Boards"}, Node: "Screen.Kanban#1"}, "Boards", []string{"Screen.Kanban#1"}, ""},
		{"element only", link.Link{Element: []string{"Tracker", "Boards"}}, "Boards", nil, ""},
		{"scene path names", link.Link{Element: []string{"Tracker", "Tracker_2"}, Node: "Other.Card"}, "Tracker 2", []string{"Other.Card"}, ""},
		{"unknown node", link.Link{Element: []string{"Tracker", "Boards"}, Node: "Screen.Nope"}, "Boards", nil, "No node Screen.Nope"},
		{"unknown element", link.Link{Element: []string{"Tracker", "Gone"}, Node: "x"}, "Boards", nil, "No element Tracker.Gone"},
	}
	for _, c := range cases {
		got := v.gotoLink(c.l)
		if !strings.HasPrefix(got, c.missing) || (c.missing == "") != (got == "") {
			t.Errorf("%s: missing %q, want %q", c.name, got, c.missing)
		}
		if c.name == "unknown element" {
			continue // stays where it was
		}
		if v.state.Current.Name != c.element || !slices.Equal(v.selNodes, c.nodes) {
			t.Errorf("%s: element %q nodes %q, want %q %q", c.name, v.state.Current.Name, v.selNodes, c.element, c.nodes)
		}
	}
}

const linkDiagram = "element \"Map\" {\n  item \"Top idea\" { }\n  item \"Other\" { }\n}\n"

func TestGotoLinkDiagram(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "d.tenn"), linkDiagram)
	for _, node := range []string{"Top idea", "Top_idea"} {
		if got := v.gotoLink(link.Link{Element: []string{"Map"}, Node: node}); got != "" {
			t.Fatalf("%s: %s", node, got)
		}
		if sel := v.session.Selection(); len(sel) != 1 || sel[0].Name != "Top idea" || v.linkNode() != "Top idea" {
			t.Fatalf("%s: selection %v", node, sel)
		}
	}
	if got := v.gotoLink(link.Link{Element: []string{"Map"}, Node: "Nope"}); !strings.HasPrefix(got, "No node Nope") || len(v.session.Selection()) != 0 {
		t.Fatalf("unknown item: %q %v", got, v.session.Selection())
	}
}

func TestLinkElement(t *testing.T) {
	v := newDoc(t, &Shell{}, "", linkScene)
	root := &v.session.Root().Element
	for _, c := range []struct {
		segs []string
		want string
	}{
		{nil, ""},
		{[]string{"Tracker"}, "Tracker"},
		{[]string{"Tracker", "Tracker 2"}, "Tracker 2"},
		{[]string{"Tracker", "Tracker_2"}, "Tracker 2"},
		{[]string{"Tracker", "Missing"}, "-"},
	} {
		e := linkElement(root, c.segs)
		switch {
		case c.want == "-" && e != nil, c.want != "-" && e == nil, e != nil && e.Parent != nil && e.Name != c.want:
			t.Errorf("%v: %v", c.segs, e)
		}
	}
}

func gitInit(t *testing.T, dir, remote string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", remote}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestLinkRoots(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	mine := filepath.Join(work, "mine")
	other := filepath.Join(work, "other")
	for _, d := range []string{mine, other} {
		if err := os.MkdirAll(filepath.Join(d, "design"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "design", "P.tenn"), []byte(linkDiagram), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitInit(t, mine, "git@github.com:haiodo/foundation.git")
	gitInit(t, other, "https://github.com/x/other")
	// The recent list knows both clones; only one has the remote of the link.
	recent := []string{filepath.Join(other, "design", "P.tenn"), filepath.Join(mine, "design", "P.tenn")}
	l := link.Link{Repo: "github.com/haiodo/foundation", Path: "design/P.tenn"}
	var repos link.Repos
	roots := linkRoots(l, &repos, dir, nil, recent)
	got, ok := link.Resolve(l, roots, "", isFile)
	if !ok || got != filepath.Join(mine, "design", "P.tenn") {
		t.Fatalf("%q %v from %v", got, ok, roots)
	}
	// A frame saved for a file finds its clone as well.
	if err := SaveFrame(dir, filepath.Join(mine, "design", "P.tenn"), Frame{Width: 10, Height: 10}); err != nil {
		t.Fatal(err)
	}
	if roots := linkRoots(l, &repos, dir, nil, nil); len(roots) != 1 || roots[0].Dir != mine {
		t.Fatalf("frames: %v", roots)
	}
	// A folder the user chose comes first, whatever its own remote.
	chosen := t.TempDir()
	if err := SaveRepoDir(dir, l.Repo, chosen); err != nil {
		t.Fatal(err)
	}
	roots = linkRoots(l, &repos, dir, nil, recent)
	if roots[0].Dir != chosen || roots[0].Remote != l.Repo {
		t.Fatalf("chosen folder: %v", roots)
	}
	// A plain link asks for no remote.
	plain := linkRoots(link.Link{Path: "design/P.tenn"}, &repos, dir, nil, recent)
	if len(plain) != 2 || plain[0].Remote != "" {
		t.Fatalf("plain: %v", plain)
	}
}
