package optimize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// applied applies f to text and returns the text after the change and the preview.
func applied(t *testing.T, text string, f Finding) (string, string) {
	t.Helper()
	tree := load(t, text)
	edits, preview, err := Apply(tree, f)
	if err != nil {
		t.Fatalf("apply %s: %v", f.Rule, err)
	}
	if len(edits) == 0 {
		t.Fatalf("apply %s: no edits", f.Rule)
	}
	one, err := ApplyText(text, edits)
	if err != nil {
		t.Fatal(err)
	}
	return text[:one.Start] + one.New + text[one.End:], preview
}

func load(t *testing.T, text string) *scene.Tree {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", text)
	if tree == nil {
		t.Fatalf("load: %v", err)
	}
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return tree
}

// looks is the look in effect of every item of a.tenn, by the path of the node: the own commands with
// the properties of the styles it names, which is what Apply must keep the same.
func looks(tree *scene.Tree) map[string]string {
	out := map[string]string{}
	a := &analyzer{tree: tree, file: "a.tenn", src: map[string]string{}}
	a.eachNode(tree.File("a.tenn"), func(n *scene.Node) {
		if n.Kind != scene.KItem {
			return
		}
		var parts []string
		for _, p := range effectiveProps(n) {
			parts = append(parts, p.name+"="+strings.Join(p.args, " "))
		}
		out[nodePath(n)] = strings.Join(parts, ";")
	})
	return out
}

// boxes is the drawn frame of every item of the first element, in drawing order.
func boxes(t *testing.T, tree *scene.Tree) []float64 {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.BuildElement(tx, nil, tree.File("a.tenn").Elems[0], scene.Options{})
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var out []float64
	var walk func(p *scene.Placed)
	walk = func(p *scene.Placed) {
		for _, k := range p.Kids {
			out = append(out, k.X, k.Y, k.W, k.H)
			walk(k)
		}
	}
	walk(screen.Placed())
	return out
}

// TestApplyStyles: three nodes that differ only in colour share one style, each gets use-style, and the
// look and the drawing of every node stay as they were.
func TestApplyStyles(t *testing.T) {
	text := `element "Card" {
    item A { width 18; height 18; color #ffffff }
    item B { width 18; height 18; color #ff0000 }
    item C { width 18; height 18; color #0000ff }
}
`
	tree := load(t, text)
	fs := Analyze(tree)
	if len(fs) != 1 || fs[0].Rule != RuleStyles {
		t.Fatalf("findings %v", rules(fs))
	}
	before, preview := looks(tree), ""
	out, preview := applied(t, text, fs[0])
	if preview == "" || !strings.Contains(preview, "lines") {
		t.Errorf("preview %q", preview)
	}

	after := load(t, out)
	if got := looks(after); !sameMap(before, got) {
		t.Errorf("the look changed:\nold %v\nnew %v", before, got)
	}
	if b, a := boxes(t, tree), boxes(t, after); len(b) != len(a) {
		t.Errorf("boxes %v vs %v", b, a)
	} else {
		for i := range b {
			if b[i] != a[i] {
				t.Fatalf("the drawing changed at %d: %v vs %v", i, b, a)
			}
		}
	}
	// the style is named after the properties, sits in the element's block and every node names it
	if !strings.Contains(out, "styles {\n        w18-h18 { width 18; height 18 }\n    }") {
		t.Errorf("styles block:\n%s", out)
	}
	if n := strings.Count(out, "use-style w18-h18"); n != 3 {
		t.Errorf("%d use-style lines, want 3:\n%s", n, out)
	}
	if strings.Contains(out, "width 18; height 18; color") {
		t.Errorf("the shared properties stayed on the node:\n%s", out)
	}
	if left := Analyze(after); len(left) != 0 {
		t.Errorf("still offered: %v", rules(left))
	}
}

// TestApplyStylesUseStyle: a node that already names a style keeps that line, with the new name after
// it, so the style it named still comes first.
func TestApplyStylesUseStyle(t *testing.T) {
	text := `element "Card" {
    styles {
        base { color #ffffff }
    }
    item A { use-style base; width 20; title One }
    item B { use-style base; width 20; title Two }
}
`
	tree := load(t, text)
	fs := only(t, Analyze(tree), RuleStyles)
	if len(fs) != 1 {
		t.Fatalf("findings %v", rules(Analyze(tree)))
	}
	out, _ := applied(t, text, fs[0])
	if !strings.Contains(out, "use-style base w20") {
		t.Errorf("use-style line:\n%s", out)
	}
	if got, want := looks(load(t, out)), looks(tree); !sameMap(got, want) {
		t.Errorf("the look changed:\nold %v\nnew %v", want, got)
	}
}

// TestApplyLiteral: one look repeated by nodes that have nothing else in common becomes a style, and
// the nodes keep the look they had.
func TestApplyLiteral(t *testing.T) {
	text := `element "E" {
    item A { color #112233; font-size 11; padding 4 }
    item B { color #112233; font-size 11; padding 4; pos 10 0 }
    item C { color #112233; font-size 11; padding 4; title x }
}
`
	tree := load(t, text)
	fs := only(t, Analyze(tree), RuleLiteral)
	if len(fs) != 1 {
		t.Fatalf("findings %v", rules(Analyze(tree)))
	}
	out, preview := applied(t, text, fs[0])
	if preview == "" {
		t.Error("empty preview")
	}
	if !strings.Contains(out, "c112233-fs11-p4 { color #112233; font-size 11; padding 4 }") {
		t.Errorf("style:\n%s", out)
	}
	if got, want := looks(load(t, out)), looks(tree); !sameMap(got, want) {
		t.Errorf("the look changed:\nold %v\nnew %v", want, got)
	}
}

