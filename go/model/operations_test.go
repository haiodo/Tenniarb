package model

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Port of web/packages/core/test/operations.test.ts.

func makeStore() (*ElementModelStore, *Element) {
	model := NewElementModel()
	diagram := NewElement("Diagram")
	model.AddElement(diagram)
	return NewElementModelStore(model), diagram
}

func makeItem(name string, x, y float64) *DiagramItem {
	item := NewDiagramItem(ItemKindItem, name)
	item.X, item.Y = x, y
	return item
}

func itemNames(xs []*DiagramItem) []string {
	out := []string{}
	for _, x := range xs {
		out = append(out, x.Name)
	}
	return out
}

func elementNames(xs []*Element) []string {
	out := []string{}
	for _, x := range xs {
		out = append(out, x.Name)
	}
	return out
}

func noop() {}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func eq[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}

func eqNames(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func eqItems(t *testing.T, got []*DiagramItem, want ...*DiagramItem) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("items differ: got %v, want %v", itemNames(got), itemNames(want))
	}
}

func parseProps(text string) *tenn.Node {
	tree, _ := tenn.Parse(text)
	return tree
}

func saved(m *ElementModel) string {
	return ToTennStr(&m.Element, true, true)
}

// 1. AddElement / RemoveElement

func TestAddElementApplyUndoIsSymmetric(t *testing.T) {
	store, diagram := makeStore()
	child := NewElement("Child")

	op := NewAddElement(store, diagram, child, nil)
	eq(t, len(diagram.Elements), 0, "before")

	must(t, op.Apply())
	if !slices.Contains(diagram.Elements, child) || len(diagram.Elements) != 1 {
		t.Fatal("child not added")
	}
	if child.Parent != diagram || child.Model != store.Model {
		t.Fatal("parent/model not assigned")
	}

	must(t, op.Undo())
	eq(t, len(diagram.Elements), 0, "after undo")
}

func TestAddElementAtIndexRestoresPosition(t *testing.T) {
	store, diagram := makeStore()
	diagram.AddElement(NewElement("A"))
	diagram.AddElement(NewElement("B"))

	op := NewAddElement(store, diagram, NewElement("Inserted"), IntPtr(1))
	must(t, op.Apply())
	eqNames(t, elementNames(diagram.Elements), "A", "Inserted", "B")

	must(t, op.Undo())
	eqNames(t, elementNames(diagram.Elements), "A", "B")
}

func TestAddElementIndexMinusOneAppends(t *testing.T) {
	store, diagram := makeStore()
	diagram.AddElement(NewElement("A"))
	must(t, NewAddElement(store, diagram, NewElement("Z"), IntPtr(-1)).Apply())
	eqNames(t, elementNames(diagram.Elements), "A", "Z")
}

func TestRemoveElementUndoRestoresOriginalIndex(t *testing.T) {
	store, diagram := makeStore()
	b := NewElement("B")
	diagram.AddElement(NewElement("A"))
	diagram.AddElement(b)
	diagram.AddElement(NewElement("C"))

	op := NewRemoveElement(store, diagram, b)
	must(t, op.Apply())
	eqNames(t, elementNames(diagram.Elements), "A", "C")

	must(t, op.Undo())
	eqNames(t, elementNames(diagram.Elements), "A", "B", "C")
}

// 2. AddItem / RemoveItem and link preservation

func TestAddItemApplyUndoIsSymmetric(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 0, 0)

	op := NewAddItem(store, diagram, item, nil)
	must(t, op.Apply())
	eq(t, len(diagram.Items), 1, "items")
	if item.Parent != diagram {
		t.Fatal("parent not assigned")
	}

	must(t, op.Undo())
	eq(t, len(diagram.Items), 0, "items after undo")
}

func TestAddItemAtIndex(t *testing.T) {
	store, diagram := makeStore()
	diagram.AddItem(makeItem("A", 0, 0))
	diagram.AddItem(makeItem("B", 0, 0))
	op := NewAddItem(store, diagram, makeItem("X", 0, 0), IntPtr(1))
	must(t, op.Apply())
	eqNames(t, itemNames(diagram.Items), "A", "X", "B")
	must(t, op.Undo())
	eqNames(t, itemNames(diagram.Items), "A", "B")
}

func TestRemoveItemUndoRestoresOriginalIndex(t *testing.T) {
	store, diagram := makeStore()
	b := makeItem("B", 0, 0)
	diagram.AddItem(makeItem("A", 0, 0))
	diagram.AddItem(b)
	diagram.AddItem(makeItem("C", 0, 0))

	op := NewRemoveItem(store, diagram, b)
	must(t, op.Apply())
	eqNames(t, itemNames(diagram.Items), "A", "C")

	must(t, op.Undo())
	eqNames(t, itemNames(diagram.Items), "A", "B", "C")
}

func TestRemoveItemAlsoRemovesItsLinks(t *testing.T) {
	store, diagram := makeStore()
	source := makeItem("Source", 0, 0)
	target := makeItem("Target", 0, 0)
	diagram.AddSourceTarget(source, target)
	eq(t, len(diagram.Items), 3, "source, target and the link between them")

	related := diagram.GetRelatedItems(source, true, true)
	eq(t, len(related), 2, "The item itself plus the link referencing it")

	var ops []ElementOperation
	for _, r := range related {
		ops = append(ops, NewRemoveItem(store, diagram, r))
	}
	composite := NewCompositeOperation(store, diagram, ops...)
	must(t, composite.Apply())
	eqNames(t, itemNames(diagram.Items), "Target")

	must(t, composite.Undo())
	eq(t, len(diagram.Items), 3, "items after undo")
	i := slices.IndexFunc(diagram.Items, func(i *DiagramItem) bool { return i.IsLink() })
	if i < 0 {
		t.Fatal("the link must come back")
	}
	if link := diagram.Items[i]; link.Source != source || link.Target != target {
		t.Fatal("link endpoints lost")
	}
}

// 3. Value updates

func TestUpdatePositionApplyUndo(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 10, 20)
	diagram.AddItem(item)

	op := NewUpdatePosition(store, diagram, item, Point{10, 20}, Point{100, 200})
	must(t, op.Apply())
	eq(t, item.X, 100.0, "x")
	eq(t, item.Y, 200.0, "y")

	must(t, op.Undo())
	eq(t, item.X, 10.0, "x undo")
	eq(t, item.Y, 20.0, "y undo")
}

