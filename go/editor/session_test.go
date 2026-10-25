package editor

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

func eq[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestHitTestBoxLineEmpty(t *testing.T) {
	f := newFixture(t)
	eq(t, "box", f.s.Hit(f.centre(f.a)), []*model.DiagramItem{f.a})
	link := f.s.element.Items[3]
	ca, cb := f.centre(f.a), f.centre(f.b)
	eq(t, "line", f.s.Hit(render.Point{X: (ca.X + cb.X) / 2, Y: (ca.Y + cb.Y) / 2}), []*model.DiagramItem{link})
	eq(t, "empty", len(f.s.Hit(render.Point{X: 5000, Y: 5000})), 0)
}

func TestClickToggleClearBand(t *testing.T) {
	f := newFixture(t)
	if !f.s.MouseDown(MouseEvent{P: f.centre(f.a)}) {
		t.Fatal("press on an item must be grabbed")
	}
	f.s.MouseUp(MouseEvent{P: f.centre(f.a)})
	eq(t, "click", f.s.Selection(), []*model.DiagramItem{f.a})
	f.click(f.centre(f.b), ModMeta)
	eq(t, "cmd-click adds", f.s.Selection(), []*model.DiagramItem{f.a, f.b})
	f.click(f.centre(f.a), ModMeta)
	eq(t, "cmd-click removes", f.s.Selection(), []*model.DiagramItem{f.b})

	// Empty space: not grabbed (the window pans), the selection survives the pan and goes on a click.
	far := render.Point{X: 5000, Y: 5000}
	if f.s.MouseDown(MouseEvent{P: far}) {
		t.Error("empty space must not be grabbed")
	}
	f.s.MouseMove(MouseEvent{P: render.Point{X: 5100, Y: 5000}})
	f.s.MouseUp(MouseEvent{P: render.Point{X: 5100, Y: 5000}})
	eq(t, "pan keeps", len(f.s.Selection()), 1)
	f.s.MouseDown(MouseEvent{P: far})
	f.s.MouseUp(MouseEvent{P: far})
	eq(t, "click on empty clears", len(f.s.Selection()), 0)

	bc := f.s.scene.Drawables[f.c].SelectorBounds()
	from := render.Point{X: bc.X - 20, Y: bc.Y - 20}
	to := render.Point{X: bc.X + bc.Width + 20, Y: bc.Y + bc.Height + 20}
	f.s.MouseDown(MouseEvent{P: from, Mods: ModShift})
	f.s.MouseMove(MouseEvent{P: to, Mods: ModShift})
	if !slices.Contains(f.s.Selection(), f.c) || slices.Contains(f.s.Selection(), f.a) {
		t.Errorf("band selection = %v", f.s.Selection())
	}
	if f.s.Band() == nil {
		t.Error("band must be set while dragging")
	}
	f.s.MouseUp(MouseEvent{P: to, Mods: ModShift})
	if !slices.Contains(f.s.Selection(), f.c) || f.s.Band() != nil {
		t.Errorf("after band: %v, %v", f.s.Selection(), f.s.Band())
	}
	all := f.s.itemsInRect(render.Rect{X: -1e4, Y: -1e4, Width: 2e4, Height: 2e4})
	eq(t, "all in rect", len(all), 5)
}

func TestBandReleasedWithoutShiftRestoresSelection(t *testing.T) {
	f := newFixture(t)
	f.click(f.centre(f.b), 0)
	bc := f.s.scene.Drawables[f.c].SelectorBounds()
	f.s.MouseDown(MouseEvent{P: render.Point{X: bc.X - 20, Y: bc.Y - 20}, Mods: ModShift})
	eq(t, "band start clears", len(f.s.Selection()), 0)
	f.s.MouseMove(MouseEvent{P: render.Point{X: bc.X + 50, Y: bc.Y + 50}, Mods: ModShift})
	f.s.MouseUp(MouseEvent{P: render.Point{X: bc.X + 50, Y: bc.Y + 50}})
	eq(t, "restored", f.s.Selection(), []*model.DiagramItem{f.b})
}

func TestShiftClickClearsSelection(t *testing.T) {
	f := newFixture(t)
	f.click(f.centre(f.a), 0)
	f.click(f.centre(f.b), ModShift)
	eq(t, "Swift shift-click", len(f.s.Selection()), 0)
}

func TestDragIsOneUndoStep(t *testing.T) {
	f := newFixture(t)
	f.click(f.centre(f.a), 0)
	f.click(f.centre(f.b), ModMeta)
	eq(t, "selection", f.s.Selection(), []*model.DiagramItem{f.a, f.b})
	p := f.centre(f.a)
	f.s.MouseDown(MouseEvent{P: p})
	f.s.MouseMove(MouseEvent{P: render.Point{X: p.X + 20, Y: p.Y + 10}})
	f.s.MouseUp(MouseEvent{P: render.Point{X: p.X + 50, Y: p.Y + 30}})
	eq(t, "moved", pos(f.a, f.b), []float64{50, 30, 250, 30})
	eq(t, "changes", len(f.changes), 1)
	contains(t, f.changes[0], "pos 50.0 30.0")
	f.s.Undo()
	eq(t, "undone", pos(f.a, f.b), []float64{0, 0, 200, 0})
	eq(t, "changes after undo", len(f.changes), 2)
	if f.s.UndoManager().CanUndo() {
		t.Error("one step only")
	}
	f.s.Redo()
	eq(t, "redone", pos(f.a, f.b), []float64{50, 30, 250, 30})
}

func TestClickAndHomeDragLeaveNoUndo(t *testing.T) {
	f := newFixture(t)
	p := f.centre(f.a)
	f.click(p, 0)
	f.s.MouseDown(MouseEvent{P: p})
	f.s.MouseMove(MouseEvent{P: render.Point{X: p.X + 40, Y: p.Y}})
	f.s.MouseUp(MouseEvent{P: p})
	if f.s.UndoManager().CanUndo() || len(f.changes) != 0 {
		t.Errorf("undo %v, changes %d", f.s.UndoManager().CanUndo(), len(f.changes))
	}
}

func TestAttachedLinesFollowDrag(t *testing.T) {
	f := newFixture(t)
	ab0, bc0 := f.ends(3), f.ends(4)
	p := f.centre(f.a)
	f.s.MouseDown(MouseEvent{P: p})
	f.s.MouseMove(MouseEvent{P: render.Point{X: p.X, Y: p.Y + 100}})
	live := f.ends(3)
	if live == ab0 {
		t.Error("A-B must follow A")
	}
	eq(t, "B-C untouched", f.ends(4), bc0)
	if len(f.redraws) == 0 || len(f.redraws[len(f.redraws)-1].Moved) == 0 {
		t.Errorf("drag preview must report the moved drawables: %v", f.redraws)
	}
	f.s.MouseUp(MouseEvent{P: render.Point{X: p.X, Y: p.Y + 100}})
	eq(t, "after commit", f.ends(3), live)
}

func TestDragLinkChangesControl(t *testing.T) {
	f := newFixture(t)
	link := f.s.element.Items[3]
	r := f.s.scene.Drawables[link].Bounds()
	p := render.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
	f.s.MouseDown(MouseEvent{P: p})
	eq(t, "selection", f.s.Selection(), []*model.DiagramItem{link})
	f.s.MouseMove(MouseEvent{P: render.Point{X: p.X + 30, Y: p.Y + 40}})
	f.s.MouseUp(MouseEvent{P: render.Point{X: p.X + 30, Y: p.Y + 40}})
	if math.Abs(link.X-30) > 1e-9 || math.Abs(link.Y-40) > 1e-9 {
		t.Errorf("control = %v %v, want 30 40", link.X, link.Y)
	}
}

func TestReadonlyPointer(t *testing.T) {
	f := newFixture(t, fixtureOpts{readonly: true})
	if f.s.MouseDown(MouseEvent{P: f.centre(f.a)}) {
		t.Error("readonly must not grab")
	}
	f.s.MouseMove(MouseEvent{P: render.Point{X: 100, Y: 100}})
	f.s.MouseUp(MouseEvent{P: render.Point{X: 100, Y: 100}})
	eq(t, "selection", len(f.s.Selection()), 0)
	eq(t, "x", f.a.X, 0.0)
	eq(t, "changes", len(f.changes), 0)
}

func TestRename(t *testing.T) {
	f := newFixture(t)
	tg, ok := f.s.EditTarget(f.a, EditName)
	if !ok || tg.Text != "A" {
		t.Fatalf("target %v %v", tg, ok)
	}
	f.s.CommitEdit(f.a, EditName, "A")
	eq(t, "unchanged is a no-op", len(f.changes), 0)
	f.s.CommitEdit(f.a, EditName, "Alpha\nline")
	eq(t, "name", f.a.Name, "Alpha\nline")
	eq(t, "changes", len(f.changes), 1)
	contains(t, f.changes[0], "Alpha")
	f.s.Undo()
	eq(t, "undone", f.a.Name, "A")
	if f.s.UndoManager().CanUndo() {
		t.Error("one step only")
	}
}

func TestBodyCreatedEditedUndone(t *testing.T) {
	f := newFixture(t)
	text := func() string { tg, _ := f.s.EditTarget(f.a, EditBody); return tg.Text }
	f.s.CommitEdit(f.a, EditBody, "one")
	eq(t, "body", text(), "one")
	f.s.CommitEdit(f.a, EditBody, "one\ntwo")
	eq(t, "body 2", text(), "one\ntwo")
	eq(t, "changes", len(f.changes), 2)
	if f.s.scene.Drawables[f.a].SelectorBounds().Height <= 0 {
		t.Error("item must have a height")
	}
	f.s.Undo()
	eq(t, "undo 1", text(), "one")
	f.s.Undo()
	eq(t, "undo 2", text(), "")
	if f.a.Properties.Get("body") != nil {
		t.Error("body must be gone")
	}
}

func TestBodyBlockAndEscapedNewline(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: `element "D" {
  item "A" {
    body {
      text "x"
      color red
    }
  }
  item "B" {
    body {
      color red
    }
  }
}`})
	a, b := f.s.element.Items[0], f.s.element.Items[1]
	f.s.CommitEdit(a, EditBody, "new")
	eq(t, "block text", BodyText(a), "new")
	contains(t, f.props(a), "color red")
	n := len(f.changes)
	f.s.CommitEdit(b, EditBody, "ignored")
	eq(t, "block without text is left alone", len(f.changes), n)
	f.s.CommitEdit(f.s.element.Items[0], EditBody, `a\nb`)
	tg, _ := f.s.EditTarget(a, EditBody)
	eq(t, "text", tg.Text, "a\nb")
}

