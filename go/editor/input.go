package editor

import (
	"math"
	"slices"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Modifiers are the keys held during an event.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	// ModAlt is Option on macOS.
	ModAlt
	// ModMeta is Command on macOS.
	ModMeta
)

// Has reports whether m includes all of f.
func (m Modifiers) Has(f Modifiers) bool { return m&f == f }

// MouseEvent is a pointer event in scene coordinates (y up).
type MouseEvent struct {
	P    render.Point
	Mods Modifiers
	// Clicks is the click count of a press: 2 is a double click. Zero counts as 1.
	Clicks int
	// Cancel (MouseUp): the pointer was cancelled, a link being drawn is dropped.
	Cancel bool
}

// KeyEvent is a key press. Key is the DOM spelling: "Tab", "Enter", "Backspace", "Delete",
// "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", " " or the character.
type KeyEvent struct {
	Key  string
	Mods Modifiers
}

// keys maps the modifiers of a press to what Swift means by them.
func (s *Session) keys(m Modifiers) (toggle, line, alt bool) {
	if s.opts.PCKeys {
		ctrl, a := m.Has(ModCtrl), m.Has(ModAlt)
		return ctrl && !a, ctrl && a, a && !ctrl
	}
	return m.Has(ModMeta), m.Has(ModCtrl), m.Has(ModAlt)
}

// setActive is setActiveItems: the pivot goes right of the first item.
func (s *Session) setActive(items []*model.DiagramItem) {
	if len(items) == 0 && len(s.selection) == 0 {
		return
	}
	if len(items) > 0 {
		s.pivotRightOf(items[0])
	}
	s.selection = items
}

// itemsInRect are the selectable items whose bounds meet rect, in scene order.
func (s *Session) itemsInRect(rect render.Rect) []*model.DiagramItem {
	var out []*model.DiagramItem
	for _, i := range s.scene.Items() {
		if d := s.scene.Drawables[i]; d.Selectable() && rect.Intersects(d.Bounds()) {
			out = append(out, i)
		}
	}
	return out
}

// Hit is the items under p as a click finds them (Swift find): boxes before lines, in scene order.
func (s *Session) Hit(p render.Point) []*model.DiagramItem { return s.scene.Find(p, false) }

// MouseDown starts a press. It returns false when nothing was grabbed and the window may pan; the
// window then still sends MouseMove and MouseUp, which tell a click from a pan. Swift's modifiers:
// Shift starts a band, Cmd toggles, Option drags the items linked from the item too, Ctrl draws a
// link.
func (s *Session) MouseDown(e MouseEvent) bool {
	defer s.EndEvent()
	s.origin, s.moved, s.mode = e.P, false, modeNone
	s.starts, s.lineTarget = nil, nil
	if s.opts.Readonly {
		return false
	}
	// A press elsewhere ends the inline editor; the window commits or drops its text before.
	s.editing = nil
	toggle, line, alt := s.keys(e.Mods)
	if e.Mods.Has(ModShift) {
		s.mode = modeBand
		s.bandBase = slices.Clone(s.selection)
		s.selection = nil
		s.emit()
		return true
	}

	hits := s.scene.Find(e.P, false)
	// A selected item under the pointer wins over the others, so a selection can be dragged.
	var under []*model.DiagramItem
	if e.Clicks <= 1 {
		for _, a := range s.selection {
			d, ok := s.scene.Drawables[a]
			if !ok || !d.Bounds().Contains(e.P) {
				continue
			}
			if l, isLine := d.(*render.DrawableLine); !isLine || l.Find(e.P) {
				under = append(under, a)
			}
		}
	}
	if e.Clicks == 2 && len(hits) == 1 {
		if !slices.Contains(s.selection, hits[0]) {
			s.setActive([]*model.DiagramItem{hits[0]})
		}
		s.requestEdit(hits[0], EditName)
		s.emit()
		return true
	}
	if len(under) > 0 {
		hits = under
	}
	if len(hits) == 0 {
		// The selection stays through a pan; a click without movement clears it on MouseUp.
		s.mode = modePan
		s.pivot = e.P
		return false
	}

	if toggle {
		item := hits[0]
		if i := slices.Index(s.selection, item); i >= 0 {
			s.selection = slices.Delete(slices.Clone(s.selection), i, i+1)
		} else {
			s.selection = append(slices.Clone(s.selection), item)
		}
		s.setActive(s.selection)
		s.emit()
		return true
	}

	var fresh []*model.DiagramItem
	for _, h := range hits {
		if !slices.Contains(s.selection, h) {
			fresh = append(fresh, h)
		}
	}
	// Overlapping items are picked in turn by a counter shared by all clicks.
	if len(fresh) > 0 {
		s.clickCounter++
		s.setActive([]*model.DiagramItem{fresh[s.clickCounter%len(fresh)]})
	}
	drag := slices.Clone(s.selection)
	if alt && len(drag) == 1 {
		for _, from := range drag {
			for _, l := range s.element.Items {
				if l.Kind == model.ItemKindLink && l.IsLink() && l.Source == from && l.Target != nil && !slices.Contains(drag, l.Target) {
					drag = append(drag, l.Target)
				}
			}
		}
		s.setActive(drag)
	}
	if line && len(drag) == 1 {
		s.mode = modeLine
	} else {
		s.mode = modeDrag
		// Links move only alone, as in Swift.
		for _, i := range drag {
			if i.Kind == model.ItemKindItem || len(drag) == 1 {
				s.starts = append(s.starts, itemPos{i, render.Point{X: i.X, Y: i.Y}})
			}
		}
	}
	s.emit()
	return true
}