func TestUpdatePositionNameMatchesSwiftString(t *testing.T) {
	store, diagram := makeStore()
	op := NewUpdatePosition(store, diagram, makeItem("N", 0, 0), Point{1, 2.5}, Point{3, 4})
	eq(t, op.Name(), "UpdatePosition: N OLD:( 1.0, 2.5NEW:( 3.0, 4.0 ", "name")
}

func TestUpdateNameApplyUndo(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Old", 0, 0)
	diagram.AddItem(item)

	op := NewUpdateName(store, diagram, item, "Old", "New")
	must(t, op.Apply())
	eq(t, item.Name, "New", "name")
	must(t, op.Undo())
	eq(t, item.Name, "Old", "name undo")
}

func TestUpdateElementNameApplyUndo(t *testing.T) {
	store, diagram := makeStore()
	op := NewUpdateElementName(store, diagram, "Diagram", "Renamed")
	must(t, op.Apply())
	eq(t, diagram.Name, "Renamed", "name")
	must(t, op.Undo())
	eq(t, diagram.Name, "Diagram", "name undo")
}

func TestComplexUpdateItemRoundTripsProperties(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 5, 7)
	diagram.AddItem(item)

	before := ItemToTennAsProps(item, tenn.KindStatements, false)
	after := parseProps("name Node\npos 42 43\ncolor red")
	was := saved(store.Model)

	op := NewComplexUpdateItem(store, diagram, item, before, after)
	must(t, op.Apply())
	if item.X != 42 || item.Y != 43 || item.Properties.Get("color") == nil {
		t.Fatalf("not applied: %v %v", item.X, item.Y)
	}

	must(t, op.Undo())
	if item.X != 5 || item.Y != 7 || item.Properties.Get("color") != nil {
		t.Fatal("undo must restore the property snapshot taken before apply")
	}
	eq(t, saved(store.Model), was, "name, description and properties all restored")
}

func TestComplexUpdateElementRoundTripsProperties(t *testing.T) {
	store, diagram := makeStore()

	before := ElementToTennAsProps(diagram, tenn.KindStatements, false)
	after := parseProps("name Updated")
	was := saved(store.Model)

	op := NewComplexUpdateElement(store, diagram, before, after)
	must(t, op.Apply())
	eq(t, diagram.Name, "Updated", "name")
	must(t, op.Undo())
	eq(t, diagram.Name, "Diagram", "name undo")
	eq(t, saved(store.Model), was, "saved")
}

func TestComplexUpdateItemOnLinkRestoresLabel(t *testing.T) {
	store, diagram := makeStore()
	diagram.AddSourceTarget(makeItem("A", 0, 0), makeItem("B", 0, 0))
	link := diagram.Items[0]
	link.Name = "old"

	before := ItemToTennAsProps(link, tenn.KindStatements, false)
	op := NewComplexUpdateItem(store, diagram, link, before, parseProps("label new"))
	must(t, op.Apply())
	eq(t, link.Name, "new", "label")
	must(t, op.Undo())
	eq(t, link.Name, "old", "label undo")
}

// 4. CompositeOperation

func TestCompositeAppliesInOrderAndUndoesInReverse(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 0, 0)
	diagram.AddItem(item)

	first := NewUpdatePosition(store, diagram, item, Point{0, 0}, Point{1, 1})
	second := NewUpdatePosition(store, diagram, item, Point{1, 1}, Point{2, 2})
	composite := NewCompositeOperation(store, diagram, first, second)

	must(t, composite.Apply())
	eq(t, item.X, 2.0, "The last operation in the list wins")

	must(t, composite.Undo())
	eq(t, item.X, 0.0, "Reverse order undo must walk back through both positions")
}

func TestCompositeTracksUndoCalledOnChildren(t *testing.T) {
	store, diagram := makeStore()
	composite := NewCompositeOperation(store, diagram,
		NewAddItem(store, diagram, makeItem("A", 0, 0), nil),
		NewAddItem(store, diagram, makeItem("B", 0, 0), nil))

	must(t, composite.Apply())
	for _, o := range composite.Operations {
		if o.Base().UndoCalled {
			t.Fatal("child flag not cleared by apply")
		}
	}
	must(t, composite.Undo())
	for _, o := range composite.Operations {
		if !o.Base().UndoCalled {
			t.Fatal("child flag not set by undo")
		}
	}
}

func TestCompositeEventKindIsStructureWhenChildrenDisagree(t *testing.T) {
	store, diagram := makeStore()
	eq(t, NewCompositeOperation(store, diagram).GetEventKind(), EventStructure, "empty composite falls back to the base kind")

	// Every operation of this port reports Structure; a Layout child is the only way to see the kinds disagree.
	layout := &layoutKindOp{OpBase: newOpBase(store)}
	mixed := NewCompositeOperation(store, diagram, layout, NewAddItem(store, diagram, makeItem("a", 0, 0), nil))
	eq(t, mixed.GetEventKind(), EventStructure, "mixed kinds")
	eq(t, NewCompositeOperation(store, diagram, layout, layout).GetEventKind(), EventLayout, "uniform kinds are kept")
}

type layoutKindOp struct{ OpBase }

func (*layoutKindOp) GetEventKind() ModelEventKind { return EventLayout }

func TestCompositeNameJoinsChildNames(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 0, 0)
	composite := NewCompositeOperation(store, diagram, NewAddItem(store, diagram, item, nil), NewRemoveItem(store, diagram, item))
	eq(t, composite.Name(), "AddItem,RemoveItem", "name")
}

func TestCompositeNotifierIsTheGivenElement(t *testing.T) {
	store, _ := makeStore()
	other := NewElement("Other")
	eq(t, NewCompositeOperation(store, other).GetNotifier(), other, "notifier")
}

// 5. ModelEvent collection

func TestAddItemCollectsAppendThenRemove(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 0, 0)
	op := NewAddItem(store, diagram, item, nil)

	must(t, op.Apply())
	op.UndoCalled = false
	var applied EventMap[*DiagramItem]
	op.CollectItems(&applied)
	if got, _ := applied.Get(item); got != EventAppend {
		t.Fatalf("got %v", got)
	}

	must(t, op.Undo())
	op.UndoCalled = true
	var undone EventMap[*DiagramItem]
	op.CollectItems(&undone)
	if got, _ := undone.Get(item); got != EventRemove {
		t.Fatalf("got %v", got)
	}
}

