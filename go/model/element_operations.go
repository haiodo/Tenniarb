package model

import (
	"fmt"
	"slices"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Port of web/packages/core/src/element-operations.ts (ElementOperations.swift). Listeners run synchronously.
// TS class hierarchies become the ElementOperation interface plus OpBase, which carries the shared state and
// the default behaviour. Where TS throws (a bad index), Apply, Undo and Execute return an error.

type ModelEventKind string

const (
	EventStructure ModelEventKind = "Structure"
	EventLayout    ModelEventKind = "Layout"
)

type ModelEventOperation string

const (
	EventAppend ModelEventOperation = "Append"
	EventRemove ModelEventOperation = "Remove"
	EventUpdate ModelEventOperation = "Update"
)

type Point struct {
	X float64
	Y float64
}

// EventMap is a JS Map: keys keep the order of their first Set, a repeated Set only replaces the value.
type EventMap[K comparable] struct {
	keys []K
	ops  map[K]ModelEventOperation
}

func (m *EventMap[K]) Set(key K, op ModelEventOperation) {
	if m.ops == nil {
		m.ops = map[K]ModelEventOperation{}
	}
	if _, ok := m.ops[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.ops[key] = op
}

func (m *EventMap[K]) Get(key K) (ModelEventOperation, bool) {
	op, ok := m.ops[key]
	return op, ok
}

func (m *EventMap[K]) Len() int { return len(m.keys) }

func (m *EventMap[K]) Keys() []K { return m.keys }

type ModelEvent struct {
	Kind     ModelEventKind
	Element  *Element
	Items    EventMap[*DiagramItem]
	Elements EventMap[*Element]
}

func NewModelEvent(kind ModelEventKind, element *Element) *ModelEvent {
	return &ModelEvent{Kind: kind, Element: element}
}

// OpBase is the state and defaults of TS ElementOperation. UndoCalled starts true, so a fresh operation
// reports the undo direction until it is applied.
type OpBase struct {
	Store      *ElementModelStore
	UndoCalled bool
}

func newOpBase(store *ElementModelStore) OpBase {
	return OpBase{Store: store, UndoCalled: true}
}

func (b *OpBase) Base() *OpBase                          { return b }
func (b *OpBase) Name() string                           { return "Unnamed" }
func (b *OpBase) Apply() error                           { return nil }
func (b *OpBase) Undo() error                            { return nil }
func (b *OpBase) GetNotifier() *Element                  { return &b.Store.Model.Element }
func (b *OpBase) GetEventKind() ModelEventKind           { return EventStructure }
func (b *OpBase) CollectItems(_ *EventMap[*DiagramItem]) {}
func (b *OpBase) CollectElements(_ *EventMap[*Element])  {}

type ElementOperation interface {
	Base() *OpBase
	Name() string
	Apply() error
	Undo() error
	GetNotifier() *Element
	GetEventKind() ModelEventKind
	CollectItems(items *EventMap[*DiagramItem])
	CollectElements(elements *EventMap[*Element])
}

// CompositeOperation is a composite operation for Element inside changes.
type CompositeOperation struct {
	OpBase
	Operations []ElementOperation
	Notifier   *Element
}

func NewCompositeOperation(store *ElementModelStore, notifier *Element, ops ...ElementOperation) *CompositeOperation {
	return &CompositeOperation{OpBase: newOpBase(store), Operations: slices.Clone(ops), Notifier: notifier}
}

func (c *CompositeOperation) Add(ops ...ElementOperation) {
	c.Operations = append(c.Operations, ops...)
}

// GetEventKind is the common kind of the children, Structure when they differ or there are none.
func (c *CompositeOperation) GetEventKind() ModelEventKind {
	if len(c.Operations) == 0 {
		return EventStructure
	}
	kind := c.Operations[0].GetEventKind()
	for _, op := range c.Operations {
		if kind != op.GetEventKind() {
			return EventStructure
		}
	}
	return kind
}

func (c *CompositeOperation) Name() string {
	names := make([]string, 0, len(c.Operations))
	for _, op := range c.Operations {
		names = append(names, op.Name())
	}
	return strings.Join(names, ",")
}

func (c *CompositeOperation) Apply() error {
	for _, op := range c.Operations {
		if err := op.Apply(); err != nil {
			return err
		}
		op.Base().UndoCalled = false
	}
	return nil
}

func (c *CompositeOperation) Undo() error {
	for _, op := range slices.Backward(c.Operations) {
		if err := op.Undo(); err != nil {
			return err
		}
		op.Base().UndoCalled = true
	}
	return nil
}

func (c *CompositeOperation) GetNotifier() *Element { return c.Notifier }

func (c *CompositeOperation) CollectItems(items *EventMap[*DiagramItem]) {
	for _, op := range c.Operations {
		op.CollectItems(items)
	}
}

func (c *CompositeOperation) CollectElements(elements *EventMap[*Element]) {
	for _, op := range c.Operations {
		op.CollectElements(elements)
	}
}

type IElementModelListener interface {
	NotifyChanges(event *ModelEvent)
}

// ListenerFunc adapts a function to IElementModelListener.
type ListenerFunc func(event *ModelEvent)

func (f ListenerFunc) NotifyChanges(event *ModelEvent) { f(event) }

// EvalTarget is what GetEvaluated takes: *Element or *DiagramItem (TS Element | DiagramItem).
type EvalTarget interface {
	evalTarget()
}

func (*Element) evalTarget()     {}
func (*DiagramItem) evalTarget() {}

// DrawableSize is the size the scene gives a node for GetEvaluated.
type DrawableSize struct {
	Width  float64
	Height float64
}

// IExecutionContext is the part of Swift ExecutionContext the app calls.
type IExecutionContext interface {
	SetScaleFactor(factor float64)
	SetElement(element *Element)
	UpdateAll(notifier func())
	NotifyChanges(event *ModelEvent)
	// GetEvaluated returns, with node, the values of that node evaluated in the item/element scope; node and
	// drawable may be nil. Keys are token identities, as in TS.
	GetEvaluated(target EvalTarget, node *tenn.Node, drawable *DrawableSize) map[*tenn.Token]any
}

// ElementModelStore is how the UI changes the model: undo/redo, listener notifications and recalculation.
type ElementModelStore struct {
	Model *ElementModel

	OnUpdate         []IElementModelListener
	Modified         bool
	ExecutionContext IExecutionContext

	pending   []*ModelEvent
	notifying bool
}

func NewElementModelStore(model *ElementModel) *ElementModelStore {
	return &ElementModelStore{Model: model}
}

// IntPtr makes the optional index of AddElement and AddItem (TS number | null).
func IntPtr(i int) *int { return &i }

// Execute applies the action, or undoes it when it was applied before, so the same action toggles between
// undo and redo. A nil undoManager or refresh is skipped.
func (s *ElementModelStore) Execute(action ElementOperation, undoManager *UndoManager, refresh func()) error {
	base := action.Base()
	if !base.UndoCalled {
		if err := action.Undo(); err != nil {
			return err
		}
		base.UndoCalled = true
	} else {
		if err := action.Apply(); err != nil {
			return err
		}
		base.UndoCalled = false
	}
	// Registered after apply so a failing op leaves no undo entry. The same action is re-registered, so undo/redo toggle.
	if undoManager != nil {
		undoManager.RegisterUndo(func() error { return s.Execute(action, undoManager, refresh) })
	}

	evt := NewModelEvent(action.GetEventKind(), action.GetNotifier())
	action.CollectItems(&evt.Items)
	action.CollectElements(&evt.Elements)

	if s.ExecutionContext != nil {
		s.ExecutionContext.NotifyChanges(evt)
	}
	s.ModifiedEvent(evt)
	if refresh != nil {
		refresh()
	}
	return nil
}

// UpdateName is Swift updateName(item:).
func (s *ElementModelStore) UpdateName(item *DiagramItem, newName string, undoManager *UndoManager, refresh func()) error {
	return s.Execute(NewUpdateName(s, item.Parent, item, item.Name, newName), undoManager, refresh)
}

// UpdateElementName is Swift updateName(element:).
func (s *ElementModelStore) UpdateElementName(element *Element, newName string, undoManager *UndoManager, refresh func()) error {
	return s.Execute(NewUpdateElementName(s, element, element.Name, newName), undoManager, refresh)
}

func (s *ElementModelStore) UpdatePosition(item *DiagramItem, newPos Point, undoManager *UndoManager, refresh func()) error {
	return s.Execute(s.CreateUpdatePosition(item, newPos), undoManager, refresh)
}

func (s *ElementModelStore) CreateUpdatePosition(item *DiagramItem, newPos Point) ElementOperation {
	return NewUpdatePosition(s, item.Parent, item, Point{item.X, item.Y}, newPos)
}

// CreateUpdateOrder moves the item to newPos, to the end when nil.
func (s *ElementModelStore) CreateUpdateOrder(item *DiagramItem, newPos *int) []ElementOperation {
	return []ElementOperation{NewRemoveItem(s, item.Parent, item), NewAddItem(s, item.Parent, item, newPos)}
}

func (s *ElementModelStore) CompositeOperation(notifier *Element, undoManager *UndoManager, refresh func(), operations []ElementOperation) error {
	return s.Execute(NewCompositeOperation(s, notifier, operations...), undoManager, refresh)
}

// Add is Swift add(_ parent, _ child: Element).
func (s *ElementModelStore) Add(parent, child *Element, undoManager *UndoManager, refresh func(), index *int) error {
	return s.Execute(NewAddElement(s, parent, child, index), undoManager, refresh)
}

// AddElements ignores the index, as Swift does, so it has none here.
func (s *ElementModelStore) AddElements(parent *Element, childs []*Element, undoManager *UndoManager, refresh func()) error {
	ops := make([]ElementOperation, 0, len(childs))
	for _, el := range childs {
		ops = append(ops, NewAddElement(s, parent, el, nil))
	}
	return s.CompositeOperation(parent, undoManager, refresh, ops)
}

func (s *ElementModelStore) Move(element, newParent *Element, undoManager *UndoManager, refresh func(), index int) error {
	op := NewCompositeOperation(s, element)

	op.Add(NewRemoveElement(s, element.Parent, element))
	op.Add(NewAddElement(s, newParent, element, &index))

	return s.Execute(op, undoManager, refresh)
}

func (s *ElementModelStore) Remove(parent, child *Element, undoManager *UndoManager, refresh func()) error {
	return s.Execute(NewRemoveElement(s, parent, child), undoManager, refresh)
}

// AddItem is Swift add(_ element, _ item: DiagramItem).
func (s *ElementModelStore) AddItem(element *Element, item *DiagramItem, undoManager *UndoManager, refresh func()) error {
	return s.Execute(NewAddItem(s, element, item, nil), undoManager, refresh)
}

// AddItems is Swift add(_ element, _ items: [DiagramItem]).
func (s *ElementModelStore) AddItems(element *Element, items []*DiagramItem, undoManager *UndoManager, refresh func()) error {
	ops := make([]ElementOperation, 0, len(items))
	for _, itm := range items {
		ops = append(ops, NewAddItem(s, element, itm, nil))
	}
	return s.Execute(NewCompositeOperation(s, element, ops...), undoManager, refresh)
}

// AddLink is Swift add(_ element, source:, target:). Endpoints missing from the element are added before the link.
func (s *ElementModelStore) AddLink(element *Element, source, target *DiagramItem, undoManager *UndoManager, refresh func(), props []*tenn.Node) error {
	op := NewCompositeOperation(s, element)

	link := NewLinkItem(ItemKindLink, "", source, target)
	link.Properties.AppendContentsOf(props)

	if !slices.Contains(element.Items, source) {
		op.Add(NewAddItem(s, element, source, nil))
	}

	if !slices.Contains(element.Items, target) {
		op.Add(NewAddItem(s, element, target, nil))
	}

	op.Add(NewAddItem(s, element, link, nil))

	return s.Execute(op, undoManager, refresh)
}

// RemoveItem is Swift remove(_ element, item:); the links of the item go with it.
func (s *ElementModelStore) RemoveItem(element *Element, item *DiagramItem, undoManager *UndoManager, refresh func()) error {
	op := NewCompositeOperation(s, element)
	for _, it := range element.GetRelatedItems(item, true, true) {
		op.Add(NewRemoveItem(s, element, it))
	}
	return s.Execute(op, undoManager, refresh)
}

// RemoveItems is Swift remove(_ element, items:); an item related to several deleted ones is removed once.
func (s *ElementModelStore) RemoveItems(element *Element, deleteItems []*DiagramItem, undoManager *UndoManager, refresh func()) error {
	op := NewCompositeOperation(s, element)

	var items []*DiagramItem

	for _, itm := range deleteItems {
		for _, rel := range element.GetRelatedItems(itm, true, true) {
			if !slices.Contains(items, rel) {
				items = append(items, rel)
			}
		}
	}

	for _, item := range items {
		op.Add(NewRemoveItem(s, element, item))
	}
	return s.Execute(op, undoManager, refresh)
}

func (s *ElementModelStore) MakeNonModified() {
	s.Modified = false
}

// ModifiedEvent is Swift modified(_ event); renamed because the dirty flag is called Modified too.
func (s *ElementModelStore) ModifiedEvent(event *ModelEvent) {
	s.Modified = true
	// A listener may execute again: queue the inner event so every listener sees events in production order.
	s.pending = append(s.pending, event)
	if s.notifying {
		return
	}
	s.notifying = true
	defer func() {
		s.notifying = false
		s.pending = nil
	}()
	for len(s.pending) > 0 {
		e := s.pending[0]
		s.pending = s.pending[1:]
		for _, op := range slices.Clone(s.OnUpdate) {
			op.NotifyChanges(e)
		}
	}
}

// SetProperties is Swift setProperties(_ element, _ node).
func (s *ElementModelStore) SetProperties(element *Element, node *tenn.Node, undoManager *UndoManager, refresh func()) error {
	old := ElementToTennAsProps(element, tenn.KindStatements, false)
	return s.Execute(NewComplexUpdateElement(s, element, old, node), undoManager, refresh)
}

// SetItemProperties is Swift setProperties(_ element, _ item, _ node).
func (s *ElementModelStore) SetItemProperties(element *Element, item *DiagramItem, node *tenn.Node, undoManager *UndoManager, refresh func()) error {
	return s.Execute(s.CreateProperties(element, item, node), undoManager, refresh)
}

func (s *ElementModelStore) CreateProperties(element *Element, item *DiagramItem, node *tenn.Node) ElementOperation {
	old := ItemToTennAsProps(item, tenn.KindStatements, false)
	return NewComplexUpdateItem(s, element, item, old, node)
}

// updateItemValue is TS AbstractUpdateValue: apply sets the new value, undo the old one.
type updateItemValue[V any] struct {
	OpBase
	Element  *Element
	Item     *DiagramItem
	OldValue V
	NewValue V
	set      func(V)
}

func (u *updateItemValue[V]) Apply() error {
	u.set(u.NewValue)
	return nil
}

func (u *updateItemValue[V]) Undo() error {
	u.set(u.OldValue)
	return nil
}

// CollectItems reports Update regardless of direction.
func (u *updateItemValue[V]) CollectItems(items *EventMap[*DiagramItem]) {
	items.Set(u.Item, EventUpdate)
}

func (u *updateItemValue[V]) GetNotifier() *Element { return u.Element }

// updateElementValue is TS AbstractUpdateElementValue.
type updateElementValue[V any] struct {
	OpBase
	Element  *Element
	OldValue V
	NewValue V
	set      func(V)
}

func (u *updateElementValue[V]) Apply() error {
	u.set(u.NewValue)
	return nil
}

func (u *updateElementValue[V]) Undo() error {
	u.set(u.OldValue)
	return nil
}

func (u *updateElementValue[V]) GetNotifier() *Element { return u.Element }

type UpdatePosition struct {
	updateItemValue[Point]
}

func NewUpdatePosition(store *ElementModelStore, element *Element, item *DiagramItem, old, newValue Point) *UpdatePosition {
	return &UpdatePosition{updateItemValue[Point]{
		OpBase: newOpBase(store), Element: element, Item: item, OldValue: old, NewValue: newValue,
		set: func(p Point) { item.X, item.Y = p.X, p.Y },
	}}
}

// Name keeps the Swift string, including the missing space before NEW.
func (u *UpdatePosition) Name() string {
	return fmt.Sprintf("UpdatePosition: %s OLD:( %s, %sNEW:( %s, %s ", u.Item.Name,
		tenn.SwiftDouble(u.OldValue.X), tenn.SwiftDouble(u.OldValue.Y),
		tenn.SwiftDouble(u.NewValue.X), tenn.SwiftDouble(u.NewValue.Y))
}

type UpdateName struct {
	updateItemValue[string]
}

func NewUpdateName(store *ElementModelStore, element *Element, item *DiagramItem, old, newValue string) *UpdateName {
	return &UpdateName{updateItemValue[string]{
		OpBase: newOpBase(store), Element: element, Item: item, OldValue: old, NewValue: newValue,
		set: func(v string) { item.Name = v },
	}}
}

func (u *UpdateName) Name() string { return "UpdateName" }

type ComplexUpdateItem struct {
	updateItemValue[*tenn.Node]
}

func NewComplexUpdateItem(store *ElementModelStore, element *Element, item *DiagramItem, old, newValue *tenn.Node) *ComplexUpdateItem {
	return &ComplexUpdateItem{updateItemValue[*tenn.Node]{
		OpBase: newOpBase(store), Element: element, Item: item, OldValue: old, NewValue: newValue,
		set: func(n *tenn.Node) { ItemFromTennProps(item, n) },
	}}
}

func (u *ComplexUpdateItem) Name() string { return "UpdateItem" }

type ComplexUpdateElement struct {
	updateElementValue[*tenn.Node]
}

func NewComplexUpdateElement(store *ElementModelStore, element *Element, old, newValue *tenn.Node) *ComplexUpdateElement {
	return &ComplexUpdateElement{updateElementValue[*tenn.Node]{
		OpBase: newOpBase(store), Element: element, OldValue: old, NewValue: newValue,
		set: func(n *tenn.Node) { ElementFromTennProps(element, n) },
	}}
}

func (u *ComplexUpdateElement) Name() string { return "UpdateElement" }

type UpdateElementName struct {
	updateElementValue[string]
}

func NewUpdateElementName(store *ElementModelStore, element *Element, old, newValue string) *UpdateElementName {
	return &UpdateElementName{updateElementValue[string]{
		OpBase: newOpBase(store), Element: element, OldValue: old, NewValue: newValue,
		set: func(v string) { element.Name = v },
	}}
}

func (u *UpdateElementName) Name() string { return "UpdateElementName" }

type AddElement struct {
	OpBase
	Parent *Element
	Child  *Element
	Index  *int // nil or -1 appends
}

func NewAddElement(store *ElementModelStore, parent, child *Element, index *int) *AddElement {
	return &AddElement{OpBase: newOpBase(store), Parent: parent, Child: child, Index: index}
}

func (a *AddElement) Name() string { return "AddElement" }

func (a *AddElement) Apply() error {
	if a.Index == nil || *a.Index == -1 {
		a.Parent.AddElement(a.Child)
		return nil
	}
	return a.Parent.InsertElement(*a.Index, a.Child)
}

func (a *AddElement) Undo() error {
	a.Parent.RemoveElement(a.Child)
	return nil
}

func (a *AddElement) GetNotifier() *Element { return a.Parent }

func (a *AddElement) CollectElements(elements *EventMap[*Element]) {
	elements.Set(a.Child, appendUnless(a.UndoCalled))
}

type AddItem struct {
	OpBase
	Parent *Element
	Item   *DiagramItem
	At     *int // nil appends
}

func NewAddItem(store *ElementModelStore, parent *Element, item *DiagramItem, at *int) *AddItem {
	return &AddItem{OpBase: newOpBase(store), Parent: parent, Item: item, At: at}
}

func (a *AddItem) Name() string { return "AddItem" }

func (a *AddItem) Apply() error {
	if a.At == nil {
		a.Parent.AddItem(a.Item)
		return nil
	}
	return a.Parent.InsertItem(*a.At, a.Item)
}

func (a *AddItem) Undo() error {
	a.Parent.RemoveItem(a.Item)
	return nil
}

func (a *AddItem) GetNotifier() *Element { return a.Parent }

func (a *AddItem) CollectItems(items *EventMap[*DiagramItem]) {
	items.Set(a.Item, appendUnless(a.UndoCalled))
}

type RemoveElement struct {
	OpBase
	Parent      *Element
	Child       *Element
	RemoveIndex int
}

func NewRemoveElement(store *ElementModelStore, parent, child *Element) *RemoveElement {
	return &RemoveElement{OpBase: newOpBase(store), Parent: parent, Child: child, RemoveIndex: -1}
}

func (r *RemoveElement) Name() string { return "RemoveElement" }

func (r *RemoveElement) Apply() error {
	r.RemoveIndex = r.Parent.RemoveElement(r.Child)
	return nil
}

// Undo does nothing when Apply found nothing to remove.
func (r *RemoveElement) Undo() error {
	if r.RemoveIndex < 0 {
		return nil
	}
	return r.Parent.InsertElement(r.RemoveIndex, r.Child)
}

func (r *RemoveElement) GetNotifier() *Element { return r.Parent }

func (r *RemoveElement) CollectElements(elements *EventMap[*Element]) {
	elements.Set(r.Child, appendUnless(!r.UndoCalled))
}

type RemoveItem struct {
	OpBase
	Parent      *Element
	Child       *DiagramItem
	RemoveIndex int
}

func NewRemoveItem(store *ElementModelStore, parent *Element, child *DiagramItem) *RemoveItem {
	return &RemoveItem{OpBase: newOpBase(store), Parent: parent, Child: child, RemoveIndex: -1}
}

func (r *RemoveItem) Name() string { return "RemoveItem" }

func (r *RemoveItem) Apply() error {
	r.RemoveIndex = r.Parent.RemoveItem(r.Child)
	return nil
}

func (r *RemoveItem) Undo() error {
	if r.RemoveIndex < 0 {
		return nil
	}
	return r.Parent.InsertItem(r.RemoveIndex, r.Child)
}

func (r *RemoveItem) GetNotifier() *Element { return r.Parent }

func (r *RemoveItem) CollectItems(items *EventMap[*DiagramItem]) {
	items.Set(r.Child, appendUnless(!r.UndoCalled))
}

// appendUnless gives Append for false and Remove for true: the Add* operations pass UndoCalled,
// the Remove* ones its negation.
func appendUnless(removed bool) ModelEventOperation {
	if removed {
		return EventRemove
	}
	return EventAppend
}