// TestApplyDead: a style nobody names, a group nobody chooses from, an ignored pos and an override that
// changes nothing are deleted whole.
func TestApplyDead(t *testing.T) {
	text := `element "Card" {
    item Root { width 100; height 40 }
}
element "E" {
    styles {
        dead { color red }
        live { color blue }
        alone { color green }
        pick one-of live alone
        pick2 one-of alone
    }
    item A {
        auto-layout column
        item B { use-style live }
        item Moved { pos 10 10; width 5 }
        use Card as one { at Root { width 100 } }
        use Card as two { at Root { } }
    }
}
`
	want := map[string]string{
		"dead-style":    "dead { color red }",
		"dead-group":    "pick2 one-of alone",
		"dead-pos":      "pos 10 10",
		"dead-override": "at Root { width 100 }",
	}
	for rule, gone := range want {
		fs := only(t, Analyze(load(t, text)), rule)
		if len(fs) == 0 {
			t.Fatalf("%s: no finding", rule)
		}
		out, preview := applied(t, text, fs[0])
		if preview == "" {
			t.Errorf("%s: empty preview", rule)
		}
		if strings.Contains(out, gone) {
			t.Errorf("%s: %q stayed:\n%s", rule, gone, out)
		}
		load(t, out) // still parses and loads
	}
	// the empty override is the second dead-override of the file
	fs := only(t, Analyze(load(t, text)), "dead-override")
	if len(fs) != 2 {
		t.Fatalf("dead-override findings %d", len(fs))
	}
	out, _ := applied(t, text, fs[1])
	if strings.Contains(out, "at Root { }") {
		t.Errorf("the empty override stayed:\n%s", out)
	}
	load(t, out)
}

// TestApplyDifferentElements: one style cannot sit in two elements, so a finding that spans them is
// refused and the file is left as it is.
func TestApplyDifferentElements(t *testing.T) {
	text := `element "A" {
    item a1 { color #112233; font-size 11; padding 4 }
}
element "B" {
    item b1 { color #112233; font-size 11; padding 4 }
}
`
	tree := load(t, text)
	fs := only(t, Analyze(tree), RuleStyles)
	if len(fs) != 1 {
		t.Fatalf("styles findings %v", rules(Analyze(tree)))
	}
	edits, preview, err := Apply(tree, fs[0])
	if err == nil || !strings.Contains(err.Error(), "different elements") {
		t.Fatalf("apply: %v", err)
	}
	if len(edits) != 0 || preview != "" {
		t.Errorf("a refused finding still changes something: %v %q", edits, preview)
	}
}

// TestApplyReportOnly: a component and a warning to keep nodes apart are reported, never applied.
func TestApplyReportOnly(t *testing.T) {
	text := `element "E" {
    item Card {
        auto-layout column
        gap 8
        item Head { font-size 18 }
    }
    item Other {
        auto-layout column
        gap 8
        item Head { font-size 18 }
    }
}
`
	fs := Analyze(load(t, text))
	if len(fs) != 1 || fs[0].Rule != RuleRepeat {
		t.Fatalf("findings %v", rules(fs))
	}
	if _, _, err := Apply(load(t, text), fs[0]); err == nil || !strings.Contains(err.Error(), "not applied") {
		t.Errorf("repeat applied: %v", err)
	}
	f := Finding{Rule: RuleNoMerge}
	if _, _, err := Apply(load(t, text), f); err == nil {
		t.Error("no-merge applied")
	}
}

// TestApplyTextOneEdit: several ranges become one edit that makes the same change, as one undo step.
func TestApplyTextOneEdit(t *testing.T) {
	text := "a b c\n"
	one, err := ApplyText(text, []Edit{{4, 5, "C"}, {0, 1, "A"}, {2, 3, "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if one.Start != 0 || one.End != 5 {
		t.Errorf("edit %+v", one)
	}
	if got := text[:one.Start] + one.New + text[one.End:]; got != "A B C\n" {
		t.Errorf("text %q", got)
	}
	// the text between two edits is kept
	one, err = ApplyText("x=1;y=2", []Edit{{6, 7, "20"}, {2, 3, "10"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := "x=1;y=2"[:one.Start] + one.New + "x=1;y=2"[one.End:]; got != "x=10;y=20" {
		t.Errorf("text %q", got)
	}
}

// TestApplyExamples is the quality gate on the real files: every finding that is not a report-only one
// becomes a file that still loads without new errors.
func TestApplyExamples(t *testing.T) {
	for _, file := range []string{"examples/button-styles.tenn", "examples/onboarding-flow.tenn"} {
		root := filepath.Join("..", "..")
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Skipf("%s: %v", file, err)
		}
		tree, err := scene.Load(root, file)
		if tree == nil {
			t.Fatalf("%s: %v", file, err)
		}
		was := len(tree.Diagnostics())
		for _, f := range Analyze(tree) {
			edits, preview, err := Apply(tree, f)
			if err != nil {
				continue // repeat and no-merge: report only
			}
			if preview == "" {
				t.Errorf("%s: %s: empty preview", file, f.Rule)
			}
			src, _ := tree.Source(f.File)
			one, err := ApplyText(src, edits)
			if err != nil {
				t.Fatalf("%s: %s: %v", file, f.Rule, err)
			}
			out := src[:one.Start] + one.New + src[one.End:]
			after, err := scene.LoadSource(root, file, out)
			if after == nil || err != nil {
				t.Fatalf("%s: %s: does not load: %v", file, f.Rule, err)
			}
			if n := len(after.Diagnostics()); n > was {
				t.Errorf("%s: %s: %d diagnostics, was %d", file, f.Rule, n, was)
			}
		}
	}
}

func sameMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