func TestRemoveElementCollectsRemoveThenAppend(t *testing.T) {
	store, diagram := makeStore()
	child := NewElement("Child")
	diagram.AddElement(child)
	op := NewRemoveElement(store, diagram, child)

	must(t, op.Apply())
	op.UndoCalled = false
	var applied EventMap[*Element]
	op.CollectElements(&applied)
	if got, _ := applied.Get(child); got != EventRemove {
		t.Fatalf("got %v", got)
	}

	must(t, op.Undo())
	op.UndoCalled = true
	var undone EventMap[*Element]
	op.CollectElements(&undone)
	if got, _ := undone.Get(child); got != EventAppend {
		t.Fatalf("got %v", got)
	}
}

func TestCompositeCollectsFromEveryChild(t *testing.T) {
	store, diagram := makeStore()
	a, b := makeItem("A", 0, 0), makeItem("B", 0, 0)
	composite := NewCompositeOperation(store, diagram, NewAddItem(store, diagram, a, nil), NewAddItem(store, diagram, b, nil))
	must(t, composite.Apply())

	var items EventMap[*DiagramItem]
	composite.CollectItems(&items)
	eq(t, items.Len(), 2, "size")
	if oa, _ := items.Get(a); oa != EventAppend {
		t.Fatal("a")
	}
	if ob, _ := items.Get(b); ob != EventAppend {
		t.Fatal("b")
	}
}

func TestUpdateOperationCollectsUpdate(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 0, 0)
	diagram.AddItem(item)

	op := NewUpdateName(store, diagram, item, "Node", "Renamed")
	var items EventMap[*DiagramItem]
	op.CollectItems(&items)
	if got, _ := items.Get(item); got != EventUpdate {
		t.Fatalf("Value updates report Update regardless of direction, got %v", got)
	}
}

func TestOperationsCollectOnlyIntoTheirOwnMap(t *testing.T) {
	store, diagram := makeStore()
	child := NewElement("C")
	add := NewAddElement(store, diagram, child, nil)
	var els EventMap[*Element]
	var its EventMap[*DiagramItem]
	add.CollectElements(&els)
	add.CollectItems(&its)
	if got, _ := els.Get(child); got != EventRemove {
		t.Fatal("UndoCalled starts true, so a fresh op reports the undo direction")
	}
	eq(t, its.Len(), 0, "items")
	must(t, add.Apply())
	add.UndoCalled = false
	add.CollectElements(&els)
	if got, _ := els.Get(child); got != EventAppend {
		t.Fatal("applied")
	}

	item := makeItem("I", 0, 0)
	rm := NewRemoveItem(store, diagram, item)
	var els2 EventMap[*Element]
	rm.CollectElements(&els2)
	eq(t, els2.Len(), 0, "elements")
	rm.UndoCalled = false
	rm.CollectItems(&its)
	if got, _ := its.Get(item); got != EventRemove {
		t.Fatal("remove")
	}
	rm.UndoCalled = true
	rm.CollectItems(&its)
	if got, _ := its.Get(item); got != EventAppend {
		t.Fatal("append")
	}
}

// 6. Store execute, undo manager and listeners

type recordingListener struct{ events []*ModelEvent }

func (r *recordingListener) NotifyChanges(e *ModelEvent) { r.events = append(r.events, e) }

func TestExecuteAppliesAndNotifiesListener(t *testing.T) {
	store, diagram := makeStore()
	listener := &recordingListener{}
	store.OnUpdate = []IElementModelListener{listener}

	item := makeItem("Node", 0, 0)
	must(t, store.AddItem(diagram, item, nil, noop))

	eq(t, len(diagram.Items), 1, "items")
	eq(t, len(listener.events), 1, "events")
	event := listener.events[0]
	eq(t, event.Element, diagram, "event element")
	eq(t, event.Kind, EventStructure, "kind")
	if got, _ := event.Items.Get(item); got != EventAppend {
		t.Fatal("expected Append")
	}
	if !store.Modified {
		t.Fatal("not modified")
	}
}

func TestExecuteCallsRefreshAfterApply(t *testing.T) {
	store, diagram := makeStore()
	refreshed := 0
	must(t, store.AddItem(diagram, makeItem("Node", 0, 0), nil, func() {
		refreshed++
		eq(t, len(diagram.Items), 1, "items at refresh")
	}))
	eq(t, refreshed, 1, "refreshed")
}

type fakeContext struct{ onNotify func(*ModelEvent) }

func (f *fakeContext) SetScaleFactor(float64)      {}
func (f *fakeContext) SetElement(*Element)         {}
func (f *fakeContext) UpdateAll(func())            {}
func (f *fakeContext) NotifyChanges(e *ModelEvent) { f.onNotify(e) }
func (f *fakeContext) GetEvaluated(EvalTarget, *tenn.Node, *DrawableSize) map[*tenn.Token]any {
	return nil
}

func TestExecuteCallsTheExecutionContextBeforeListeners(t *testing.T) {
	store, diagram := makeStore()
	var order []string
	store.ExecutionContext = &fakeContext{onNotify: func(*ModelEvent) { order = append(order, "ctx") }}
	store.OnUpdate = []IElementModelListener{ListenerFunc(func(*ModelEvent) { order = append(order, "listener") })}
	must(t, store.AddItem(diagram, makeItem("Node", 0, 0), nil, func() { order = append(order, "refresh") }))
	eqNames(t, order, "ctx", "listener", "refresh")
}

func TestUndoManagerRoundTripRestoresModel(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}

	um.BeginUndoGrouping()
	must(t, store.AddItem(diagram, makeItem("Node", 0, 0), um, noop))
	must(t, um.EndUndoGrouping())
	eq(t, len(diagram.Items), 1, "items")
	if !um.CanUndo() {
		t.Fatal("canUndo")
	}

	must(t, um.Undo())
	eq(t, len(diagram.Items), 0, "Undo must take the item back out")
	if !um.CanRedo() || um.CanUndo() {
		t.Fatal("stacks after undo")
	}

	must(t, um.Redo())
	eq(t, len(diagram.Items), 1, "Redo must put it back")
	if !um.CanUndo() || um.CanRedo() {
		t.Fatal("stacks after redo")
	}
}

