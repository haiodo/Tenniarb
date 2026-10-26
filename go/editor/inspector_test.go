package editor

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

const inspSrc = `element "D" {
  styles {
    item { color blue }
  }
  item "A" {
    pos 0 0
    color red
    width ${100 + 20}
  }
  item "B" { pos 200 40 }
  link "A" "B" { display arrow }
}`

func TestPropReadsSelectionAndMixed(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	a, b := f.s.element.Items[0], f.s.element.Items[1]
	f.s.Select([]*model.DiagramItem{a})
	p := f.s.Prop("color")
	eq(t, "set", p.Set, true)
	eq(t, "text", p.Arg(0).Text, "red")
	eq(t, "pos", []string{f.s.Prop("pos").Arg(0).Text, f.s.Prop("pos").Arg(1).Text}, []string{"0.0", "0.0"})
	w := f.s.Prop("width")
	eq(t, "expr", w.Arg(0).Expr, true)
	eq(t, "expr source", w.Arg(0).Text, "${100 + 20}")
	eq(t, "expr value", w.Arg(0).Value, "120")

	f.s.Select([]*model.DiagramItem{a, b})
	p = f.s.Prop("color")
	eq(t, "mixed color", p.Mixed, true)
	eq(t, "mixed pos", f.s.Prop("pos").Mixed, true)
	eq(t, "unset", f.s.Prop("font-size").Set, false)

	f.s.Select(nil)
	eq(t, "element name", f.s.Prop("name").Arg(0).Text, "D")
}

func TestSetPropOneUndoStepOverSelection(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	a, b := f.s.element.Items[0], f.s.element.Items[1]
	f.s.Select([]*model.DiagramItem{a, b})
	if !f.s.SetProp("font-size", "30") {
		t.Fatal("refused")
	}
	eq(t, "text", strings.Count(f.s.Text(), "font-size 30"), 2)
	f.s.Undo()
	eq(t, "one undo", strings.Contains(f.s.Text(), "font-size"), false)
	f.s.Redo()
	f.s.SetProp("font-size")
	eq(t, "removed", strings.Contains(f.s.Text(), "font-size"), false)
	f.s.Undo()
	eq(t, "undo remove", strings.Count(f.s.Text(), "font-size 30"), 2)
}

func TestSetPropExpressionAndBadInput(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	b := f.s.element.Items[1]
	f.s.Select([]*model.DiagramItem{b})
	if !f.s.SetProp("width", "${40 * 2}") {
		t.Fatal("expression refused")
	}
	p := f.s.Prop("width")
	eq(t, "value", p.Arg(0).Value, "80")
	if f.s.SetProp("width", "${40 * ") || f.s.SetProp("color", "") {
		t.Error("a broken expression or an empty argument must be refused")
	}
	f.s.SetProp("marker", "x")
	contains(t, f.s.Text(), `marker "x"`)
}

func TestSetPropArgKeepsTheOtherArgument(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	a, b := f.s.element.Items[0], f.s.element.Items[1]
	f.s.Select([]*model.DiagramItem{a, b})
	f.s.SetPropArg("pos", 0, "50")
	eq(t, "x", pos(a, b), []float64{50, 0, 50, 40})
	f.s.Undo()
	eq(t, "undone", pos(a, b), []float64{0, 0, 200, 40})

	f.s.Select([]*model.DiagramItem{b})
	f.s.SetPropArg("shadow", 2, "7")
	contains(t, f.s.Text(), "shadow 0 0 7")
	f.s.SetPropArg("shadow", 2, "")
	contains(t, f.s.Text(), "shadow 0 0")
}

func TestSetPropOnElementAndLink(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	f.s.Select(nil)
	f.s.SetProp("name", "Renamed")
	eq(t, "element name", f.s.Element().Name, "Renamed")
	f.s.Undo()
	eq(t, "undone", f.s.Element().Name, "D")

	link := f.s.element.Items[2]
	f.s.Select([]*model.DiagramItem{link})
	eq(t, "display", f.s.Prop("display").Arg(0).Text, "arrow")
	f.s.SetProp("label", "uses")
	eq(t, "label", link.Name, "uses")
	f.s.SetProp("line-width", "2")
	contains(t, f.s.Text(), "line-width 2")
}

func TestStyleProp(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	eq(t, "item color", f.s.StyleProp("item", "color").Arg(0).Text, "blue")
	eq(t, "no line group", f.s.StyleProp("line", "color").Set, false)
	f.s.SetStyleProp("line", "line-width", "2")
	eq(t, "line width", f.s.StyleProp("line", "line-width").Arg(0).Text, "2")
	f.s.SetStyleProp("item", "color", "green")
	eq(t, "item color", f.s.StyleProp("item", "color").Arg(0).Text, "green")
	f.s.Undo()
	eq(t, "undo", f.s.StyleProp("item", "color").Arg(0).Text, "blue")
	f.s.SetStyleProp("item", "color")
	eq(t, "removed", f.s.StyleProp("item", "color").Set, false)
}

func TestEffective(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	a, b := f.s.element.Items[0], f.s.element.Items[1]
	f.s.Select([]*model.DiagramItem{a})
	eq(t, "own color", f.s.Effective("color"), "#ff0000")
	f.s.Select([]*model.DiagramItem{b})
	eq(t, "style color", f.s.Effective("color"), "#0000ff")
	eq(t, "default size", f.s.Effective("font-size"), "18.0")
	eq(t, "display", f.s.Effective("display"), "rect")
	f.s.Select([]*model.DiagramItem{f.s.element.Items[2]})
	eq(t, "link width", f.s.Effective("line-width"), "1.0")
}

func TestSetPropReadonly(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc, readonly: true})
	if f.s.SetProp("color", "red") || f.s.SetPropArg("pos", 0, "1") || f.s.SetStyleProp("item", "color", "red") {
		t.Error("readonly session must refuse")
	}
}

func TestDescriptionAndBody(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: inspSrc})
	eq(t, "none", f.s.Description(), "")
	f.s.SetDescription("about D")
	eq(t, "set", f.s.Description(), "about D")
	contains(t, f.s.Text(), `description "about D"`)
	f.s.Undo()
	eq(t, "undone", f.s.Description(), "")
	f.s.SetDescription("x")
	f.s.SetDescription("")
	eq(t, "removed", f.s.Description(), "")

	b := f.s.element.Items[1]
	eq(t, "no body", f.s.BodyOf(b), "")
	f.s.CommitEdit(b, EditBody, "line 1\nline 2")
	eq(t, "body", f.s.BodyOf(b), "line 1\nline 2")
	f.s.Undo()
	eq(t, "body undone", f.s.BodyOf(b), "")
}

func TestTrimFloat(t *testing.T) {
	eq(t, "measured", trimFloat(43.0068359375), "43.01")
	eq(t, "whole", trimFloat(30), "30.0")
}
