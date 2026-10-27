//go:build darwin || linux || windows

package app

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

const stubDoc = `item Page {
  color white
  item Header {
    title Hi
    item Logo { color red }
  }
  item Row { item A; item B { gap 1 }; gap 2 }
  use kit.Button as ok {
    title Go
    at inner { item X { color red } }
  }
  item Row2
}
`

const stubPage = `item Page {
  color white
  item Header
  item Row
  use kit.Button as ok
  item Row2
}`

// merge is the file text after the Code section of the node at path was changed from OwnText to panel.
func merge(t *testing.T, text, path, panel string) string {
	t.Helper()
	return apply(text, ReplaceOwnEdit(text, nodeOf(t, text, path), panel))
}

func TestOwnTextStubsKids(t *testing.T) {
	cases := []struct{ path, want string }{
		{"Page", stubPage},
		{"Page.Header", "item Header {\n  title Hi\n  item Logo\n}"},
		{"Page.Row", "item Row { item A; item B; gap 2 }"},
		{"Page.ok", "use kit.Button as ok {\n  title Go\n  at inner { item X }\n}"},
		{"Page.Row2", "item Row2"},
	}
	for _, c := range cases {
		if got := OwnText(stubDoc, nodeOf(t, stubDoc, c.path)); got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.path, got, c.want)
		}
	}
}

func TestStubRoundTrip(t *testing.T) {
	cyr := "item Экран {\n  item Шапка { title Привет }\n  color red\n}\n"
	for _, text := range []string{stubDoc, cyr, "item P {\n  item A { x 1 }\n  item A { x 2 }\n}\n"} {
		loaded, _ := scene.LoadSource(t.TempDir(), "a.tenn", text)
		tree := loaded.File("a.tenn")
		var walk func(path string)
		walk = func(path string) {
			n := NodeAt(tree, path)
			if got := merge(t, text, path, OwnText(text, n)); got != text {
				t.Errorf("%s:\n%s", path, got)
			}
			for _, k := range sceneKids(n) {
				walk(NodePath(tree, k))
			}
		}
		for _, k := range sceneKids(tree) {
			walk(NodePath(tree, k))
		}
	}
}

