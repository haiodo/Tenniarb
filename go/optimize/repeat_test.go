package optimize

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// cards is three cards of one structure that differ in their title and colour and sit at other places:
// one element with two parameters and three uses.
const cards = `element "Screen" {
    width 300; height 200
    item CardOne {
        pos 10 10
        auto-layout column
        gap 8; padding 12; color #ffffff; width 120; height 80
        item Head { title "One"; font-size 18 }
    }
    item CardTwo {
        pos 150 10
        auto-layout column
        gap 8; padding 12; color #ff0000; width 120; height 80
        item Head { title "Two"; font-size 18 }
    }
    item CardThree {
        pos 10 110
        auto-layout column
        gap 8; padding 12; color #0000ff; width 120; height 80
        item Head { title "Three"; font-size 18 }
    }
}
`

// built is the element name of a.tenn laid out with the expressions of a screen.
func built(t *testing.T, tree *scene.Tree, name string) *scene.Screen {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		for _, el := range tree.File("a.tenn").Elems {
			if el.Name == name {
				screen, err = tree.BuildElement(tx, nil, el, scene.Options{Measurer: gfx.DefaultMeasurer, EvalVars: exec.SceneEvalVars})
			}
		}
	})
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	return screen
}

// frames is every drawn item of the element as "name x y w h", in the coordinates of the screen and in
// drawing order: equal lists mean the same picture.
func frames(t *testing.T, tree *scene.Tree, name string) []string {
	t.Helper()
	var out []string
	var walk func(*scene.Placed)
	walk = func(p *scene.Placed) {
		for _, k := range p.Kids {
			src := ""
			if k.Src != nil {
				src = k.Src.Name
			}
			out = append(out, fmt.Sprintf("%s %.1f %.1f %.1f %.1f", src, k.X, k.Y, k.W, k.H))
			walk(k)
		}
	}
	walk(built(t, tree, name).Placed())
	return out
}

// drawn is the look and the text of every card of the element, as the screen draws them: the values the
// parameters of the uses pass in, not the defaults of the element.
func drawn(t *testing.T, tree *scene.Tree, name string) map[string]string {
	t.Helper()
	out := map[string]string{}
	var walk func(*scene.Placed)
	walk = func(p *scene.Placed) {
		for _, k := range p.Kids {
			if k.Src != nil && k.Src.Kind == scene.KItem && k.Parent != nil && k.Parent.Src != nil {
				if look := propText(k.Parent.Prop("color")) + " / " + propText(k.Prop("title")); look != " / " {
					out[k.Parent.Src.Name] = look
				}
			}
			walk(k)
		}
	}
	walk(built(t, tree, name).Placed())
	return out
}

// propText is the value in effect of a property: the evaluated one for an expression.
func propText(p *scene.Prop) string {
	if p == nil {
		return ""
	}
	if p.Val != nil {
		return strings.Join(p.Val, " ")
	}
	return strings.Join(p.Values(), " ")
}

// TestApplyRepeat: three cards of one structure become one element with an export let for every value
// that differs, a use per card with its parameters and pos, and the picture stays the same: the same
// boxes, no new diagnostic, and the repeat is gone from the next report.
func TestApplyRepeat(t *testing.T) {
	tree := load(t, cards)
	fs := only(t, Analyze(tree), RuleRepeat)
	if len(fs) != 1 {
		t.Fatalf("repeat findings %d, want 1: %v", len(fs), rules(Analyze(tree)))
	}
	f := fs[0]
	if f.why != "" {
		t.Fatalf("refused: %s", f.why)
	}
	out, preview := applied(t, cards, f)
	if !strings.Contains(preview, "element") || !strings.Contains(preview, "use ") {
		t.Errorf("preview %q", preview)
	}
	for _, want := range []string{
		`element "CardOne-2" {`,
		"kind Component",
		"export let color = #ffffff",
		`export let title = "One"`,
		`use "CardOne-2" as CardOne { color #ffffff; title One; pos 10 10 }`,
		`use "CardOne-2" as CardTwo { color #ff0000; title Two; pos 150 10 }`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "export let"); n != 2 {
		t.Errorf("%d parameters, want 2:\n%s", n, out)
	}
	if n := strings.Count(out, `use "CardOne-2" as`); n != 3 {
		t.Errorf("%d uses, want 3:\n%s", n, out)
	}
	if n := strings.Count(out, "pos 10 10"); n != 1 {
		t.Errorf("%d pos 10 10, want 1 (the use only, not the body):\n%s", n, out)
	}

	after := load(t, out)
	if n, was := len(after.Diagnostics()), len(tree.Diagnostics()); n > was {
		t.Errorf("%d diagnostics, was %d: %v", n, was, after.Diagnostics())
	}
	if b, a := frames(t, tree, "Screen"), frames(t, after, "Screen"); !slices.Equal(b, a) {
		t.Errorf("the boxes changed:\nold %v\nnew %v", b, a)
	}
	// and every card keeps its own colour and title: the uses pass them into the component
	if b, a := drawn(t, tree, "Screen"), drawn(t, after, "Screen"); len(b) != len(a) {
		t.Errorf("looks %v vs %v", b, a)
	} else {
		for k, v := range b {
			if a[k] != v {
				t.Errorf("%s looks %q, was %q", k, a[k], v)
			}
		}
	}
	if left := only(t, Analyze(after), RuleRepeat); len(left) != 0 {
		t.Errorf("still offered: %v", rules(left))
	}
	// the same finding against the new text is stale, and says so instead of cutting the wrong range
	if _, _, err := Apply(after, f); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Errorf("a stale finding was applied: %v", err)
	}
}