func TestCopyText(t *testing.T) {
	f := newFixture(t)
	eq(t, "nothing", f.s.CopyText(), "")
	f.s.Select([]*model.DiagramItem{f.a, f.b, f.s.element.Items[3]})
	text := f.s.CopyText()
	contains(t, text, `item "A"`)
	contains(t, text, `link "A" "B"`)
	if strings.Contains(text, `item "C"`) {
		t.Error("C must not be copied")
	}
	f.s.Select([]*model.DiagramItem{f.c})
	if strings.Contains(f.s.CopyText(), "link") {
		t.Error("a lone item has no links")
	}
}

func TestPasteRemapsLinks(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a, f.b, f.s.element.Items[3]})
	text := f.s.CopyText()
	if f.s.Paste(`item "broken {`) || f.s.Paste("hello") {
		t.Error("garbage must be ignored")
	}
	eq(t, "changes", len(f.changes), 0)
	if !f.s.Paste(text) {
		t.Fatal("paste failed")
	}
	eq(t, "items", len(f.s.element.Items), 8)
	eq(t, "changes", len(f.changes), 1)
	sel := f.s.Selection()
	if len(sel) != 3 || sel[0] == f.a || sel[1] == f.b {
		t.Fatalf("selection %v", sel)
	}
	eq(t, "kinds", []model.ItemKind{sel[0].Kind, sel[1].Kind, sel[2].Kind}, []model.ItemKind{"Item", "Item", "Link"})
	if sel[2].Source != sel[0] || sel[2].Target != sel[1] {
		t.Error("link must join the pasted items")
	}
	if _, ok := f.s.scene.Drawables[sel[2]]; !ok {
		t.Error("link must be drawn")
	}
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
	eq(t, "selection", len(f.s.Selection()), 0)
	if f.s.UndoManager().CanUndo() {
		t.Error("one step only")
	}
}