func TestExecuteTogglesBetweenApplyAndUndo(t *testing.T) {
	store, diagram := makeStore()
	op := NewAddItem(store, diagram, makeItem("Node", 0, 0), nil)

	must(t, store.Execute(op, nil, noop))
	eq(t, len(diagram.Items), 1, "items")
	eq(t, op.UndoCalled, false, "flag")

	must(t, store.Execute(op, nil, noop))
	eq(t, len(diagram.Items), 0, "Re-executing the same operation undoes it")
	eq(t, op.UndoCalled, true, "flag")
}

func TestUngroupedRegistrationsAreSeparateStepsAndNewEditClearsRedo(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	must(t, store.AddItem(diagram, makeItem("B", 0, 0), um, noop))

	must(t, um.Undo())
	eqNames(t, itemNames(diagram.Items), "A")
	must(t, um.Undo())
	eqNames(t, itemNames(diagram.Items))
	must(t, um.Redo())
	eqNames(t, itemNames(diagram.Items), "A")

	must(t, store.AddItem(diagram, makeItem("C", 0, 0), um, noop))
	if um.CanRedo() {
		t.Fatal("A fresh registration drops the redo stack")
	}
	must(t, um.Undo())
	must(t, um.Undo())
	eqNames(t, itemNames(diagram.Items))
	if um.CanUndo() {
		t.Fatal("canUndo")
	}
}

func TestGroupUndoesAsOneStepInReverseOrder(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	um.BeginUndoGrouping()
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	must(t, store.AddItem(diagram, makeItem("B", 0, 0), um, noop))
	must(t, um.EndUndoGrouping())

	must(t, um.Undo())
	eq(t, len(diagram.Items), 0, "after undo")
	must(t, um.Redo())
	eqNames(t, itemNames(diagram.Items), "A", "B")
	must(t, um.Undo())
	eq(t, len(diagram.Items), 0, "after second undo")
}

func TestRemoveAllActionsAndEmptyStacks(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	must(t, um.Undo())
	must(t, um.Redo())
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	must(t, um.RemoveAllActions())
	if um.CanUndo() || um.CanRedo() {
		t.Fatal("stacks not cleared")
	}
	must(t, um.Undo())
	eq(t, len(diagram.Items), 1, "items")
}

func TestListenersSeeEveryStepAndModifiedIsSet(t *testing.T) {
	store, diagram := makeStore()
	listener := &recordingListener{}
	store.OnUpdate = []IElementModelListener{listener}
	um := &UndoManager{}
	item := makeItem("A", 0, 0)
	if store.Modified {
		t.Fatal("modified at start")
	}
	must(t, store.AddItem(diagram, item, um, noop))
	if !store.Modified {
		t.Fatal("modified")
	}
	must(t, um.Undo())
	if !store.Modified {
		t.Fatal("modified stays true after undoing back to the start")
	}
	must(t, um.Redo())
	var got []ModelEventOperation
	for _, e := range listener.events {
		op, _ := e.Items.Get(item)
		got = append(got, op)
	}
	if !slices.Equal(got, []ModelEventOperation{EventAppend, EventRemove, EventAppend}) {
		t.Fatalf("got %v", got)
	}
}

// Store entry points: every operation through the store, with undo and redo.

func undoRedo(t *testing.T, um *UndoManager, applied, undone func()) {
	t.Helper()
	applied()
	must(t, um.Undo())
	undone()
	must(t, um.Redo())
	applied()
}

func TestStoreAddElementUndoRedo(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	child := NewElement("Child")
	must(t, store.Add(diagram, child, um, noop, nil))
	undoRedo(t, um,
		func() {
			if !slices.Equal(diagram.Elements, []*Element{child}) {
				t.Fatal("applied")
			}
		},
		func() { eq(t, len(diagram.Elements), 0, "undone") })
}

func TestStoreAddAndRemoveElements(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	a, b := NewElement("A"), NewElement("B")

	must(t, store.AddElements(diagram, []*Element{a, b}, um, noop))
	eqNames(t, elementNames(diagram.Elements), "A", "B")
	must(t, um.Undo())
	eqNames(t, elementNames(diagram.Elements))
	must(t, um.Redo())

	must(t, store.Remove(diagram, a, um, noop))
	eqNames(t, elementNames(diagram.Elements), "B")
	must(t, um.Undo())
	eqNames(t, elementNames(diagram.Elements), "A", "B")
	must(t, um.Redo())
	eqNames(t, elementNames(diagram.Elements), "B")
}

func TestStoreMoveElement(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	other := NewElement("Other")
	store.Model.AddElement(other)
	keep, child := NewElement("Keep"), NewElement("Child")
	other.AddElement(keep)
	diagram.AddElement(child)

	must(t, store.Move(child, other, um, noop, 0))
	if slices.Contains(diagram.Elements, child) {
		t.Fatal("still in the old parent")
	}
	eqNames(t, elementNames(other.Elements), "Child", "Keep")
	eq(t, child.Parent, other, "parent")

	must(t, um.Undo())
	eqNames(t, elementNames(diagram.Elements), "Child")
	eqNames(t, elementNames(other.Elements), "Keep")

	must(t, um.Redo())
	eqNames(t, elementNames(other.Elements), "Child", "Keep")
}

func TestStoreAddItemAndAddItemsUndoRedo(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	must(t, store.AddItems(diagram, []*DiagramItem{makeItem("B", 0, 0), makeItem("C", 0, 0)}, um, noop))
	eqNames(t, itemNames(diagram.Items), "A", "B", "C")
	must(t, um.Undo())
	eqNames(t, itemNames(diagram.Items), "A")
	must(t, um.Redo())
	eqNames(t, itemNames(diagram.Items), "A", "B", "C")
}

func TestStoreAddLinkAddsMissingEndpointsAndTheLink(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	a, b := makeItem("A", 0, 0), makeItem("B", 0, 0)
	diagram.AddItem(a)
	props := parseProps("color red").Children

	must(t, store.AddLink(diagram, a, b, um, noop, props))
	eq(t, len(diagram.Items), 3, "items")
	eqItems(t, diagram.Items[:2], a, b) // only the missing endpoint is added, before the link
	link := diagram.Items[2]
	if !link.IsLink() || link.Source != a || link.Target != b {
		t.Fatal("link ends")
	}
	if link.Properties.Get("color") == nil {
		t.Fatal("link properties")
	}

	must(t, um.Undo())
	eqItems(t, diagram.Items, a)
	must(t, um.Redo())
	eq(t, len(diagram.Items), 3, "items after redo")
}