// TestRepeatRefused: a set the tool cannot fold into a component is still reported, the report says
// why, and Apply changes nothing.
func TestRepeatRefused(t *testing.T) {
	style := `element "E" {
    styles {
        one { Head { title "One" } }
        two { Head { title "Two" } }
    }
    item Card1 {
        use-style one; pos 0 0
        auto-layout column; gap 8; padding 12; width 100; height 60
        item Head { width 40; height 20; font-size 16 }
    }
    item Card2 {
        use-style two; pos 150 0
        auto-layout column; gap 8; padding 12; width 100; height 60
        item Head { width 40; height 20; font-size 16 }
    }
}
`
	for _, tc := range []struct{ name, text, why string }{
		{"nested name", `element "E" {
    item Card {
        auto-layout column; gap 8; padding 16
        item Head { width 40; height 20 }
        item Body { width 40; height 20 }
    }
    item Other {
        auto-layout column; gap 8; padding 16
        item Top { width 40; height 20 }
        item Body { width 40; height 20 }
    }
}
`, "nested node is named differently"},
		{"use path", `element "Icon" { kind Component; item I { width 8; height 8 } }
element "Dot" { kind Component; item I { width 8; height 8 } }
element "E" {
    item A {
        auto-layout row; gap 4; padding 8
        item S1 { width 10; height 10 }
        item S2 { width 10; height 10 }
        use Icon as ic
    }
    item B {
        auto-layout row; gap 4; padding 8
        item S1 { width 10; height 10 }
        item S2 { width 10; height 10 }
        use Dot as ic
    }
}
`, "nested use has another path"},
		{"style", style, "name different styles"},
		{"inside a use", `element "Box" {
    kind Component
    item Root { slot }
}
element "E" {
    use Box as box {
        item A {
            pos 0 0
            auto-layout column
            gap 8
            padding 12
            width 100
            height 60
            item Head { width 40; height 20; title a }
        }
        item B {
            pos 10 0
            auto-layout column
            gap 8
            padding 12
            width 100
            height 60
            item Head { width 40; height 20; title a }
        }
    }
}
`, "sits in the block of a use"},
		{"nested pos", `element "E" {
    item Card {
        auto-layout column; gap 8; padding 16; width 100; height 60
        item Head { pos 4 4; width 40; height 20; font-size 16 }
        item Body { width 40; height 20; title a }
    }
    item Other {
        auto-layout column; gap 8; padding 16; width 100; height 60
        item Head { pos 8 4; width 40; height 20; font-size 16 }
        item Body { width 40; height 20; title a }
    }
}
`, "nested node sits at another place"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := load(t, tc.text)
			fs := only(t, Analyze(tree), RuleRepeat)
			if len(fs) != 1 {
				t.Fatalf("repeat findings %d, want 1: %v", len(fs), rules(Analyze(tree)))
			}
			f := fs[0]
			if !strings.Contains(f.Detail, "apply by hand") || !strings.Contains(f.Detail, tc.why) {
				t.Errorf("the report does not say why: %q", f.Detail)
			}
			edits, preview, err := Apply(tree, f)
			if err == nil || !strings.Contains(err.Error(), tc.why) {
				t.Fatalf("apply: %v, want %q", err, tc.why)
			}
			if len(edits) != 0 || preview != "" {
				t.Errorf("a refused finding still changes something: %v %q", edits, preview)
			}
		})
	}
}