func TestCutAndDelete(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.b})
	contains(t, f.s.Cut(), `item "B"`)
	eq(t, "items", len(f.s.element.Items), 2) // A and C stay, both links went with B
	eq(t, "changes", len(f.changes), 1)
	eq(t, "selection", len(f.s.Selection()), 0)
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
	f.s.Select([]*model.DiagramItem{f.s.element.Items[3]})
	f.key("Backspace", 0)
	eq(t, "link deleted", len(f.s.element.Items), 4)
	f.s.Undo()
	eq(t, "link back", len(f.s.element.Items), 5)
}

func TestDuplicate(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.b})
	f.key("d", ModMeta)
	eq(t, "items", len(f.s.element.Items), 7) // B copy plus the A->B link cloned onto it
	sel := f.s.Selection()
	nb, nl := sel[0], sel[1]
	eq(t, "copy", []any{nb.Name, nb.X, nb.Y}, []any{"B", 275.0, 0.0})
	if nl.Target != nb || nl.Source != f.a {
		t.Error("the link of the copy ends at the copy")
	}
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
}

func TestReadonlyEdits(t *testing.T) {
	f := newFixture(t, fixtureOpts{readonly: true})
	if _, ok := f.s.EditTarget(f.a, EditName); ok {
		t.Error("readonly has no edit target")
	}
	f.s.CommitEdit(f.a, EditName, "X")
	if f.s.Paste(`item "X" { pos 1 1 }`) {
		t.Error("readonly paste")
	}
	f.s.selection = []*model.DiagramItem{f.a}
	f.s.DeleteSelection()
	f.s.Duplicate()
	eq(t, "items", len(f.s.element.Items), 5)
	eq(t, "name", f.a.Name, "A")
	eq(t, "changes", len(f.changes), 0)
	contains(t, f.s.CopyText(), `item "A"`)
}

func TestPropsTargetAndText(t *testing.T) {
	f := newFixture(t)
	eq(t, "target", f.s.PropsTarget(), Target{Element: f.s.element})
	if got := f.s.PropsText(Target{Element: f.s.element}); !strings.HasPrefix(got, `name "D"`) {
		t.Errorf("element props %q", got)
	}
	f.click(f.centre(f.a), 0)
	eq(t, "item target", f.s.PropsTarget(), Target{Item: f.a})
	contains(t, f.props(f.a), "pos 0.0 0.0")
}

func TestApplyPropsIsOneUndoStep(t *testing.T) {
	f := newFixture(t)
	if !f.s.ApplyProps(Target{Item: f.a}, "name \"A\"\npos 0 0\ncolor red\nfontSize 20") {
		t.Fatal("apply failed")
	}
	contains(t, f.props(f.a), "color red")
	contains(t, f.props(f.a), "fontSize 20")
	eq(t, "changes", len(f.changes), 1)
	f.s.Undo()
	if strings.Contains(f.props(f.a), "color red") {
		t.Error("undo must remove color")
	}
	eq(t, "changes after undo", len(f.changes), 2)
	f.s.Redo()
	contains(t, f.props(f.a), "color red")
}