// MouseMove is a pointer move with the button held.
func (s *Session) MouseMove(e MouseEvent) { s.moveTo(e.P) }

func (s *Session) moveTo(p render.Point) {
	dx, dy := p.X-s.origin.X, p.Y-s.origin.Y
	if !s.moved && math.Hypot(dx, dy) < dragSlop {
		return
	}
	s.moved = true
	s.last = p
	switch s.mode {
	case modeBand:
		b := render.Rect{X: math.Min(s.origin.X, p.X), Y: math.Min(s.origin.Y, p.Y), Width: math.Abs(dx), Height: math.Abs(dy)}
		s.band = &b
		s.selection = s.itemsInRect(b)
	case modeLine:
		if len(s.selection) == 0 {
			return
		}
		source := s.selection[0]
		s.lineTarget = nil
		// Not the source itself: Swift would add a self-link there.
		if hits := s.scene.Find(p, false); len(hits) > 0 && hits[0].Kind == model.ItemKindItem && hits[0] != source {
			s.lineTarget = hits[0]
		}
		s.scene.UpdateLineTo(source, p, s.lineTarget)
	case modeDrag:
		s.dragTo(p)
	default:
		return
	}
	s.emit()
}

// dragTo moves the dragged drawables to where p puts them. The model is not touched until MouseUp.
func (s *Session) dragTo(p render.Point) {
	dx, dy := p.X-s.origin.X, p.Y-s.origin.Y
	positions := make(map[*model.DiagramItem]render.Point, len(s.starts))
	for _, st := range s.starts {
		positions[st.item] = render.Point{X: st.pos.X + dx, Y: st.pos.Y + dy}
	}
	for _, d := range s.scene.UpdateLayout(positions) {
		if !slices.Contains(s.dirty, d) {
			s.dirty = append(s.dirty, d)
		}
	}
}

// MouseUp ends a press. A drag is committed as one undo step, a link is added, a band is released.
func (s *Session) MouseUp(e MouseEvent) {
	defer s.EndEvent()
	s.moveTo(e.P)
	m := s.mode
	s.mode, s.band = modeNone, nil
	switch m {
	case modeBand:
		// Swift: a band released without Shift is dropped.
		if !e.Mods.Has(ModShift) {
			s.setActive(s.bandBase)
		}
		s.bandBase = nil
	case modePan:
		if !s.moved {
			s.setActive(nil)
		}
	case modeLine:
		var source *model.DiagramItem
		if len(s.selection) > 0 {
			source = s.selection[0]
		}
		if !e.Cancel && source != nil && s.lineTarget != nil && slices.Contains(s.element.Items, s.lineTarget) {
			s.fail(s.store.AddLink(s.element, source, s.lineTarget, s.um, s.refresh, nil))
		}
		s.scene.RemoveLineTo()
		s.lineTarget = nil
	case modeDrag:
		if s.moved {
			s.commitDrag()
		}
	}
	s.emit()
}

