package editor

import (
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

const stylesSrc = `element "D" {
  styles { big { font-size 30 } }
  item "A" { pos 0 0 }
  item "B" {
    pos 200 0
    use-style big
  }
  link "A" "B" { display arrow }
}`

func newStyles(t *testing.T, readonly bool) (*fixture, *model.DiagramItem, *model.DiagramItem, *model.DiagramItem) {
	f := newFixture(t, fixtureOpts{src: stylesSrc, readonly: readonly})
	items := f.s.element.Items
	return f, items[0], items[1], items[2]
}

func TestQuickStylesFor(t *testing.T) {
	var link []string
	for _, q := range QuickStylesFor(model.ItemKindLink) {
		link = append(link, q.Prop)
	}
	if slices.Contains(link, "border-color") || slices.Contains(link, "width") || !slices.Contains(link, "color") {
		t.Errorf("link styles %v", link)
	}
	for _, q := range QuickStylesFor(model.ItemKindLink) {
		if q.Prop == "display" {
			eq(t, "link display", q.Options[:2], []string{"solid", "arrow"})
		}
	}
	for _, q := range QuickStylesFor(model.ItemKindItem) {
		if q.Prop == "display" {
			eq(t, "item display", q.Options[0], "rect")
		}
	}
	// The package list must stay intact for the next call.
	for _, q := range QuickStylesFor(model.ItemKindLink) {
		if q.Prop == "marker" {
			t.Error("markers are for items")
		}
	}
	eq(t, "labels", OptionLabel("red"), "🔴red")
	eq(t, "no label", OptionLabel("15"), "15")
}

func TestOptionNodes(t *testing.T) {
	eq(t, "marker", OptionNodes("marker", "🔥")[0].Kind, tenn.KindStringLit)
	eq(t, "layout", tenn.ToStr(OptionNodes("layout", "top right")[1], 0, false), "right")
	eq(t, "shadow", len(OptionNodes("shadow", "5 -5")), 2)
}

func TestStyleNames(t *testing.T) {
	f, _, _, _ := newStyles(t, false)
	eq(t, "names", f.s.StyleNames(), []string{"big"})
}

func TestApplyStyleWholeSelectionOneStep(t *testing.T) {
	f, a, b, _ := newStyles(t, false)
	f.s.Select([]*model.DiagramItem{a, b})
	f.s.ApplyStyle("big")
	eq(t, "changes", len(f.changes), 1) // b already has it
	contains(t, f.props(a), "use-style big")
	f.s.ApplyStyle("big")
	eq(t, "same style is a no-op", len(f.changes), 1)
	f.s.Undo()
	if strings.Contains(f.props(a), "use-style") {
		t.Error("undo removes the style")
	}
	contains(t, f.props(b), "use-style big")
}

func TestApplyStyleTwoItemsOneStep(t *testing.T) {
	f, a, b, _ := newStyles(t, false)
	f.s.Select([]*model.DiagramItem{a, b})
	f.s.ApplyStyle("other")
	eq(t, "changes", len(f.changes), 1)
	contains(t, f.props(a), "use-style other")
	contains(t, f.props(b), "use-style other")
	f.s.Undo()
	if strings.Contains(f.props(a), "use-style") {
		t.Error("a is back")
	}
	contains(t, f.props(b), "use-style big")
}

func TestApplyStyleReplaces(t *testing.T) {
	f, _, b, _ := newStyles(t, false)
	f.s.Select([]*model.DiagramItem{b})
	f.s.ApplyStyle("other")
	contains(t, f.props(b), "use-style other")
	if strings.Contains(f.props(b), "use-style big") {
		t.Error("big must be replaced")
	}
}

func TestDefineStyle(t *testing.T) {
	f, _, _, _ := newStyles(t, false)
	eq(t, "name", f.s.DefineStyle(), "new_style_2")
	eq(t, "names", f.s.StyleNames(), []string{"big", "new_style_2"})
	f.s.Undo()
	eq(t, "undone", f.s.StyleNames(), []string{"big"})
}

func TestSetQuickStyle(t *testing.T) {
	f, a, b, link := newStyles(t, false)
	f.s.Select([]*model.DiagramItem{a, b})
	f.s.SetQuickStyle("color", "red")
	eq(t, "two selected: nothing", len(f.changes), 0)
	f.s.Select([]*model.DiagramItem{a})
	f.s.SetQuickStyle("color", "red")
	f.s.SetQuickStyle("color", "blue")
	contains(t, f.props(a), "color blue")
	if strings.Contains(f.props(a), "color red") {
		t.Error("blue replaces red")
	}
	f.s.Undo()
	f.s.Undo()
	if strings.Contains(f.props(a), "color") {
		t.Error("two undo steps remove the color")
	}
	f.s.Select([]*model.DiagramItem{link})
	f.s.SetQuickStyle("display", "arrows")
	contains(t, f.props(link), "display arrows")
	f.s.Select([]*model.DiagramItem{a})
	f.s.SetQuickStyle("marker", "🔥")
	contains(t, f.props(a), `marker "🔥"`)
}

func TestEnableShadows(t *testing.T) {
	f, _, _, _ := newStyles(t, false)
	f.s.EnableShadows()
	f.s.EnableShadows()
	eq(t, "once", len(strings.Split(f.s.Text(), "shadow -5 -5 5")), 2)
	contains(t, f.s.Text(), "big")
	f.s.Undo()
	f.s.Undo()
	if strings.Contains(f.s.Text(), "shadow") {
		t.Error("undo removes the shadow")
	}
}

func TestReadonlyStyles(t *testing.T) {
	f, a, _, _ := newStyles(t, true)
	f.s.selection = []*model.DiagramItem{a}
	f.s.ApplyStyle("big")
	f.s.SetQuickStyle("color", "red")
	f.s.EnableShadows()
	eq(t, "define", f.s.DefineStyle(), "")
	eq(t, "changes", len(f.changes), 0)
}

func TestStyleWithoutBlockDoesNotPanic(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: "element \"D\" {\n  styles\n  item \"A\"\n}"})
	f.s.DefineStyle()
	f.s.EnableShadows()
}
