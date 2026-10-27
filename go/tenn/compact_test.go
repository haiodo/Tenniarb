package tenn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sameTree: a bare word reads back as an Ident where the source had a string, so those two match
// on the literal.
func sameTree(a, b *Node) bool {
	textual := func(k NodeKind) bool { return k == KindIdent || k == KindStringLit }
	if a.Kind != b.Kind && !(textual(a.Kind) && textual(b.Kind)) {
		return false
	}
	if (a.Token == nil) != (b.Token == nil) || (a.Token != nil && a.Token.Literal != b.Token.Literal) {
		return false
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !sameTree(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

func TestCompactShape(t *testing.T) {
	src := `item "A b" { title "x"; color red; n "12"; e "" }
item Wide { item In { a 1 } }`
	tree, _ := Parse(src)
	want := `item "A b" { title x; color red; n "12"; e "" }
item Wide {
  item In { a 1 }
}`
	if got := Compact(tree); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := CompactWidth(tree, 10); !strings.Contains(got, "item \"A b\" {\n  title x\n") {
		t.Errorf("narrow limit keeps a long block on one line:\n%s", got)
	}
}

func TestCompactRoundTripFixtures(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(fixturesDir, "*.tenn"))
	if len(files) == 0 {
		t.Fatal("no fixtures")
	}
	checked := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		tree, p := Parse(string(data))
		if p.Errors.HasErrors() {
			continue // error recovery trees are not meant to print back
		}
		out := Compact(tree)
		again, p2 := Parse(out)
		if p2.Errors.HasErrors() || !sameTree(tree, again) {
			t.Errorf("%s: compact text does not parse back to the same tree\n%s", filepath.Base(f), out)
		}
		if Compact(again) != out {
			t.Errorf("%s: Compact is not stable", filepath.Base(f))
		}
		checked++
	}
	t.Logf("%d of %d fixtures checked", checked, len(files))
}

func TestCompactStylePack(t *testing.T) {
	tree, _ := Parse("item A {\n  a 1\n  b 2\n  item In { c 3 }\n  d 4\n  e 5\n  item Out { item Deep { f 6 }; g 7 }\n}")
	st := CompactStyle{Indent: "\t", Pack: true, Limit: 20}
	want := "item A {\n\ta 1; b 2\n\titem In { c 3 }\n\td 4; e 5\n\titem Out {\n\t\titem Deep { f 6 }\n\t\tg 7\n\t}\n}"
	if got := CompactAs(tree, st); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	// the limit counts the indent: only the first command stays on a line that is full already
	st.Limit = 8
	if got := CompactAs(tree, st); !strings.Contains(got, "\n\ta 1\n\tb 2\n") {
		t.Errorf("narrow limit packs:\n%s", got)
	}
	again, p := Parse(CompactAs(tree, CompactStyle{Indent: "\t", Pack: true}))
	if p.Errors.HasErrors() || !sameTree(tree, again) {
		t.Error("packed text does not parse back to the same tree")
	}
}
