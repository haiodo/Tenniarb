// Package editor is the editing logic of Tenniarb without a window: selection, drag, link drawing,
// item and element commands, properties and styles, undo. It is a port of web/packages/editor
// (session.ts and the pure parts around it), which ports SceneDrawView and ViewController.
//
// A window feeds a Session with plain events in scene coordinates (y up, the space of go/render):
// MouseDown, MouseMove, MouseUp, Key and the text of an inline editor. Every change of the model
// goes through model.ElementModelStore, so undo, notifications and recalculation work as in the
// Swift app. The Session draws nothing on the diagram; Overlay gives the selection as a separate
// gfx picture, so a window redraws the selection without rebuilding the scene.
//
// A Session is not safe for concurrent use.
package editor

import (
	"errors"
	"slices"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Redraw says what changed on the way to a repaint. The selection overlay is always to be drawn
// again.
type Redraw struct {
	// Rebuilt: Scene is a new scene, realize it again.
	Rebuilt bool
	// Moved lists drawables of the current scene whose picture changed (a drag preview).
	Moved []render.Drawable
}

// Options configure a Session.
type Options struct {
	// Render builds the scene. Its Exec is the expression engine; when nil the session makes one
	// (render.NewExecContext) unless NoEvaluate is set. Items and FirstID are ignored.
	Render render.Options
	// Readonly: no selection and no edits.
	Readonly bool
	// PCKeys is the TS mapping for other systems: Ctrl toggles the selection, Ctrl+Alt draws a
	// link, Alt drags with the items linked from the dragged one. Default (Swift): Cmd, Ctrl, Option.
	PCKeys bool
	// OnChange gets the serialized document after each committed change, undo and redo included.
	OnChange func(text string)
	// OnModelChange is OnChange without the text, for a caller that writes the document itself.
	OnModelChange func()
	// OnRedraw is called when the scene, the selection or a preview changed.
	OnRedraw func(Redraw)
	// OnElement is called when the edited element changes (SetElement, or the old one was removed).
	OnElement func()
	// OnEdit asks the window to open its inline editor on item (double click, Enter): call
	// BeginEdit, show the text and call FinishEdit.
	OnEdit func(item *model.DiagramItem, mode EditMode)
	// OnOperation asks for the operation box (Space) over the selection: call Operate with its text.
	OnOperation func()
	// OnError gets what the store reports, in Swift nothing: a bad index of an undo step.
	OnError func(error)
}

type mode int

const (
	modeNone mode = iota
	modeDrag
	modeBand
	modeLine
	modePan
)

// EditMode is what an inline editor changes: the name, the body or the value field of an item.
type EditMode string

const (
	EditName  EditMode = "name"
	EditBody  EditMode = "body"
	EditValue EditMode = "value"
)

// dragSlop (scene units): below it a press is a click, not a drag. Swift uses 0.2 s of time
// instead.
const dragSlop = 2

// undoDepth is how many undo steps a session keeps; every step holds the state it replaced.
const undoDepth = 200

type itemPos struct {
	item *model.DiagramItem
	pos  render.Point
}

type editing struct {
	item *model.DiagramItem
	mode EditMode
	rect render.Rect
}

// Session is the editor state of one document: the element on the canvas, its scene, the selection.
type Session struct {
	store *model.ElementModelStore
	um    *model.UndoManager
	root  *model.ElementModel
	exec  model.IExecutionContext
	opts  Options

	element   *model.Element
	scene     *render.DrawableScene
	selection []*model.DiagramItem
	band      *render.Rect

	elementCount int
	createIndex  int
	// pivot is where the next new item goes: the last click on empty space or right of the selection.
	pivot render.Point
	zoom  float64

	mode         mode
	origin, last render.Point
	moved        bool
	starts       []itemPos
	lineTarget   *model.DiagramItem
	clickCounter int
	// bandBase is the selection before a band started; a band released without Shift restores it.
	bandBase []*model.DiagramItem
	editing  *editing

	rebuilt bool
	dirty   []render.Drawable
	batch   int
	err     error
}

// New starts a session on element, which must belong to a model read or made by go/model.
func New(element *model.Element, opts Options) (*Session, error) {
	root := element
	for root.Parent != nil {
		root = root.Parent
	}
	if root.Kind != model.KindRoot || root.Model == nil {
		return nil, errors.New("editor: element is not attached to a model")
	}
	s := &Session{
		element:     element,
		opts:        opts,
		root:        root.Model,
		um:          &model.UndoManager{GroupsByEvent: true, MaxGroups: undoDepth},
		createIndex: 1,
		zoom:        1,
	}
	s.store = model.NewElementModelStore(root.Model)
	if opts.Render.Exec != nil {
		s.exec = opts.Render.Exec
		s.exec.SetElement(element)
	} else if !opts.Render.NoEvaluate {
		s.exec = render.NewExecContext(element, opts.Render)
	}
	if s.exec != nil {
		s.store.ExecutionContext = s.exec
	}
	s.rebuild()
	s.rebuilt = false
	return s, nil
}

// Store is the element store every change goes through; add listeners to its OnUpdate.
func (s *Session) Store() *model.ElementModelStore { return s.store }

// UndoManager is the session's undo history.
func (s *Session) UndoManager() *model.UndoManager { return s.um }

// Root is the document.
func (s *Session) Root() *model.ElementModel { return s.root }

// Element is the element on the canvas.
func (s *Session) Element() *model.Element { return s.element }

// Scene is the current scene, laid out. It is replaced after each change of the model.
func (s *Session) Scene() *render.DrawableScene { return s.scene }

// Selection is the selected items in the order they were selected. Do not change the slice.
func (s *Session) Selection() []*model.DiagramItem { return s.selection }

// Band is the rubber band being dragged, nil when none.
func (s *Session) Band() *render.Rect { return s.band }

// Pivot is where the next new item goes.
func (s *Session) Pivot() render.Point { return s.pivot }

// Readonly reports whether edits are refused.
func (s *Session) Readonly() bool { return s.opts.Readonly }

// Err is the last error the store reported, nil when none.
func (s *Session) Err() error { return s.err }

// Text is the whole document as .tenn text.
func (s *Session) Text() string { return model.ToTennStr(&s.root.Element, true, true) }

func (s *Session) fail(err error) {
	if err == nil {
		return
	}
	s.err = err
	if s.opts.OnError != nil {
		s.opts.OnError(err)
	}
}

// EndEvent closes the undo group of the current event. Every method of the session that changes the
// model calls it on return, so each is one undo step; Batch joins several.
func (s *Session) EndEvent() {
	if s.batch == 0 {
		s.um.CloseAutoGroup()
	}
}

// Batch runs f as one event: what it changes is one undo step.
func (s *Session) Batch(f func()) {
	s.batch++
	f()
	s.batch--
	s.EndEvent()
}

func (s *Session) emit() {
	if s.opts.OnRedraw == nil {
		s.rebuilt, s.dirty = false, nil
		return
	}
	r := Redraw{Rebuilt: s.rebuilt, Moved: s.dirty}
	s.rebuilt, s.dirty = false, nil
	s.opts.OnRedraw(r)
}

// refresh is the store's callback after every operation, undo and redo included.
func (s *Session) refresh() {
	if !s.attached(s.element) {
		s.setElement(s.fallback())
	} else {
		s.rebuild()
	}
	if s.opts.OnChange != nil {
		s.opts.OnChange(s.Text())
	}
	if s.opts.OnModelChange != nil {
		s.opts.OnModelChange()
	}
	s.emit()
}

func (s *Session) attached(e *model.Element) bool {
	for c := e; c.Parent != nil; c = c.Parent {
		if !slices.Contains(c.Parent.Elements, c) {
			return false
		}
	}
	return true
}

// fallback is the nearest ancestor of the element that is still in the model, else the model
// itself.
func (s *Session) fallback() *model.Element {
	c := s.element
	for c.Parent != nil && !s.attached(c) {
		c = c.Parent
	}
	return c
}

func (s *Session) rebuild() {
	o := s.opts.Render
	o.Exec, o.Items = s.exec, nil
	s.scene = render.Build(s.element, o)
	s.rebuilt = true
	kept := s.selection[:0:0]
	for _, i := range s.selection {
		if _, ok := s.scene.Drawables[i]; ok {
			kept = append(kept, i)
		}
	}
	// Swift setActiveItems after a removal: the pivot follows the first item left.
	if len(kept) != len(s.selection) && len(kept) > 0 {
		s.selection = kept
		s.pivotRightOf(kept[0])
	}
	s.selection = kept
	if s.editing != nil {
		if _, ok := s.scene.Drawables[s.editing.item]; !ok {
			s.editing = nil
		}
	}
	// Rebuilt under a drag (an update from outside): the dragged items stay under the pointer.
	if s.mode == modeDrag && s.moved {
		s.dragTo(s.last)
	}
}

// SetReadonly turns edits off (and drops the selection) or on.
func (s *Session) SetReadonly(readonly bool) {
	s.opts.Readonly = readonly
	if readonly {
		s.selection = nil
	}
	s.emit()
}

// SetDarkMode rebuilds the scene in the other scheme.
func (s *Session) SetDarkMode(dark bool) {
	s.opts.Render.DarkMode = dark
	s.rebuild()
	s.emit()
}

// SetZoom tells the session the view scale, which sizes the glow of the selection like Swift's
// shadow: in screen points. Call Overlay again after.
func (s *Session) SetZoom(k float64) {
	if k > 0 {
		s.zoom = k
	}
}

// SetElement edits another element of the document; the selection goes, the undo history stays.
func (s *Session) SetElement(element *model.Element) {
	s.setElement(element)
	s.emit()
}

func (s *Session) setElement(element *model.Element) {
	if element == s.element {
		return
	}
	s.element = element
	if s.exec != nil {
		s.exec.SetElement(element)
	}
	s.selection, s.editing = nil, nil
	s.pivot = render.Point{}
	s.rebuild()
	if s.opts.OnElement != nil {
		s.opts.OnElement()
	}
}

// Reveal shows item in its element and selects it.
func (s *Session) Reveal(item *model.DiagramItem) {
	if item.Parent == nil {
		return
	}
	s.setElement(item.Parent)
	s.selection = []*model.DiagramItem{item}
	s.emit()
}

func (s *Session) pivotRightOf(item *model.DiagramItem) {
	width := 90.0
	if d, ok := s.scene.Drawables[item]; ok {
		width = d.Bounds().Width
	}
	s.pivot = render.Point{X: item.X + width + 10, Y: item.Y}
}

// Undo and Redo do nothing when readonly or when there is nothing to undo.
func (s *Session) Undo() {
	if !s.opts.Readonly {
		s.fail(s.um.Undo())
	}
}

func (s *Session) Redo() {
	if !s.opts.Readonly {
		s.fail(s.um.Redo())
	}
}

// Target is what the properties panel edits: an item, or an element when Item is nil.
type Target struct {
	Item    *model.DiagramItem
	Element *model.Element
}

// PropsTarget is the first selected item, else the element (Swift activeItems.first).
func (s *Session) PropsTarget() Target {
	if len(s.selection) > 0 {
		return Target{Item: s.selection[0]}
	}
	return Target{Element: s.element}
}

func (t Target) props(kind tenn.NodeKind) *tenn.Node {
	if t.Item != nil {
		return model.ItemToTennAsProps(t.Item, kind, false)
	}
	return model.ElementToTennAsProps(t.Element, kind, false)
}

// PropsText is the target as the text the properties panel shows.
func (s *Session) PropsText(t Target) string {
	return tenn.ToStr(t.props(tenn.KindStatements), 0, false)
}

// ApplyProps is mergeProperties: one undo step. False: readonly, parse errors or the target is
// gone.
func (s *Session) ApplyProps(t Target, text string) bool {
	defer s.EndEvent()
	node, parser := tenn.Parse(text)
	if s.opts.Readonly || parser.Errors.HasErrors() {
		return false
	}
	if t.Item != nil {
		if !slices.Contains(s.element.Items, t.Item) {
			return false
		}
		s.fail(s.store.SetItemProperties(s.element, t.Item, node, s.um, s.refresh))
	} else {
		s.fail(s.store.SetProperties(t.Element, node, s.um, s.refresh))
	}
	return true
}

// PropsValues are the evaluated expressions of text by 0-based line (Swift updateAnnotations);
// empty when it does not parse or there is no engine.
func (s *Session) PropsValues(t Target, text string) map[int]string {
	values := map[int]string{}
	node, parser := tenn.Parse(text)
	if parser.Errors.HasErrors() || s.exec == nil {
		return values
	}
	var size *model.DrawableSize
	var evalTarget model.EvalTarget = t.Element
	if t.Item != nil {
		evalTarget = t.Item
		if d, ok := s.scene.Drawables[t.Item]; ok {
			b := d.SelectorBounds()
			size = &model.DrawableSize{Width: b.Width, Height: b.Height}
		}
	}
	for tok, v := range s.exec.GetEvaluated(evalTarget, node, size) {
		values[tok.Line] = render.ValueString(v)
	}
	return values
}
