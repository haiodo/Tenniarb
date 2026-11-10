package scene_test

import (
	"strings"
	"testing"
)

// TestContextInherit: a context command on an item declares for its subtree, and the node that writes it
// reads it in its own properties too (spec 5f).
func TestContextInherit(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "ctx.tenn", `element "Screen" {
  item Wrap {
    context theme dark
    width ${ctx.theme === "dark" ? 60 : 20}; height 20
    item Box { title ${ctx.theme} }
  }
}`), "Screen")
	m := boxMap(screen)
	if w := m["Wrap"].W; w != 60 {
		t.Errorf("Wrap width %v, want 60 from its own context", w)
	}
	if l := m["Wrap.Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("Box title %v, want dark", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextSoft: ?= is a default for a use without a provider, and the place of the use wins over it.
func TestContextSoft(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "soft.tenn", `element "Card" {
  kind Component
  context tone ?= light
  item Box { width 10; height 10; title ${ctx.tone} }
}
element "Screen" {
  item col { auto-layout column
    use Card as plain
    item wrap { context tone dark; use Card as dark }
  }
}`), "Screen")
	m := boxMap(screen)
	if l := m["col.plain.Box"].Lines; len(l) != 1 || l[0] != "light" {
		t.Errorf("plain title %v, want the default light", l)
	}
	if l := m["col.wrap.dark.Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("dark title %v, want the context of the place the use is written in", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextUseScope: the value travels through use, a block of the use is stronger than the definition,
// and ?= there fills only a key nothing above has (spec 5f).
func TestContextUseScope(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "scope.tenn", `element "Card" {
  kind Component
  context tone ?= light
  item Box { width 10; height 10; title ${ctx.tone} }
}
element "Screen" {
  context tone base
  item col { auto-layout column
    use Card as a
    use Card as b { context tone accent }
    item pen { context tone ink; use Card as c { context tone ?= soft } }
  }
}`), "Screen")
	m := boxMap(screen)
	for path, want := range map[string]string{
		"col.a.Box":     "base",
		"col.b.Box":     "accent",
		"col.pen.c.Box": "ink",
	} {
		if l := m[path].Lines; len(l) != 1 || l[0] != want {
			t.Errorf("%s title %v, want %s", path, l, want)
		}
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextSlot: the content of a slot is written at the use, and it inherits the context of the
// component it lands in.
func TestContextSlot(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "slot.tenn", `element "Panel" {
  kind Component
  context theme dark
  item Body { auto-layout column; slot S }
}
element "Screen" {
  use Panel as p { slot S { item X { width 10; height 10; title ${ctx.theme} } } }
}`), "Screen")
	if l := boxMap(screen)["p.Body.X"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("slot content title %v, want dark from the provider", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextNearest: the declaration closest up the tree wins.
func TestContextNearest(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "near.tenn", `element "Screen" {
  context tone outer
  item mid {
    context tone inner
    item Box { title ${ctx.tone} }
  }
  item Box { width 10; height 10; title ${ctx.tone} }
}`), "Screen")
	m := boxMap(screen)
	if l := m["mid.Box"].Lines; len(l) != 1 || l[0] != "inner" {
		t.Errorf("nested title %v, want inner", l)
	}
	if l := m["Box"].Lines; len(l) != 1 || l[0] != "outer" {
		t.Errorf("root title %v, want outer", l)
	}
}

// TestContextMissing: ctx of an unknown key is undefined, which is a bad expression with a position, not
// the text "undefined" in the scene (spec 5f).
func TestContextMissing(t *testing.T) {
	tree := paramsTree(t, "missing.tenn", `element "Screen" {
  context tone dark
  item A { width ${ctx.nope}; height 10 }
  item B { width 10; height 10; title ${ctx.nope} }
}`)
	screen, svg := paramsBuild(t, tree, "Screen")
	var found bool
	for _, w := range screen.Warnings {
		found = found || strings.Contains(w.Msg, "not evaluated")
	}
	if !found {
		t.Errorf("no diagnostic about the missing key, warnings %v", screen.Warnings)
	}
	if strings.Contains(svg, "undefined") {
		t.Errorf("undefined is drawn in the scene:\n%s", svg)
	}
}

// TestContextReservedName: export let ctx is a warning and no parameter; ctx stays the context object.
func TestContextReservedName(t *testing.T) {
	tree := paramsTree(t, "reserved.tenn", `element "C" {
  kind Component
  export let ctx = dark
  item Box { width 10; height 10 }
}
element "Screen" { use C as c }`)
	if ps := tree.File("reserved.tenn").Elems[0].Params; len(ps) != 0 {
		t.Errorf("params %v, want none: ctx is reserved", ps)
	}
	var found bool
	for _, d := range tree.Diagnostics() {
		found = found || strings.Contains(d.Msg, "ctx")
	}
	if !found {
		t.Errorf("no warning about export let ctx: %v", tree.Diagnostics())
	}
	screen, _ := paramsBuild(t, tree, "Screen")
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextStyleIsNoScope: context in a style block is an ordinary property of that style and does not
// change the context of the nodes that use it (spec 5f).
func TestContextStyleIsNoScope(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "style.tenn", `element "Card" {
  kind Component
  context tone dark
  styles {
    alt { context tone light; height 20 }
  }
  item Box { auto-layout row; use-style alt; title ${ctx.tone} }
}
element "Screen" { item col { auto-layout column; use Card as c } }`), "Screen")
	m := boxMap(screen)
	if l := m["col.c.Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("title %v, want dark: a style does not set context", l)
	}
	if h := m["col.c.Box"].H; h != 20 {
		t.Errorf("height %v, want 20 from the style", h)
	}
}

// TestContextAbsent: a file without context is built as before (spec 5f).
func TestContextAbsent(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "plain.tenn", componentDoc), "Screen")
	m := boxMap(screen)
	if h := m["col.save.Btn"].H; h != 48 {
		t.Errorf("save height %v, want 48", h)
	}
	if l := m["col.plain.Btn.Label"].Lines; len(l) != 1 || l[0] != "Button" {
		t.Errorf("plain label %v, want Button", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}
