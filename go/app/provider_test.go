//go:build darwin || linux || windows

package app

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProvider(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{}
	p := provider{sh}
	newDoc(t, sh, filepath.Join(dir, "lib.tenn"), libText)
	mapPath := filepath.Join(dir, "map.tenn")
	newDoc(t, sh, mapPath, "element \"Map\" {\n    item \"Idea\"\n    item \"Idea\"\n    item \"Plan\"\n}\n")
	path := filepath.Join(dir, "screen.tenn")
	v := newDoc(t, sh, path, screenText)

	if docs := sh.serviceDocs(); len(docs) != 3 || docs[2].Path != realPath(path) || docs[2].Version != int64(v.version) {
		t.Fatalf("docs %+v", docs)
	}
	r, err := p.Text(path)
	if err != nil || r.Text != screenText || r.Dirty {
		t.Fatalf("text %+v %v", r, err)
	}
	next := strings.Replace(screenText, "pos 0 0", "pos 10 0", 1)
	if _, err := p.Apply(path, r.Version+1, next); !errors.Is(err, ErrStale) {
		t.Fatalf("stale base: %v", err)
	}
	v.props.changes++
	if _, err := p.Apply(path, r.Version, next); !errors.Is(err, errTyping) {
		t.Fatalf("while typing: %v", err)
	}
	v.props.applied = v.props.changes
	ver, err := p.Apply(path, r.Version, next)
	if err != nil || ver != r.Version+1 || v.text != next || !v.isDirty() {
		t.Fatalf("apply %d %v\n%s", ver, err, v.text)
	}
	v.session.Undo()
	if v.text != screenText {
		t.Fatalf("undo:\n%s", v.text)
	}

	if err := p.Select(path, []string{"Screen.ok"}, true); err != nil || !slices.Equal(v.selNodes, []string{"Screen.ok"}) {
		t.Fatalf("select %v %v", err, v.selNodes)
	}
	if sel, _ := p.Selection(); sel.Path != realPath(path) || sel.Element != "" || !slices.Equal(sel.Nodes, []string{"Screen.ok"}) {
		t.Fatalf("selection %+v", sel)
	}
	if err := p.Select(path, []string{"Screen.nope"}, false); err == nil {
		t.Error("a missing node was selected")
	}
	m, _ := sh.viewer(mapPath)
	sh.active = m
	if err := p.Select(mapPath, []string{"Map.Plan"}, false); err != nil || len(m.session.Selection()) != 1 || m.session.Selection()[0].Name != "Plan" {
		t.Fatalf("classic select %v", err)
	}
	if sel, _ := p.Selection(); sel.Element != "Map" || !slices.Equal(sel.Nodes, []string{"Map.Plan"}) {
		t.Fatalf("classic selection %+v", sel)
	}
	if _, err := p.Text(filepath.Join(dir, "other.tenn")); !errors.Is(err, ErrNoDocument) {
		t.Errorf("unknown document: %v", err)
	}
}

func TestDiffRange(t *testing.T) {
	for _, c := range [][2]string{{"abc", "abc"}, {"abc", "abXc"}, {"aaa", "aa"}, {"", "x"}, {"xyz", ""}, {"abab", "ab"}} {
		a, b := diffRange(c[0], c[1])
		if got := c[0][:a] + c[1][a:len(c[1])-b] + c[0][len(c[0])-b:]; got != c[1] || a+b > min(len(c[0]), len(c[1])) {
			t.Errorf("%q -> %q: %d %d", c[0], c[1], a, b)
		}
	}
}