func TestRemoveItemThroughStoreDropsRelatedLinks(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	source, target := makeItem("Source", 0, 0), makeItem("Target", 0, 0)
	diagram.AddSourceTarget(source, target)
	original := slices.Clone(diagram.Items)

	must(t, store.RemoveItem(diagram, source, um, noop))
	eqNames(t, itemNames(diagram.Items), "Target") // removing an endpoint drops the link too

	must(t, um.Undo())
	eqItems(t, diagram.Items, original...)
	must(t, um.Redo())
	eqNames(t, itemNames(diagram.Items), "Target")
}

func TestStoreAddAndRemoveMultipleItems(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	a, b, c := makeItem("A", 0, 0), makeItem("B", 0, 0), makeItem("C", 0, 0)

	must(t, store.AddItems(diagram, []*DiagramItem{a, b}, um, noop))
	eq(t, len(diagram.Items), 2, "items")

	diagram.AddSourceTarget(a, c)
	original := slices.Clone(diagram.Items)
	must(t, store.RemoveItems(diagram, []*DiagramItem{a, b}, um, noop))
	eqNames(t, itemNames(diagram.Items), "C") // shared related items are removed once
	must(t, um.Undo())
	eqItems(t, diagram.Items, original...)
	must(t, um.Redo())
	eqNames(t, itemNames(diagram.Items), "C")
}

func TestStoreUpdateNameForItemAndElement(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	item := makeItem("Old", 0, 0)
	diagram.AddItem(item)

	must(t, store.UpdateName(item, "New", um, noop))
	eq(t, item.Name, "New", "name")
	must(t, um.Undo())
	eq(t, item.Name, "Old", "undo")
	must(t, um.Redo())
	eq(t, item.Name, "New", "redo")

	must(t, store.UpdateElementName(diagram, "Renamed", um, noop))
	eq(t, diagram.Name, "Renamed", "element name")
	must(t, um.Undo())
	eq(t, diagram.Name, "Diagram", "element undo")
	must(t, um.Redo())
	eq(t, diagram.Name, "Renamed", "element redo")
}

func TestStoreUpdatePosition(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	item := makeItem("Node", 1, 2)
	diagram.AddItem(item)

	pos := func() [2]float64 { return [2]float64{item.X, item.Y} }
	must(t, store.UpdatePosition(item, Point{30, 40}, um, noop))
	eq(t, pos(), [2]float64{30, 40}, "applied")
	must(t, um.Undo())
	eq(t, pos(), [2]float64{1, 2}, "undone")
	must(t, um.Redo())
	eq(t, pos(), [2]float64{30, 40}, "redone")
}

func TestStoreSetPropertiesOnItemAndElement(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	item := makeItem("Node", 1, 2)
	diagram.AddItem(item)
	pos := func() [2]float64 { return [2]float64{item.X, item.Y} }

	was := saved(store.Model)
	must(t, store.SetItemProperties(diagram, item, parseProps("name Node\npos 11 12\ncolor red"), um, noop))
	eq(t, pos(), [2]float64{11, 12}, "applied")
	must(t, um.Undo())
	eq(t, pos(), [2]float64{1, 2}, "undone")
	eq(t, saved(store.Model), was, "saved")
	must(t, um.Redo())
	eq(t, pos(), [2]float64{11, 12}, "redone")

	must(t, store.SetProperties(diagram, parseProps("name Renamed"), um, noop))
	eq(t, diagram.Name, "Renamed", "element")
	must(t, um.Undo())
	eq(t, diagram.Name, "Diagram", "element undo")
	must(t, um.Redo())
	eq(t, diagram.Name, "Renamed", "element redo")
}

func TestCreatePropertiesBuildsUndoableOperation(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 3, 4)
	diagram.AddItem(item)

	op := store.CreateProperties(diagram, item, parseProps("name Node\npos 55 66"))
	must(t, op.Apply())
	eq(t, item.X, 55.0, "applied")
	must(t, op.Undo())
	eq(t, item.X, 3.0, "undone")
}

func TestMakeNonModifiedClearsDirtyFlag(t *testing.T) {
	store, diagram := makeStore()
	must(t, store.AddItem(diagram, makeItem("Node", 0, 0), nil, noop))
	if !store.Modified {
		t.Fatal("modified")
	}
	store.MakeNonModified()
	if store.Modified {
		t.Fatal("still modified")
	}
}

func TestCompositeAddAppendsOperations(t *testing.T) {
	store, diagram := makeStore()
	composite := NewCompositeOperation(store, diagram)
	eq(t, len(composite.Operations), 0, "empty")
	composite.Add(NewAddItem(store, diagram, makeItem("A", 0, 0), nil), NewAddItem(store, diagram, makeItem("B", 0, 0), nil))
	eq(t, len(composite.Operations), 2, "after add")
}

func TestCompositeOperationThroughStore(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	item := makeItem("Node", 1, 1)
	diagram.AddItem(item)
	ops := []ElementOperation{store.CreateUpdatePosition(item, Point{5, 5}), NewUpdateName(store, diagram, item, "Node", "N2")}
	must(t, store.CompositeOperation(diagram, um, noop, ops))
	eq(t, item.X, 5.0, "x")
	eq(t, item.Name, "N2", "name")
	must(t, um.Undo())
	eq(t, item.X, 1.0, "x undo")
	eq(t, item.Name, "Node", "name undo")
	must(t, um.Redo())
	eq(t, item.X, 5.0, "x redo")
	eq(t, item.Name, "N2", "name redo")
}

func TestCreateUpdatePositionCapturesCurrentPositionAsOldValue(t *testing.T) {
	store, diagram := makeStore()
	item := makeItem("Node", 7, 8)
	diagram.AddItem(item)

	op := store.CreateUpdatePosition(item, Point{70, 80})
	must(t, op.Apply())
	eq(t, item.X, 70.0, "applied")
	must(t, op.Undo())
	eq(t, item.X, 7.0, "x: old value is the position captured at creation time")
	eq(t, item.Y, 8.0, "y")
}

