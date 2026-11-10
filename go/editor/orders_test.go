package editor

import (
	"slices"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// names is the drawing order of the items of the fixture by name.
func (f *fixture) names() []string {
	out := make([]string, 0, len(f.s.element.Items))
	for _, it := range f.s.element.Items {
		out = append(out, it.Name)
	}
	return out
}

// The order of the items is the order they are drawn in: Raise moves an item one place towards the top,
// Lower one place towards the bottom, and neither passes a neighbour that is in the selection too.
func TestRaiseAndLowerMoveOnePlace(t *testing.T) {
	f := newFixture(t)
	before := f.names()
	a := f.s.element.Items[0]
	f.s.Select([]*model.DiagramItem{a})
	f.s.Raise()
	if got := f.names(); !slices.Equal(got, []string{before[1], before[0], before[2], before[3], before[4]}) {
		t.Fatalf("raise gave %v, was %v", got, before)
	}
	f.s.Lower()
	if got := f.names(); !slices.Equal(got, before) {
		t.Fatalf("lower gave %v, want %v", got, before)
	}
	// The last item cannot go higher, the first cannot go lower.
	f.s.Select([]*model.DiagramItem{f.s.element.Items[len(f.s.element.Items)-1]})
	f.s.Raise()
	if got := f.names(); !slices.Equal(got, before) {
		t.Errorf("raise of the top item changed the order: %v", got)
	}
	f.s.Select([]*model.DiagramItem{f.s.element.Items[0]})
	f.s.Lower()
	if got := f.names(); !slices.Equal(got, before) {
		t.Errorf("lower of the bottom item changed the order: %v", got)
	}
}

// Two neighbours move together: the block keeps its own order and does not pass its own items.
func TestRaiseKeepsTheOrderOfTheBlock(t *testing.T) {
	f := newFixture(t)
	before := f.names()
	items := f.s.element.Items
	f.s.Select([]*model.DiagramItem{items[0], items[1]})
	f.s.Raise()
	got := f.names()
	if !slices.Equal(got[1:3], []string{before[0], before[1]}) {
		t.Fatalf("the block lost its order: %v (was %v)", got, before)
	}
	if got[0] != before[2] {
		t.Errorf("the item above the block did not stay above it: %v", got)
	}
}

// Bring to Front and Send to Back put the selection at the ends of the order, keeping its own order.
func TestToFrontAndToBack(t *testing.T) {
	f := newFixture(t)
	before := f.names()
	items := f.s.element.Items
	f.s.Select([]*model.DiagramItem{items[0], items[1]})
	f.s.ToFront()
	got := f.names()
	if !slices.Equal(got[len(got)-2:], []string{before[0], before[1]}) {
		t.Fatalf("to front gave %v, was %v", got, before)
	}
	f.s.ToBack()
	got = f.names()
	if !slices.Equal(got[:2], []string{before[0], before[1]}) {
		t.Fatalf("to back gave %v, was %v", got, before)
	}
}

// One command is one undo step: the order of the items comes back with a single Undo.
func TestOrderIsOneUndoStep(t *testing.T) {
	f := newFixture(t)
	before := f.names()
	f.s.Select([]*model.DiagramItem{f.s.element.Items[0]})
	f.s.Raise()
	if slices.Equal(f.names(), before) {
		t.Fatal("the order did not change")
	}
	f.s.Undo()
	if got := f.names(); !slices.Equal(got, before) {
		t.Fatalf("undo gave %v, want %v", got, before)
	}
}

// The layer is the other way to say what is on top: background is drawn under the rest, hover above it.
func TestLayerUpAndDown(t *testing.T) {
	f := newFixture(t)
	it := f.s.element.Items[0]
	layer := func() string { return layerOf(it) }
	f.s.Select([]*model.DiagramItem{it})
	if got := layer(); got != "default" {
		t.Fatalf("the item starts at %q", got)
	}
	f.s.LayerUp()
	if got := layer(); got != "hover" {
		t.Fatalf("layer up gave %q, want hover", got)
	}
	f.s.LayerUp() // already the top: nothing happens
	if got := layer(); got != "hover" {
		t.Fatalf("the top layer went further: %q", got)
	}
	f.s.LayerDown()
	f.s.LayerDown()
	if got := layer(); got != "background" {
		t.Fatalf("layer down twice gave %q, want background", got)
	}
	f.s.Undo()
	if got := layer(); got != "default" {
		t.Fatalf("undo of a layer step gave %q, want default", got)
	}
}

// A command on two items is still one undo step: the group by event of the undo manager holds the two
// writes of the order together.
func TestOrderOfTwoItemsIsOneUndoStep(t *testing.T) {
	f := newFixture(t)
	before := f.names()
	items := f.s.element.Items
	f.s.Select([]*model.DiagramItem{items[0], items[1]})
	f.s.Raise()
	if slices.Equal(f.names(), before) {
		t.Fatal("the order did not change")
	}
	f.s.Undo()
	if got := f.names(); !slices.Equal(got, before) {
		t.Fatalf("one undo gave %v, want %v", got, before)
	}
	if f.s.UndoManager().CanUndo() {
		t.Error("the command left more than one step")
	}
}