func (s *Session) commitDrag() {
	dx, dy := s.last.X-s.origin.X, s.last.Y-s.origin.Y
	var ops []model.ElementOperation
	for _, st := range s.starts {
		pos := model.Point{X: st.pos.X + dx, Y: st.pos.Y + dy}
		if (pos.X != st.item.X || pos.Y != st.item.Y) && slices.Contains(s.element.Items, st.item) {
			ops = append(ops, s.store.CreateUpdatePosition(st.item, pos))
		}
	}
	s.commit(ops)
}

// commit runs ops as one composite, one undo step.
func (s *Session) commit(ops []model.ElementOperation) {
	if len(ops) > 0 {
		s.fail(s.store.CompositeOperation(s.element, s.um, s.refresh, ops))
	}
}

// Pick is the right click: it selects what is under p unless something there is selected already
// (Swift menu(for:)); on empty space it clears the selection and moves the pivot there.
func (s *Session) Pick(p render.Point) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	hits := s.scene.Find(p, false)
	if !slices.ContainsFunc(hits, func(i *model.DiagramItem) bool { return slices.Contains(s.selection, i) }) {
		if len(hits) > 0 {
			s.setActive([]*model.DiagramItem{hits[0]})
		} else {
			s.setActive(nil)
		}
	}
	if len(s.selection) == 0 {
		s.pivot = p
	}
	s.emit()
}

// Key handles a key press of the canvas: undo, redo, duplicate, select all, Enter (edit), Delete,
// Tab, Space (operation box) and the arrows. It returns true when the key was taken. Copy, cut and
// paste are the window's: call CopyText, Cut and Paste.
func (s *Session) Key(e KeyEvent) bool {
	defer s.EndEvent()
	if s.editing != nil {
		return false
	}
	mod := e.Mods.Has(ModMeta) || e.Mods.Has(ModCtrl)
	shift, alt := e.Mods.Has(ModShift), e.Mods.Has(ModAlt)
	key := strings.ToLower(e.Key)
	switch {
	case mod && key == "z":
		if shift {
			s.Redo()
		} else {
			s.Undo()
		}
	case mod && key == "d":
		s.Duplicate()
	case mod && key == "a":
		if shift {
			s.SelectAll(model.ItemKindItem)
		} else {
			s.SelectAll("")
		}
	case !mod && e.Key == "Enter" && len(s.selection) > 0:
		mode := EditName
		if alt {
			mode = EditValue
		} else if shift {
			mode = EditBody
		}
		s.requestEdit(s.selection[0], mode)
	case !mod && (e.Key == "Backspace" || e.Key == "Delete" || e.Key == "x"):
		s.DeleteSelection()
	case !mod && e.Key == "Tab":
		s.AddNewItem(alt)
	case !mod && e.Key == " " && len(s.selection) > 0:
		if s.opts.OnOperation != nil && !s.opts.Readonly {
			s.opts.OnOperation()
		}
	case isArrow(e.Key) && len(s.selection) > 0:
		// Shift (from the edge) or Cmd (from the centre) resizes a single item, anything else moves the
		// selection.
		fromCentre := e.Mods.Has(ModMeta)
		if s.opts.PCKeys {
			fromCentre = e.Mods.Has(ModCtrl)
		}
		if (shift || fromCentre) && len(s.selection) == 1 {
			s.ResizeBy(e.Key, fromCentre)
		} else {
			dx, dy := arrowStep(e.Key)
			s.MoveBy(dx, dy)
		}
	default:
		return false
	}
	return true
}

func isArrow(key string) bool {
	return key == "ArrowLeft" || key == "ArrowRight" || key == "ArrowUp" || key == "ArrowDown"
}

// arrowStep is the direction of an arrow key; up is positive y.
func arrowStep(key string) (dx, dy float64) {
	switch key {
	case "ArrowLeft":
		return -1, 0
	case "ArrowRight":
		return 1, 0
	case "ArrowUp":
		return 0, 1
	}
	return 0, -1
}

func (s *Session) requestEdit(item *model.DiagramItem, mode EditMode) {
	if s.opts.OnEdit != nil && !s.opts.Readonly {
		s.opts.OnEdit(item, mode)
	}
}