func TestCreateUpdateOrderMovesItemToNewIndex(t *testing.T) {
	store, diagram := makeStore()
	c := makeItem("C", 0, 0)
	diagram.AddItem(makeItem("A", 0, 0))
	diagram.AddItem(makeItem("B", 0, 0))
	diagram.AddItem(c)

	composite := NewCompositeOperation(store, diagram, store.CreateUpdateOrder(c, IntPtr(0))...)
	must(t, composite.Apply())
	eqNames(t, itemNames(diagram.Items), "C", "A", "B")
	must(t, composite.Undo())
	eqNames(t, itemNames(diagram.Items), "A", "B", "C")
}

func TestCreateUpdateOrderThroughStore(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	listener := &recordingListener{}
	store.OnUpdate = []IElementModelListener{listener}
	a, b, c := makeItem("A", 0, 0), makeItem("B", 0, 0), makeItem("C", 0, 0)
	diagram.AddItem(a)
	diagram.AddItem(b)
	diagram.AddItem(c)
	was := saved(store.Model)

	must(t, store.CompositeOperation(diagram, um, noop, store.CreateUpdateOrder(c, IntPtr(0))))
	eqItems(t, diagram.Items, c, a, b)
	must(t, um.Undo())
	eqItems(t, diagram.Items, a, b, c)
	must(t, um.Redo())
	eqItems(t, diagram.Items, c, a, b)
	must(t, um.Undo())

	must(t, store.CompositeOperation(diagram, um, noop, store.CreateUpdateOrder(a, nil)))
	eqItems(t, diagram.Items, b, c, a)
	must(t, um.Undo())
	eqItems(t, diagram.Items, a, b, c)
	eq(t, saved(store.Model), was, "saved")

	evt := listener.events[0]
	eq(t, evt.Element, diagram, "event element")
	if got, _ := evt.Items.Get(c); got != EventAppend {
		t.Fatal("Remove then Append of the same item: last wins")
	}
}

func TestRemoveOfAMissingTargetUndoIsNoOp(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	diagram.AddItem(makeItem("a", 0, 0))
	diagram.AddItem(makeItem("b", 0, 0))
	diagram.AddElement(NewElement("e"))
	was := saved(store.Model)

	must(t, store.Execute(NewRemoveItem(store, diagram, makeItem("ghost", 0, 0)), um, noop))
	must(t, store.Execute(NewRemoveElement(store, diagram, NewElement("ghost")), um, noop))
	eq(t, saved(store.Model), was, "after remove")
	must(t, um.Undo())
	must(t, um.Undo())
	eq(t, saved(store.Model), was, "no phantom insert")
}

func TestBadIndexLeavesTheModelUntouched(t *testing.T) {
	_, diagram := makeStore()
	diagram.AddItem(makeItem("a", 0, 0))
	diagram.AddElement(NewElement("e"))
	if diagram.InsertItem(2, makeItem("z", 0, 0)) == nil || diagram.InsertItem(-1, makeItem("z", 0, 0)) == nil {
		t.Fatal("expected an error for a bad item index")
	}
	if diagram.InsertElement(-1, NewElement("z")) == nil {
		t.Fatal("expected an error for a negative element index")
	}
	eq(t, len(diagram.Items), 1, "items")
	eq(t, len(diagram.Elements), 1, "elements")
	must(t, diagram.InsertItem(1, makeItem("end", 0, 0)))
	eqNames(t, itemNames(diagram.Items), "a", "end")
	must(t, diagram.InsertElement(99, NewElement("tail")))
	eq(t, diagram.Elements[1].Name, "tail", "elements beyond count append, as in Swift")
}

func TestFailingExecuteLeavesNoUndoEntryAndNoStateChange(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	op := NewAddItem(store, diagram, makeItem("a", 0, 0), IntPtr(5))
	if store.Execute(op, um, noop) == nil {
		t.Fatal("expected an error")
	}
	if um.CanUndo() {
		t.Fatal("undo entry registered")
	}
	if !op.UndoCalled {
		t.Fatal("flag flipped")
	}
	if store.Modified {
		t.Fatal("modified")
	}
	eq(t, len(diagram.Items), 0, "items")
}

func TestListenerRemovingItselfDuringNotifyDoesNotSkipTheNext(t *testing.T) {
	store, diagram := makeStore()
	var calls []string
	var l1 ListenerFunc
	l1 = func(*ModelEvent) {
		calls = append(calls, "l1")
		store.OnUpdate = store.OnUpdate[1:]
	}
	l2 := ListenerFunc(func(*ModelEvent) { calls = append(calls, "l2") })
	store.OnUpdate = []IElementModelListener{l1, l2}
	must(t, store.AddItem(diagram, makeItem("a", 0, 0), nil, noop))
	eqNames(t, calls, "l1", "l2")
}

func TestReentrantExecuteFromAListenerKeepsProductionOrder(t *testing.T) {
	store, diagram := makeStore()
	var log []string
	fired := false
	tag := func(e *ModelEvent) string {
		var names []string
		for _, i := range e.Items.Keys() {
			names = append(names, i.Name)
		}
		return strings.Join(names, ",")
	}
	store.OnUpdate = []IElementModelListener{
		ListenerFunc(func(e *ModelEvent) {
			log = append(log, "A:"+tag(e))
			if !fired {
				fired = true
				must(t, store.AddItem(diagram, makeItem("inner", 0, 0), nil, noop))
			}
		}),
		ListenerFunc(func(e *ModelEvent) { log = append(log, "B:"+tag(e)) }),
	}
	must(t, store.AddItem(diagram, makeItem("outer", 0, 0), nil, noop))
	eqNames(t, log, "A:outer", "B:outer", "A:inner", "B:inner")
	eq(t, len(diagram.Items), 2, "items")
}

