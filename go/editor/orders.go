package editor

import (
	"slices"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// The order of the items of a classic diagram is the order they are drawn in: the last one is on top. The
// commands below move the selection through that order, and through the layers the format names, as one
// undo step each.

// Raise moves every selected item one place up in the drawing order, the top item first so a block of
// neighbours keeps its own order.
func (s *Session) Raise() { s.orderStep(true) }

// Lower moves every selected item one place down in the drawing order.
func (s *Session) Lower() { s.orderStep(false) }

// orderStep is Raise and Lower: the items of the selection that are not in the element are left alone, and
// a move that would put an item past its neighbour of the selection does nothing.
func (s *Session) orderStep(forward bool) {
	defer s.EndEvent()
	if s.opts.Readonly || s.element == nil || len(s.selection) == 0 {
		return
	}
	items := s.element.Items
	sel := slices.Clone(s.selection)
	if forward {
		// From the top: the item that moves up leaves room for the one above it.
		for i := len(sel) - 1; i >= 0; i-- {
			it := sel[i]
			at := indexOfItem(items, it)
			if at < 0 || at+1 >= len(items) {
				continue
			}
			if s.holdsSelection(items[at+1]) {
				continue
			}
			next := at + 1
			s.commit(s.store.CreateUpdateOrder(it, &next))
		}
		return
	}
	for _, it := range sel {
		at := indexOfItem(items, it)
		if at <= 0 {
			continue
		}
		if s.holdsSelection(items[at-1]) {
			continue
		}
		next := at - 1
		s.commit(s.store.CreateUpdateOrder(it, &next))
	}
}

// ToFront puts the selected items on top, keeping their order among themselves; ToBack does the same at
// the bottom. The last selected item ends up on top, as it was selected last.
func (s *Session) ToFront() { s.orderEnd(true) }

// ToBack puts the selected items at the bottom of the drawing order.
func (s *Session) ToBack() { s.orderEnd(false) }

func (s *Session) orderEnd(forward bool) {
	defer s.EndEvent()
	if s.opts.Readonly || s.element == nil || len(s.selection) == 0 {
		return
	}
	sel := slices.Clone(s.selection)
	if !forward {
		// The bottom of the block goes first, so the order of the selection stays as it was.
		slices.Reverse(sel)
	}
	for _, it := range sel {
		var at *int
		if !forward {
			at = model.IntPtr(0)
		}
		s.commit(s.store.CreateUpdateOrder(it, at))
	}
}

// holdsSelection reports whether the item is part of the selection being moved: two neighbours that both
// move keep their order, and neither passes the other.
func (s *Session) holdsSelection(it *model.DiagramItem) bool {
	return slices.Contains(s.selection, it)
}

// indexOfItem is the place of the item in the drawing order, -1 when it is not there.
func indexOfItem(items []*model.DiagramItem, it *model.DiagramItem) int {
	for i, x := range items {
		if x == it {
			return i
		}
	}
	return -1
}

// LayerUp and LayerDown move the selection through the layers of the format: background, default, hover.
// A background item is drawn under the others, a hover item above them, whatever the order says.
func (s *Session) LayerUp() { s.layerStep(1) }

// LayerDown moves the selection one layer down.
func (s *Session) LayerDown() { s.layerStep(-1) }

// layers is the order of the layers of the format, bottom to top.
var layers = [...]string{"background", "default", "hover"}

// layerStep writes the layer of every selected item one step along layers.
func (s *Session) layerStep(delta int) {
	defer s.EndEvent()
	if s.opts.Readonly || s.element == nil || len(s.selection) == 0 {
		return
	}
	for _, it := range slices.Clone(s.selection) {
		cur := layerOf(it)
		at := slices.Index(layers[:], cur)
		if at < 0 {
			at = 1 // unset is the default layer
		}
		next := at + delta
		if next < 0 || next >= len(layers) {
			continue
		}
		props := model.ItemToTennAsProps(it, tenn.KindBlockExpr, false)
		setCommand(props, "layer", OptionNodes("layer", layers[next])...)
		s.fail(s.store.SetItemProperties(s.element, it, props, s.um, s.refresh))
	}
}

// layerOf is the layer the item names, "default" when it names none.
func layerOf(it *model.DiagramItem) string {
	if p := it.Properties.Get("layer"); p != nil && len(p.Children) >= 2 {
		if s, ok := p.Children[1].GetIdentText(); ok && s != "" {
			return s
		}
	}
	return "default"
}