func TestStubMerge(t *testing.T) {
	header := "  item Header {\n    title Hi\n    item Logo { color red }\n  }\n"
	row2 := "  item Row2\n"
	cases := []struct{ name, panel, want string }{
		{"own property", strings.Replace(stubPage, "white", "black", 1), strings.Replace(stubDoc, "white", "black", 1)},
		{"own property added", strings.Replace(stubPage, "  item Row2\n", "  item Row2\n  gap 4\n", 1), strings.Replace(stubDoc, "  item Row2\n", "  item Row2\n  gap 4\n", 1)},
		{"moved", strings.Replace(strings.Replace(stubPage, "  item Row2\n", "", 1), "  item Header\n", "  item Row2\n  item Header\n", 1),
			strings.Replace(strings.Replace(stubDoc, row2, "", 1), header, row2+header, 1)},
		{"deleted in the middle", strings.Replace(stubPage, "  item Row\n", "", 1), stubDoc},
		{"deleted first", strings.Replace(stubPage, "  item Header\n", "", 1), stubDoc},
		{"deleted last", strings.Replace(stubPage, "  item Row2\n", "", 1), stubDoc},
		{"renamed", strings.Replace(stubPage, "item Row2", "item Rowx", 1), stubDoc},
		{"new item", strings.Replace(stubPage, "  item Row\n", "  item Fresh { color blue }\n  item Row\n", 1), strings.Replace(stubDoc, "  item Row {", "  item Fresh { color blue }\n  item Row {", 1)},
		{"trailing newline", stubPage + "\n", stubDoc},
	}
	for _, c := range cases {
		if got := merge(t, stubDoc, "Page", c.panel); got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
	bare := strings.Replace(stubPage, "  item Row2\n", "  item Row2\n  item Z\n", 1)
	if got := merge(t, stubDoc, "Page", bare); got != strings.Replace(stubDoc, "  item Row2\n", "  item Row2\n  item Z\n", 1) {
		t.Errorf("new bare item:\n%s", got)
	}
}

func TestStubMergeNested(t *testing.T) {
	got := merge(t, stubDoc, "Page.Header", "item Header {\n  title Yo\n  item Logo\n}")
	if want := strings.Replace(stubDoc, "title Hi", "title Yo", 1); got != want {
		t.Errorf("nested:\n%s", got)
	}
	cyr := "item Экран {\n  item Шапка { title Привет }\n  color red\n}\n"
	if got, want := merge(t, cyr, "Экран", "item Экран {\n  item Шапка\n  color blue\n}"), strings.Replace(cyr, "red", "blue", 1); got != want {
		t.Errorf("cyrillic:\n%s", got)
	}
}

func TestStubMergeOneLine(t *testing.T) {
	got := merge(t, stubDoc, "Page.Row", "item Row { item A; item B; gap 3 }")
	if want := strings.Replace(stubDoc, "gap 2", "gap 3", 1); got != want {
		t.Errorf("edit:\n%s", got)
	}
	got = merge(t, stubDoc, "Page.Row", "item Row { item B; gap 2 }")
	if got != stubDoc {
		t.Errorf("deleted A:\n%s", got)
	}
	got = merge(t, stubDoc, "Page.Row", "item Row { gap 2; item B; item A }")
	if want := strings.Replace(stubDoc, "item A; item B { gap 1 }; gap 2", "gap 2; item B { gap 1 }; item A", 1); got != want {
		t.Errorf("moved:\n%s", got)
	}
}

func TestStubMergeDeletedAll(t *testing.T) {
	got := merge(t, stubDoc, "Page", "item Page {\n  color white\n}")
	for _, kid := range []string{"item Header {\n    title Hi\n    item Logo { color red }\n  }", "item Row { item A; item B { gap 1 }; gap 2 }", "use kit.Button as ok {\n    title Go\n    at inner { item X { color red } }\n  }", "item Row2"} {
		if !strings.Contains(got, kid) {
			t.Errorf("lost %q:\n%s", kid, got)
		}
	}
	if n := len(sceneKids(nodeOf(t, got, "Page"))); n != 4 {
		t.Errorf("%d kids:\n%s", n, got)
	}
}

func TestStubMergeSameName(t *testing.T) {
	text := "item P {\n  item A { x 1 }\n  item A { x 2 }\n}\n"
	if got := merge(t, text, "P", "item P {\n  item A\n  item A\n}"); got != text {
		t.Errorf("unchanged:\n%s", got)
	}
	// stubs of one name are matched by order: the first stub is the first kid
	if got := merge(t, text, "P", "item P {\n  item A\n  color red\n  item A\n}"); got != "item P {\n  item A { x 1 }\n  color red\n  item A { x 2 }\n}\n" {
		t.Errorf("property between:\n%s", got)
	}
	if got := merge(t, text, "P", "item P {\n  item A\n}"); got != text {
		t.Errorf("deleted one:\n%s", got)
	}
}

func TestFindStubs(t *testing.T) {
	stubs, root := findStubs(nodeOf(t, stubDoc, "Page"), stubPage)
	if root == nil || len(stubs) != 4 {
		t.Fatalf("%d stubs", len(stubs))
	}
	for i, name := range []string{"Header", "Row", "ok", "Row2"} {
		if s := stubs[i]; s.Kid.Name != name || !strings.HasSuffix(stubPage[s.Start:s.End], name) {
			t.Errorf("stub %d: %q for %s", i, stubPage[s.Start:s.End], s.Kid.Name)
		}
	}
	if stubs, root := findStubs(nodeOf(t, stubDoc, "Page"), "item Page {"); root != nil || stubs != nil {
		t.Errorf("text that does not parse: %v", stubs)
	}
}