func TestApplyPropsRefusals(t *testing.T) {
	f := newFixture(t)
	if f.s.ApplyProps(Target{Item: f.a}, "color {") {
		t.Error("parse errors must be refused")
	}
	if err := f.s.store.RemoveItems(f.s.element, []*model.DiagramItem{f.a}, f.s.um, func() {}); err != nil {
		t.Fatal(err)
	}
	n := len(f.changes)
	if f.s.ApplyProps(Target{Item: f.a}, `name "A"`) {
		t.Error("removed target must be refused")
	}
	eq(t, "changes", len(f.changes), n)
	ro := newFixture(t, fixtureOpts{readonly: true})
	if ro.s.ApplyProps(Target{Item: ro.a}, `name "X"`) {
		t.Error("readonly")
	}
}

func TestPropsValues(t *testing.T) {
	m := model.ReadTenn(testSrc)
	ctx := exec.NewExecutionContext(exec.Options{})
	s, err := New(m.Elements[0], Options{Render: render.Options{Measurer: gfx.EstimateText{}, Exec: ctx}})
	if err != nil {
		t.Fatal(err)
	}
	a := s.element.Items[0]
	values := s.PropsValues(Target{Item: a}, "name \"A\"\npos 0 0\nfontSize $(10 + 5)")
	eq(t, "line 2", values[2], "15")
	eq(t, "does not parse", len(s.PropsValues(Target{Item: a}, "fontSize $(1 +")), 0)
}

func TestArrowsMoveBySnappedStep(t *testing.T) {
	f := newFixture(t)
	f.a.X = 3
	f.s.Select([]*model.DiagramItem{f.a, f.b})
	f.key("ArrowRight", 0)
	eq(t, "right", pos(f.a, f.b), []float64{5, 0, 205, 0})
	f.key("ArrowDown", 0)
	eq(t, "down", pos(f.a, f.b), []float64{5, -5, 205, -5})
	eq(t, "changes", len(f.changes), 2)
	f.s.Undo()
	eq(t, "undo", pos(f.a, f.b), []float64{5, 0, 205, 0})
	f.key("ArrowLeft", 0)
	f.key("ArrowLeft", 0)
	eq(t, "toward zero", f.a.X, -5.0) // Swift drops the remainder of the truncated value
}

func TestResizeByGridStep(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	width := func() float64 {
		var w float64
		n := f.a.Properties.Get("width")
		if n == nil {
			t.Fatal("no width")
		}
		w, _ = n.GetFloat(1)
		return w
	}
	f.key("ArrowRight", ModShift)
	w := width()
	eq(t, "changes", len(f.changes), 1)
	f.key("ArrowRight", ModMeta) // Cmd keeps the centre
	if math.Abs(width()-(w+5)) > 1e-3 {
		t.Errorf("width %v, want %v", width(), w+5)
	}
	eq(t, "x", f.a.X, -2.5)
	f.key("ArrowDown", ModShift)
	contains(t, f.props(f.a), "height ")
	eq(t, "changes", len(f.changes), 3)
	f.s.Undo()
	if strings.Contains(f.props(f.a), "height ") {
		t.Error("undo must remove height")
	}
	f.key("ArrowUp", ModMeta)
	f.key("ArrowUp", ModMeta)
	h, _ := f.a.Properties.Get("height").GetFloat(1)
	if h < 10 {
		t.Errorf("height %v under the minimum", h)
	}
	f.s.Select([]*model.DiagramItem{f.a, f.b})
	n := len(f.changes)
	f.s.ResizeBy("ArrowRight", false)
	eq(t, "two selected: nothing", len(f.changes), n)
}

func TestTabLinkedItem(t *testing.T) {
	f := newFixture(t)
	f.s.ApplyProps(Target{Item: f.a}, "name \"A\"\npos 0 0\ncolor red")
	f.s.Select([]*model.DiagramItem{f.a})
	f.key("Tab", ModAlt) // Option+Tab copies the style properties
	items := f.s.element.Items
	item, link := items[len(items)-2], items[len(items)-1]
	eq(t, "link", []any{link.Kind, link.Source, link.Target, f.s.Selection()}, []any{model.ItemKindLink, f.a, item, []*model.DiagramItem{item}})
	contains(t, f.props(item), "color red")
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
	f.s.Select([]*model.DiagramItem{f.a})
	f.key("Tab", 0)
	if strings.Contains(f.props(f.s.element.Items[len(f.s.element.Items)-2]), "color red") {
		t.Error("plain Tab copies no properties")
	}
	f.s.Select([]*model.DiagramItem{f.s.element.Items[3]}) // a link: nothing
	n := len(f.s.element.Items)
	f.key("Tab", 0)
	eq(t, "link selected", len(f.s.element.Items), n)
}