func TestUndoneOpsReportTheOppositeEventKind(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	listener := &recordingListener{}
	store.OnUpdate = []IElementModelListener{listener}
	a, b := makeItem("A", 0, 0), makeItem("B", 0, 0)
	diagram.AddItem(a)
	diagram.AddItem(b)
	child := NewElement("Child")
	diagram.AddElement(child)

	itemOps := func(events []*ModelEvent, it *DiagramItem) []ModelEventOperation {
		var out []ModelEventOperation
		for _, e := range events {
			op, _ := e.Items.Get(it)
			out = append(out, op)
		}
		return out
	}
	want := func(got []ModelEventOperation, w ...ModelEventOperation) {
		t.Helper()
		if !slices.Equal(got, w) {
			t.Fatalf("got %v, want %v", got, w)
		}
	}

	must(t, store.RemoveItem(diagram, a, um, noop))
	must(t, um.Undo())
	want(itemOps(listener.events, a), EventRemove, EventAppend)

	must(t, store.Remove(diagram, child, um, noop))
	must(t, um.Undo())
	var els []ModelEventOperation
	for _, e := range listener.events[2:] {
		op, _ := e.Elements.Get(child)
		els = append(els, op)
	}
	want(els, EventRemove, EventAppend)

	listener.events = nil
	must(t, store.UpdatePosition(b, Point{5, 6}, um, noop))
	must(t, um.Undo())
	want(itemOps(listener.events, b), EventUpdate, EventUpdate)
	for _, e := range listener.events {
		eq(t, e.Kind, EventStructure, "kind")
	}
	must(t, store.SetItemProperties(diagram, b, parseProps("name B\npos 1 1"), um, noop))
	must(t, um.Undo())
	want(itemOps(listener.events[2:], b), EventUpdate, EventUpdate)
}

func TestMoveNotifiesTheMovedElementAndAddElementsIgnoresIndex(t *testing.T) {
	store, diagram := makeStore()
	listener := &recordingListener{}
	store.OnUpdate = []IElementModelListener{listener}
	x, y, target := NewElement("X"), NewElement("Y"), NewElement("T")
	diagram.AddElement(x)
	diagram.AddElement(y)
	diagram.AddElement(target)

	must(t, store.Move(x, target, nil, noop, 0))
	eq(t, listener.events[0].Element, x, "event element")
	eq(t, x.Parent, target, "parent")

	p, q := NewElement("P"), NewElement("Q")
	must(t, store.AddElements(diagram, []*Element{p, q}, nil, noop))
	eqNames(t, elementNames(diagram.Elements[len(diagram.Elements)-2:]), "P", "Q")
}

func TestNestedGroupsCloseOnlyAtTheOutermostEnd(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	um.BeginUndoGrouping()
	um.BeginUndoGrouping()
	eq(t, um.GroupingLevel(), 2, "level")
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	must(t, um.EndUndoGrouping())
	if um.CanUndo() {
		t.Fatal("inner end closed the group")
	}
	must(t, store.AddItem(diagram, makeItem("B", 0, 0), um, noop))
	must(t, um.EndUndoGrouping())
	eq(t, um.GroupingLevel(), 0, "level")
	must(t, um.Undo())
	eq(t, len(diagram.Items), 0, "items")
	if um.EndUndoGrouping() == nil {
		t.Fatal("expected an error")
	}
	eq(t, um.GroupingLevel(), 0, "level never goes negative")
}

func TestUndoRedoInsideAnOpenGroupAndRemoveAllActionsDuringUndoFail(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	um.BeginUndoGrouping()
	if um.Undo() == nil || um.Redo() == nil {
		t.Fatal("expected errors")
	}
	must(t, um.EndUndoGrouping())

	um2 := &UndoManager{}
	var removeErr error
	must(t, store.AddItem(diagram, makeItem("B", 0, 0), um2, func() {
		if um2.IsUndoing {
			removeErr = um2.RemoveAllActions()
		}
	}))
	must(t, um2.Undo())
	if removeErr == nil {
		t.Fatal("removeAllActions while undoing must fail")
	}
	eq(t, um2.GroupingLevel(), 0, "level")
	if !um2.CanRedo() {
		t.Fatal("the failed removeAllActions must leave the stacks alone")
	}
}

func TestRegistrationDuringUndoOfAMultiStepGroup(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	a, b := makeItem("A", 0, 0), makeItem("B", 0, 0)
	diagram.AddItem(a)
	diagram.AddItem(b)
	was := saved(store.Model)

	um.BeginUndoGrouping()
	must(t, store.RemoveItem(diagram, a, um, noop))
	must(t, store.RemoveItem(diagram, b, um, noop))
	must(t, um.EndUndoGrouping())
	eq(t, len(diagram.Items), 0, "items")

	must(t, um.Undo())
	eq(t, saved(store.Model), was, "undone")
	if !um.CanRedo() || um.CanUndo() {
		t.Fatal("stacks")
	}
	must(t, um.Redo())
	eq(t, len(diagram.Items), 0, "redone")
	must(t, um.Undo())
	eqItems(t, diagram.Items, a, b)
}

func TestGroupsByEventCollectsOneEventsRegistrationsIntoOneStep(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{GroupsByEvent: true}
	must(t, store.AddItem(diagram, makeItem("A", 0, 0), um, noop))
	must(t, store.AddItem(diagram, makeItem("B", 0, 0), um, noop))
	um.CloseAutoGroup() // end of the first event
	must(t, store.AddItem(diagram, makeItem("C", 0, 0), um, noop))
	um.CloseAutoGroup()
	eq(t, um.GroupingLevel(), 0, "level")

	must(t, um.Undo())
	eqNames(t, itemNames(diagram.Items), "A", "B")
	must(t, um.Undo())
	eq(t, len(diagram.Items), 0, "items")
	must(t, um.Redo())
	eqNames(t, itemNames(diagram.Items), "A", "B")

	// undo in the same event closes the open automatic group first
	must(t, store.AddItem(diagram, makeItem("D", 0, 0), um, noop))
	must(t, um.Undo())
	eqNames(t, itemNames(diagram.Items), "A", "B")
	um.CloseAutoGroup()
	eq(t, um.GroupingLevel(), 0, "level")
}

func TestGeneratedIDsAreV4AndUnique(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	ids := map[string]bool{}
	for range 50 {
		id := NewElement("E").ID
		if !re.MatchString(id) {
			t.Fatalf("bad id %q", id)
		}
		ids[id] = true
	}
	eq(t, len(ids), 50, "unique ids")
}

func TestEventMapKeepsFirstSetOrder(t *testing.T) {
	var m EventMap[string]
	m.Set("a", EventAppend)
	m.Set("b", EventAppend)
	m.Set("a", EventRemove)
	eqNames(t, m.Keys(), "a", "b")
	if got, _ := m.Get("a"); got != EventRemove {
		t.Fatal("last value wins")
	}
}

func TestUpdateNameThroughStoreUsesTheItemParentAsNotifier(t *testing.T) {
	store, diagram := makeStore()
	listener := &recordingListener{}
	store.OnUpdate = []IElementModelListener{listener}
	item := makeItem("a", 0, 0)
	diagram.AddItem(item)
	must(t, store.UpdateName(item, "b", nil, noop))
	eq(t, listener.events[0].Element, diagram, "notifier")
}

