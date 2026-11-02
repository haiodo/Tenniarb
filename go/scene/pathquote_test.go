package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// quoteDoc names an element and items with punctuation; the paths write the names in quotes, and the
// key of the old form still reaches the same nodes.
var quoteDoc = `element "2 Пространства - пусто" {
    item "My Button" { width 20; height 10 }
    item "Кнопка (primary)" { width 30; height 10 }
}
element "Screen" {
    use "2 Пространства - пусто"."My Button" as q
    use "2 Пространства - пусто".Кнопка__primary_ as k
    use "2 Пространства - пусто".My_Button as old
}
`

func quoteTree(t *testing.T, text string) *scene.Tree {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "q.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestQuotedPath(t *testing.T) {
	tree := quoteTree(t, quoteDoc)
	el, err := tree.Resolve(tree.File("q.tenn"), `"2 Пространства - пусто"."Кнопка (primary)"`)
	if err != nil || el.Kind != scene.KItem || el.Name != "Кнопка (primary)" {
		t.Fatalf("quoted item = %v, %v", el, err)
	}
	screen, _ := build(t, tree, "Screen")
	m := boxMap(screen)
	wantBox(t, m, "q", 0, 0, 20, 10)
	wantBox(t, m, "k", 0, 0, 30, 10)
	wantBox(t, m, "old", 0, 0, 20, 10) // the key of the name still works
	if _, err := tree.Resolve(tree.File("q.tenn"), `"2 Пространства - пусто"."Nope"`); err == nil || !strings.Contains(err.Error(), `nothing named "Nope"`) {
		t.Errorf("missing quoted name: %v", err)
	}
}

// A quoted segment names a node the way it is written, so it tells two names with one key apart.
func TestQuotedPathDisambiguates(t *testing.T) {
	text := `element "Kit" {
    item "a b" { width 1; height 1 }
    item "a_b" { width 2; height 2 }
}
element "Screen" {
    use Kit."a b" as one
    use Kit."a_b" as two
}
`
	tree, err := scene.LoadSource(t.TempDir(), "d.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	screen, _ := build(t, tree, "Screen")
	m := boxMap(screen)
	wantBox(t, m, "one", 0, 0, 1, 1)
	wantBox(t, m, "two", 0, 0, 2, 2)
}

// A key that two names share is ambiguous, and the candidates come back with their names in quotes.
func TestQuotedPathAmbiguousCandidates(t *testing.T) {
	text := `element "Kit" {
    item "a b" { width 1; height 1 }
    item "a_b" { width 2; height 2 }
}
element "Screen" { use Kit.a_b as x }
`
	_, err := scene.LoadSource(t.TempDir(), "d.tenn", text)
	if err == nil {
		t.Fatal("a key of two names must be ambiguous")
	}
	for _, want := range []string{`ambiguous path Kit.a_b: candidates "a b" (d.tenn:2:5), "a_b" (d.tenn:3:5)`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

// at and replace take the same quoted segments as use.
func TestQuotedPathAtAndReplace(t *testing.T) {
	text := `element "Kit" {
    item "Card" {
        auto-layout column
        item "Кнопка (primary)" { width 30; height 10 }
        item "Body" { width 50; height 20; item "Old" { width 1; height 1 } }
    }
}
element "Screen" {
    use Kit.Card as c {
        at "Кнопка (primary)" { width 60 }
        replace "Body" { item "New" { width 7; height 7 } }
    }
}
`
	screen, _ := build(t, quoteTree(t, text), "Screen")
	m := boxMap(screen)
	wantBox(t, m, "c.Кнопка__primary_", 0, 0, 60, 10)
	wantBox(t, m, "c.New", 0, 10, 7, 7)
	if _, ok := m["c.Body"]; ok {
		t.Errorf("Body was not replaced: %v", paths(screen))
	}
}

// A quote inside a name is written \" in the path, as the lexer reads it in a string.
func TestQuotedPathEscapedQuote(t *testing.T) {
	tree := quoteTree(t, `element "Kit" {
    item "say \"hi\"" { width 3; height 3 }
}
element "Screen" { use Kit."say \"hi\"" as s }
`)
	screen, _ := build(t, tree, "Screen")
	wantBox(t, boxMap(screen), "s", 0, 0, 3, 3)
}

// The segments of a path are what get_state prints and what at and read take.
func TestSplitPath(t *testing.T) {
	cases := []struct {
		path string
		ok   bool
		segs []string
	}{
		{"A.b", true, []string{"A", "b"}},
		{`A."b c".d`, true, []string{"A", `"b c"`, "d"}},
		{`"a.b"`, true, []string{`"a.b"`}},
		{`"say \"hi\""`, true, []string{`"say \"hi\""`}},
		{"A..b", false, nil},
		{"A.", false, nil},
		{".A", false, nil},
		{`"A`, false, nil},
		{"", false, nil},
	}
	for _, c := range cases {
		segs, ok := scene.SplitPath(c.path)
		if ok != c.ok {
			t.Errorf("SplitPath(%q) ok = %v", c.path, ok)
			continue
		}
		if !ok {
			continue
		}
		var got []string
		for _, s := range segs {
			got = append(got, s.Text())
		}
		if strings.Join(got, "|") != strings.Join(c.segs, "|") {
			t.Errorf("SplitPath(%q) = %v, want %v", c.path, got, c.segs)
		}
	}
	if got := scene.SegText("My Button"); got != `"My Button"` {
		t.Errorf("SegText of an awkward name = %s", got)
	}
	for _, name := range []string{"A", "A_b", "2"} {
		if got := scene.SegText(name); got != name {
			t.Errorf("SegText(%q) = %s", name, got)
		}
	}
}