func TestSelectAll(t *testing.T) {
	f := newFixture(t)
	f.key("a", ModMeta)
	eq(t, "all", len(f.s.Selection()), 5)
	f.key("a", ModMeta|ModShift)
	eq(t, "items", len(f.s.Selection()), 3)
	f.s.SelectAll(model.ItemKindLink)
	eq(t, "links", len(f.s.Selection()), 2)
	f.s.Select(nil)
	eq(t, "none", len(f.s.Selection()), 0)
	ro := newFixture(t, fixtureOpts{readonly: true})
	ro.s.SelectAll("")
	eq(t, "readonly", len(ro.s.Selection()), 0)
}

func TestPasteAsItem(t *testing.T) {
	f := newFixture(t)
	f.s.PasteAsItem("hello\nworld")
	item := f.s.element.Items[len(f.s.element.Items)-1]
	eq(t, "item", []any{item.Name, f.s.Selection(), len(f.changes)}, []any{"pasted 1", []*model.DiagramItem{item}, 1})
	contains(t, f.props(item), "text")
	contains(t, f.props(item), "${text}")
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
}

func TestPasteAsItemSet(t *testing.T) {
	f := newFixture(t)
	f.s.PasteAsItemSet("one\n\ntwo\nthree")
	added := f.s.element.Items[5:]
	var kinds []string
	var ys []float64
	for _, i := range added {
		kinds = append(kinds, string(i.Kind)+i.Name)
		if i.Kind == model.ItemKindItem {
			ys = append(ys, i.Y)
		}
	}
	eq(t, "kinds", kinds, []string{"Itemone", "Link", "Itemtwo", "Link", "Itemthree"})
	eq(t, "ys", ys, []float64{0, -35, -70})
	l1, l2 := added[1], added[3]
	eq(t, "links", []any{l1.Source, l2.Source, l1.Target.Name, l2.Target.Name}, []any{added[0], added[0], "two", "three"})
	contains(t, f.props(l1), "display arrow")
	eq(t, "changes", len(f.changes), 1)
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
	f.s.PasteAsItemSet("\n")
	eq(t, "empty paste", len(f.changes), 2)
}

func TestAlign(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a, f.b, f.c})
	right := func(i *model.DiagramItem) float64 { return i.X + f.s.scene.Drawables[i].Bounds().Width }
	f.s.Align("trailing")
	if math.Abs(right(f.a)-right(f.b)) > 1e-9 || math.Abs(right(f.b)-right(f.c)) > 1e-9 {
		t.Errorf("trailing: %v %v %v", right(f.a), right(f.b), right(f.c))
	}
	f.s.Undo()
	f.s.Align("leading")
	eq(t, "leading", []float64{f.a.X, f.b.X, f.c.X}, []float64{0, 0, 0})
	f.s.Undo()
	f.s.Align("top")
	eq(t, "top", []float64{f.a.Y, f.b.Y, f.c.Y}, []float64{0, 0, 0})
	f.s.Undo()
	f.s.Align("bottom")
	bottom := func(i *model.DiagramItem) float64 { return i.Y - f.s.scene.Drawables[i].Bounds().Height }
	// The target edge is rounded, as in Swift.
	if math.Abs(bottom(f.a)-bottom(f.c)) > 1 || math.Abs(f.c.Y+150) > 1 {
		t.Errorf("bottom: %v %v %v", bottom(f.a), bottom(f.c), f.c.Y)
	}
	eq(t, "changes", len(f.changes), 7)
}

func TestOrder(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	f.s.Order(true)
	eq(t, "forward", f.s.element.Items[len(f.s.element.Items)-1], f.a)
	f.s.Order(false)
	eq(t, "backward", f.s.element.Items[0], f.a)
	f.s.Undo()
	eq(t, "undo", f.s.element.Items[len(f.s.element.Items)-1], f.a)
	f.s.Select([]*model.DiagramItem{f.a, f.b})
	f.s.Order(true) // two selected: nothing
	eq(t, "two", f.s.element.Items[len(f.s.element.Items)-1], f.a)
}

func TestEditValue(t *testing.T) {
	f := newFixture(t)
	text := func() string { tg, _ := f.s.EditTarget(f.a, EditValue); return tg.Text }
	eq(t, "empty", text(), "")
	f.s.CommitEdit(f.a, EditValue, "42")
	if !strings.Contains(f.props(f.a), "value 42") {
		t.Errorf("props %q", f.props(f.a))
	}
	eq(t, "text", text(), "42")
	f.s.CommitEdit(f.a, EditValue, "3.5x")
	contains(t, f.props(f.a), "value 3.5x")
	f.s.CommitEdit(f.a, EditValue, "12 apples")
	contains(t, f.props(f.a), `value "12 apples"`)
	f.s.CommitEdit(f.a, EditValue, "12 apples")
	eq(t, "changes", len(f.changes), 3)
	f.s.Undo()
	contains(t, f.props(f.a), "value 3.5x")
	f.s.ApplyProps(Target{Item: f.a}, "name \"A\"\npos 0 0\nfield-name total\ntotal 7")
	eq(t, "field", text(), "7")
	f.s.CommitEdit(f.a, EditValue, "8")
	contains(t, f.props(f.a), "total 8")
}