// Store operations followed by save

func TestStoreOperationsThenSaveProduceTheExpectedTenn(t *testing.T) {
	store, diagram := makeStore()
	um := &UndoManager{}
	a, b := makeItem("A", 10, 20), makeItem("B", 30, 40)
	must(t, store.AddItems(diagram, []*DiagramItem{a, b}, um, noop))
	must(t, store.AddLink(diagram, a, b, um, noop, nil))
	must(t, store.UpdateName(a, "Alpha", um, noop))
	must(t, store.UpdatePosition(b, Point{50, 60}, um, noop))
	must(t, store.UpdateElementName(diagram, "Main", um, noop))

	expected := strings.Join([]string{
		`element "Main" {`,
		`    item "Alpha" {`,
		"        pos 10.0 20.0",
		"    }",
		`    item "B" {`,
		"        pos 50.0 60.0",
		"    }",
		`    link "Alpha" "B"`,
		"}",
	}, "\n")
	got := saved(store.Model)
	eq(t, got, expected, "saved")

	// Every step undone gives the saved text of the empty diagram; redo gives the same text again.
	for um.CanUndo() {
		must(t, um.Undo())
	}
	eq(t, saved(store.Model), "element \"Diagram\" {\n}", "all undone")
	for um.CanRedo() {
		must(t, um.Redo())
	}
	eq(t, saved(store.Model), expected, "all redone")

	// And the saved text loads back into an equivalent model.
	loaded := ReadTenn(got)
	if loaded == nil {
		t.Fatal("saved text does not load")
	}
	eq(t, saved(loaded), got, "round trip")
}

// Model helpers (ElementModel.swift parts used by the operations)

func TestElementCloneIsDeepAndRelinksLinks(t *testing.T) {
	_, diagram := makeStore()
	a, b := makeItem("A", 1, 2), makeItem("B", 0, 0)
	diagram.AddSourceTarget(a, b)
	diagram.AddElement(NewElement("Sub"))
	d := "d"
	diagram.Description = &d

	diagram.Elements[0].AddItem(makeItem("Deep", 0, 0))
	linkIdx := slices.IndexFunc(diagram.Items, func(i *DiagramItem) bool { return i.IsLink() })
	diagram.Items[linkIdx].Properties.AppendContentsOf(parseProps("color red").Children)

	cp := diagram.Clone(true, true)
	for _, i := range cp.Items {
		if slices.ContainsFunc(diagram.Items, func(o *DiagramItem) bool { return o.ID == i.ID }) {
			t.Fatal("item ids must differ")
		}
		if i.Parent != cp {
			t.Fatal("parent not reassigned")
		}
	}
	cpLink := cp.Items[slices.IndexFunc(cp.Items, func(i *DiagramItem) bool { return i.IsLink() })]
	if cpLink.Properties.Get("color") == nil {
		t.Fatal("link properties copied")
	}
	if cp.Elements[0].Items[0] == diagram.Elements[0].Items[0] {
		t.Fatal("nested items shared")
	}
	eq(t, cp.Elements[0].Items[0].Name, "Deep", "nested name")
	eq(t, cp.Elements[0].Items[0].Parent, cp.Elements[0], "nested parent")
	eq(t, cp.Name, "Diagram", "name")
	if cp.Description == nil || *cp.Description != "d" {
		t.Fatal("description")
	}
	if cp.ID == diagram.ID {
		t.Fatal("id")
	}
	eq(t, len(cp.Items), 3, "items")
	eq(t, len(cp.Elements), 1, "elements")
	eq(t, cp.Elements[0].Parent, cp, "element parent")
	if !slices.Contains(cp.Items, cpLink.Source) || cpLink.Source == a || !slices.Contains(cp.Items, cpLink.Target) || cpLink.Target == b {
		t.Fatal("link ends must be cloned items")
	}
	eq(t, cpLink.Parent, cp, "link parent")

	cp.Items[0].Name = "changed"
	eq(t, a.Name, "A", "Mutating the clone must not touch the original")

	eq(t, len(diagram.Clone(false, false).Items), 0, "no items")
	eq(t, len(diagram.Clone(true, false).Elements), 0, "no elements")
}

func TestCloneLinkEndpointOutsideTheElementBecomesNil(t *testing.T) {
	_, diagram := makeStore()
	outside, inside := makeItem("Out", 0, 0), makeItem("In", 0, 0)
	diagram.AddItem(inside)
	diagram.AddItem(NewLinkItem(ItemKindLink, "", inside, outside))
	cp := diagram.Clone(true, true)
	link := cp.Items[slices.IndexFunc(cp.Items, func(i *DiagramItem) bool { return i.IsLink() })]
	if link.Target != nil {
		t.Fatal("target must be nil")
	}
	if link.Source == inside || link.Source.Name != "In" {
		t.Fatal("source must be the cloned item")
	}
}

func TestFindLinksGetRelatedItemsRemove(t *testing.T) {
	_, diagram := makeStore()
	a, b, c := makeItem("A", 0, 0), makeItem("B", 0, 0), makeItem("C", 0, 0)
	diagram.AddItem(c)
	diagram.AddSourceTarget(a, b)
	link := diagram.Items[1]

	eqItems(t, diagram.FindLinks(a), link)
	eq(t, len(diagram.FindLinks(c)), 0, "no links")
	eqItems(t, diagram.GetRelatedItems(a, true, false), link, a)
	eqItems(t, diagram.GetRelatedItems(a, false, true), a) // a is not the target of its link
	eqItems(t, diagram.GetRelatedItems(b, false, true), link, b)

	eq(t, diagram.RemoveItem(b), 3, "index")
	eq(t, diagram.RemoveItem(b), -1, "missing")
	eq(t, diagram.RemoveElement(NewElement("none")), -1, "missing element")
}

func TestAddGetAddMakeItemCounts(t *testing.T) {
	_, diagram := makeStore()
	item := makeItem("X", 0, 0)
	eq(t, diagram.AddGet(item), item, "addGet")
	sub := NewElement("Sub")
	made := diagram.AddMakeItem(sub)
	eq(t, made.Name, "Sub", "name")
	eq(t, made.Parent, diagram, "parent")
	eq(t, diagram.ItemCount(), 2, "item count")
	eq(t, diagram.Count(), 1, "count")
}