func TestTestLayout(t *testing.T) {
	f := newFixture(t)
	before := pos(f.a, f.b, f.c)
	f.s.TestLayout(render.Rect{X: -500, Y: -400, Width: 1000, Height: 800})
	if reflect.DeepEqual(pos(f.a, f.b, f.c), before) {
		t.Error("positions must change")
	}
	eq(t, "changes", len(f.changes), 1)
	f.s.Undo()
	eq(t, "undone", pos(f.a, f.b, f.c), before)
}

func TestReadonlyCommands(t *testing.T) {
	f := newFixture(t, fixtureOpts{readonly: true})
	f.s.selection = []*model.DiagramItem{f.a}
	f.key("ArrowRight", 0)
	f.key("ArrowRight", ModShift)
	f.key("Tab", 0)
	f.s.Align("leading")
	f.s.Order(true)
	f.s.PasteAsItem("x")
	f.s.PasteAsItemSet("x")
	f.s.TestLayout(render.Rect{Width: 100, Height: 100})
	f.s.PasteImage("a.png", "QUJD")
	f.s.AttachImage("a.png", "QUJD")
	eq(t, "changes", len(f.changes), 0)
}

func TestCtrlDragAddsLink(t *testing.T) {
	f := newFixture(t)
	f.s.MouseDown(MouseEvent{P: f.centre(f.a), Mods: ModCtrl})
	f.s.MouseMove(MouseEvent{P: f.centre(f.c), Mods: ModCtrl})
	if f.s.Scene().LineTo == nil {
		t.Error("preview line expected")
	}
	if pic := f.s.Overlay(); pic.Len() == 0 {
		t.Error("overlay must draw the preview")
	}
	f.s.MouseUp(MouseEvent{P: f.centre(f.c), Mods: ModCtrl})
	if f.s.Scene().LineTo != nil {
		t.Error("preview must be gone")
	}
	eq(t, "links", f.links(), 3)
	l := f.s.element.Items[len(f.s.element.Items)-1]
	if l.Source != f.a || l.Target != f.c {
		t.Errorf("link %v -> %v", l.Source.Name, l.Target.Name)
	}
	eq(t, "changes", len(f.changes), 1)
	f.s.Undo()
	eq(t, "undone", f.links(), 2)
	if f.s.UndoManager().CanUndo() {
		t.Error("one step only")
	}
}

func TestCtrlDragDroppedElsewhereAddsNothing(t *testing.T) {
	f := newFixture(t)
	far := render.Point{X: 5000, Y: 5000}
	f.s.MouseDown(MouseEvent{P: f.centre(f.a), Mods: ModCtrl})
	f.s.MouseMove(MouseEvent{P: far})
	f.s.MouseUp(MouseEvent{P: far})
	// On the source itself (Swift would add a self-link).
	f.s.MouseDown(MouseEvent{P: f.centre(f.a), Mods: ModCtrl})
	f.s.MouseMove(MouseEvent{P: render.Point{X: f.centre(f.a).X + 3, Y: f.centre(f.a).Y}})
	f.s.MouseUp(MouseEvent{P: f.centre(f.a)})
	// Cancelled.
	f.s.MouseDown(MouseEvent{P: f.centre(f.a), Mods: ModCtrl})
	f.s.MouseMove(MouseEvent{P: f.centre(f.c)})
	f.s.MouseUp(MouseEvent{P: f.centre(f.c), Cancel: true})
	eq(t, "items", len(f.s.element.Items), 5)
	eq(t, "changes", len(f.changes), 0)
	if f.s.Scene().LineTo != nil {
		t.Error("preview must be gone")
	}
}

func TestOptionDragMovesLinkedItems(t *testing.T) {
	f := newFixture(t)
	p := f.centre(f.b)
	f.s.MouseDown(MouseEvent{P: p, Mods: ModAlt})
	eq(t, "selection", f.s.Selection(), []*model.DiagramItem{f.b, f.c})
	f.s.MouseMove(MouseEvent{P: render.Point{X: p.X + 10, Y: p.Y + 10}})
	f.s.MouseUp(MouseEvent{P: render.Point{X: p.X + 10, Y: p.Y + 10}})
	eq(t, "positions", pos(f.a, f.b, f.c), []float64{0, 0, 210, 10, 10, -140})
	if !f.s.UndoManager().CanUndo() {
		t.Error("must be undoable")
	}
	f.s.Undo()
	eq(t, "undone", []float64{f.b.X, f.c.X}, []float64{200, 0})
}

func TestPCKeys(t *testing.T) {
	f := newFixture(t, fixtureOpts{pc: true})
	f.click(f.centre(f.a), 0)
	f.click(f.centre(f.b), ModCtrl) // Ctrl toggles
	eq(t, "toggle", f.s.Selection(), []*model.DiagramItem{f.a, f.b})
	f.s.Select(nil)
	p := f.centre(f.b)
	f.s.MouseDown(MouseEvent{P: p, Mods: ModAlt}) // Alt drags with the linked items
	eq(t, "alt", f.s.Selection(), []*model.DiagramItem{f.b, f.c})
	f.s.MouseUp(MouseEvent{P: p})
	f.s.Select(nil)
	f.s.MouseDown(MouseEvent{P: f.centre(f.a), Mods: ModCtrl | ModAlt})
	f.s.MouseMove(MouseEvent{P: f.centre(f.c)})
	f.s.MouseUp(MouseEvent{P: f.centre(f.c)})
	eq(t, "links", f.links(), 3)
}

func TestOperate(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a, f.b})
	if !f.s.Operate("pos 10 20\ncolor red\n-display") {
		t.Fatal("operate failed")
	}
	eq(t, "changes", len(f.changes), 1)
	for _, i := range []*model.DiagramItem{f.a, f.b} {
		eq(t, "pos", []float64{i.X, i.Y}, []float64{10, 20})
		contains(t, f.props(i), "color red")
	}
	f.s.Undo()
	eq(t, "undone", pos(f.a, f.b), []float64{0, 0, 200, 0})
	if strings.Contains(f.props(f.a), "color") {
		t.Error("undo must remove color")
	}
}

func TestOperateRefusals(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	f.s.Operate("color red")
	if !f.s.Operate("-color") || strings.Contains(f.props(f.a), "color") {
		t.Error("-color must remove the property")
	}
	n := len(f.changes)
	if f.s.Operate("color {") {
		t.Error("parse error")
	}
	if !f.s.Operate("-nothing") {
		t.Error("removing nothing is accepted")
	}
	f.s.Select(nil)
	if f.s.Operate("color red") {
		t.Error("nothing selected")
	}
	eq(t, "changes", len(f.changes), n)
	if newFixture(t, fixtureOpts{readonly: true}).s.Operate("color red") {
		t.Error("readonly")
	}
}

func TestOverlappingItemsCycle(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: "element \"D\" { item \"A\" { pos 0 0 }\n item \"B\" { pos 0 0 } }"})
	p := f.centre(f.s.element.Items[0])
	click := func() *model.DiagramItem {
		f.s.MouseDown(MouseEvent{P: p})
		f.s.MouseUp(MouseEvent{P: p})
		return f.s.Selection()[0]
	}
	first := click()
	eq(t, "selected one is kept", click(), first)
	f.s.Select(nil)
	if click() == first {
		t.Error("the next click must pick the other item")
	}
	f.s.Select(nil)
	eq(t, "and back", click(), first)
}

func TestPasteImageNewItem(t *testing.T) {
	f := newFixture(t)
	f.s.PasteImage("a.png", "QUJD")
	item := f.s.Selection()[0]
	eq(t, "name", item.Name, "Untitled 1")
	contains(t, f.s.Text(), "QUJD")
	title := item.Properties.Get("title").GetChild(1)
	eq(t, "title", title.Token.Literal, "@(a.png|96)\n${name}")
	eq(t, "changes", len(f.changes), 1)
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Items), 5)
}

func TestPasteAndAttachImageToSelected(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	f.s.PasteImage("a.png", "QUJD")
	if n, _ := f.a.Properties.Get("image").GetIdent(1); n != "a.png" {
		t.Errorf("image %q", n)
	}
	f.s.AttachImage("b.jpg", "REVG")
	eq(t, "changes", len(f.changes), 2)
	contains(t, f.s.Text(), "REVG")
	f.s.Undo()
	if strings.Contains(f.s.Text(), "REVG") || !strings.Contains(f.s.Text(), "QUJD") {
		t.Error("undo removes the last image only")
	}
}

func TestElementCommands(t *testing.T) {
	f := newFixture(t)
	el := f.s.element
	text := f.s.CopyElement(el)
	contains(t, text, `element "D"`)
	if f.s.PasteElements(el, "hello") || f.s.PasteElements(el, `element "broken {`) {
		t.Error("garbage must be refused")
	}
	eq(t, "changes", len(f.changes), 0)
	if !f.s.PasteElements(el, text) {
		t.Fatal("paste failed")
	}
	eq(t, "children", len(el.Elements), 1)
	eq(t, "items", len(el.Elements[0].Items), 5)
	eq(t, "changes", len(f.changes), 1)
	f.s.Undo()
	eq(t, "undone", len(el.Elements), 0)

	root := &f.s.root.Element
	f.s.PasteElements(root, text)
	eq(t, "root children", len(root.Elements), 2)
	pasted := root.Elements[1]
	eq(t, "cut", f.s.CutElement(pasted), f.s.CopyElement(pasted))
	eq(t, "after cut", len(root.Elements), 1)
	ro := newFixture(t, fixtureOpts{readonly: true})
	if ro.s.PasteElements(ro.s.element, text) {
		t.Error("readonly")
	}
}

func TestRemoveCurrentElementFallsBack(t *testing.T) {
	f := newFixture(t)
	elements := 0
	f.s.opts.OnElement = func() { elements++ }
	f.s.RemoveElement(f.s.element)
	if f.s.element != &f.s.root.Element || elements != 1 {
		t.Errorf("element %v, notified %d", f.s.element.Name, elements)
	}
	f.s.Undo()
	eq(t, "root kept", len(f.s.root.Elements), 1)
}

func TestMoveAndRenameElement(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: "element \"P\" {\n element \"X\" { }\n element \"Y\" { }\n element \"Z\" { }\n}"})
	m := f.s.root
	p := m.Elements[0]
	x, y, z := p.Elements[0], p.Elements[1], p.Elements[2]
	f.s.MoveElement(x, p, 3) // down inside one parent: lands one place earlier
	eq(t, "order", []*model.Element{p.Elements[0], p.Elements[1], p.Elements[2]}, []*model.Element{y, z, x})
	f.s.MoveElement(p, x, 0) // into its own subtree: a copy of the diagram
	eq(t, "copy", len(x.Elements), 1)
	eq(t, "still there", p.Parent != nil, true)
	f.s.RenameElement(y, "Why")
	eq(t, "renamed", y.Name, "Why")
	f.s.Undo()
	eq(t, "unrenamed", y.Name, "Y")
}

func TestEditingState(t *testing.T) {
	var asked []EditMode
	f := newFixture(t)
	f.s.opts.OnEdit = func(_ *model.DiagramItem, m EditMode) { asked = append(asked, m) }
	f.s.Select([]*model.DiagramItem{f.a})
	f.key("Enter", 0)
	f.key("Enter", ModShift)
	f.key("Enter", ModAlt)
	eq(t, "modes", asked, []EditMode{EditName, EditBody, EditValue})
	// Double click on one item asks for its name.
	f.s.Select(nil)
	f.s.MouseDown(MouseEvent{P: f.centre(f.b), Clicks: 2})
	eq(t, "double click", asked[len(asked)-1], EditName)
	eq(t, "selected", f.s.Selection(), []*model.DiagramItem{f.b})

	if _, ok := f.s.BeginEdit(f.b, EditName); !ok {
		t.Fatal("BeginEdit")
	}
	eq(t, "editing", f.s.Editing(), f.b)
	if f.key("Tab", 0) || f.key("x", 0) {
		t.Error("keys are not taken while editing")
	}
	eq(t, "items", len(f.s.element.Items), 5)
	f.s.FinishEdit("Bee", true)
	eq(t, "name", f.b.Name, "Bee")
	eq(t, "closed", f.s.Editing() == nil, true)
	f.s.BeginEdit(f.b, EditName)
	f.s.FinishEdit("Ignored", false)
	eq(t, "cancel keeps", f.b.Name, "Bee")
}

func TestSpaceAsksForOperation(t *testing.T) {
	f := newFixture(t)
	asked := 0
	f.s.opts.OnOperation = func() { asked++ }
	f.key(" ", 0)
	f.s.Select([]*model.DiagramItem{f.a})
	f.key(" ", 0)
	eq(t, "asked", asked, 1)
}

func TestUndoRedoKeys(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	f.key("Delete", 0)
	eq(t, "deleted", len(f.s.element.Items), 3)
	f.key("z", ModMeta)
	eq(t, "undone", len(f.s.element.Items), 5)
	f.key("z", ModMeta|ModShift)
	eq(t, "redone", len(f.s.element.Items), 3)
}

func TestBatchJoinsUndoSteps(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	f.s.Batch(func() {
		f.s.SetQuickStyle("color", "red")
		f.s.SetQuickStyle("color", "blue")
	})
	f.s.Undo()
	if strings.Contains(f.props(f.a), "color") {
		t.Error("both changes are one undo step")
	}
}

func TestAddDuplicateInheritElement(t *testing.T) {
	f := newFixture(t)
	root := &f.s.root.Element
	el := f.s.AddElement(nil)
	eq(t, "name", el.Name, "Unnamed element: 0")
	eq(t, "second name", f.s.AddElement(nil).Name, "Unnamed element: 1")
	eq(t, "root children", len(root.Elements), 3)
	dup := f.s.DuplicateElement(f.s.element)
	eq(t, "copy items", len(dup.Items), 5)
	if dup == f.s.element || dup.Items[0] == f.s.element.Items[0] {
		t.Error("the copy must be separate")
	}
	eq(t, "copy is last", root.Elements[len(root.Elements)-1], dup)

	f.s.Inherit()
	inner := f.s.element.Elements[0]
	eq(t, "inner items", len(inner.Items), 5)
	contains(t, f.s.PropsText(Target{Item: inner.Items[0]}), `inherit "../A"`)
	f.s.Undo()
	eq(t, "undone", len(f.s.element.Elements), 0)
}

func TestSetElementAndReveal(t *testing.T) {
	f := newFixture(t)
	other := f.s.AddElement(nil)
	elements := 0
	f.s.opts.OnElement = func() { elements++ }
	f.s.Select([]*model.DiagramItem{f.a})
	f.s.SetElement(other)
	eq(t, "element", f.s.Element(), other)
	eq(t, "selection dropped", len(f.s.Selection()), 0)
	eq(t, "notified", elements, 1)
	f.s.SetElement(other)
	eq(t, "same element: nothing", elements, 1)
	f.s.Reveal(f.b)
	eq(t, "revealed", []any{f.s.Element() == f.b.Parent, f.s.Selection()[0]}, []any{true, f.b})
	// The undo history survives the switch.
	f.s.Undo()
	eq(t, "undo after switch", len(f.s.root.Elements), 1)
}
