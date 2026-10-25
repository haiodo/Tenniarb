package editor

import (
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/tenn"
)

// BodyText is SceneDrawView.getBodyText: `body "text"` or `body { text "..." }`, unprocessed.
func BodyText(item *model.DiagramItem) string {
	body := item.Properties.Get("body")
	if body == nil {
		return ""
	}
	node := body.GetChild(1)
	if node != nil && node.Kind == tenn.KindBlockExpr {
		t := node.GetNamedElement("text")
		node = nil
		if t != nil {
			node = t.GetChild(1)
		}
	}
	text, _ := render.GetString(node)
	return text
}

func fieldName(item *model.DiagramItem) string {
	if f := item.Properties.Get("field-name"); f != nil {
		if name, ok := f.GetIdent(1); ok {
			return name
		}
	}
	return "value"
}

// EditTarget is what an inline editor shows for an item.
type EditTarget struct {
	// Rect is the box of the editor in scene space: the item's box grown to 100 by 20, anchored at its
	// bottom-left corner as Swift does (the y-up frame origin).
	Rect render.Rect
	Text string
	// FontSize in scene units.
	FontSize float64
}

func (s *Session) editText(item *model.DiagramItem, mode EditMode) string {
	switch mode {
	case EditName:
		return item.Name
	case EditBody:
		return render.PrepareBodyText(BodyText(item))
	}
	var node *tenn.Node
	if v := item.Properties.Get(fieldName(item)); v != nil {
		node = v.GetChild(1)
	}
	text, _ := render.GetString(node)
	return render.PrepareBodyText(text)
}

func fontSizeOf(d render.Drawable) float64 {
	switch b := d.(type) {
	case *render.RoundBox:
		return b.Style.FontSize
	case *render.CircleBox:
		return b.Style.FontSize
	case *render.EmptyBox:
		return b.Style.FontSize
	case *render.InvisibleBox:
		return b.Style.FontSize
	case *render.DrawableLine:
		return b.Style.FontSize
	}
	return 18
}

// EditTarget gives the editor of item its box, text and font. False: readonly, or item is not in
// the scene.
func (s *Session) EditTarget(item *model.DiagramItem, mode EditMode) (EditTarget, bool) {
	d, ok := s.scene.Drawables[item]
	if s.opts.Readonly || !ok {
		return EditTarget{}, false
	}
	b := d.SelectorBounds()
	if l, isLine := d.(*render.DrawableLine); isLine {
		b = l.LabelBounds()
	}
	size := fontSizeOf(d)
	if mode != EditName {
		size -= 2
	}
	return EditTarget{
		Rect:     render.Rect{X: b.X, Y: b.Y, Width: max(b.Width, 100), Height: max(b.Height, 20)},
		Text:     s.editText(item, mode),
		FontSize: size,
	}, true
}

// BeginEdit marks item as being edited: its selection outline turns green and takes the size of the
// editor. Call FinishEdit when the editor closes.
func (s *Session) BeginEdit(item *model.DiagramItem, mode EditMode) (EditTarget, bool) {
	t, ok := s.EditTarget(item, mode)
	if ok {
		s.editing = &editing{item, mode, t.Rect}
		s.emit()
	}
	return t, ok
}

// FinishEdit closes the edit started by BeginEdit; with commit the text is stored.
func (s *Session) FinishEdit(text string, commit bool) {
	e := s.editing
	s.editing = nil
	if e != nil && commit {
		s.CommitEdit(e.item, e.mode, text)
	}
	s.emit()
}

// Editing is the item being edited, nil when none. Keys and presses meant for the canvas are
// ignored then.
func (s *Session) Editing() *model.DiagramItem {
	if s.editing == nil {
		return nil
	}
	return s.editing.item
}

// CommitEdit stores the name, body or value of item as one undo step; nothing when the text is
// unchanged.
func (s *Session) CommitEdit(item *model.DiagramItem, mode EditMode, text string) {
	defer s.EndEvent()
	if s.opts.Readonly || text == s.editText(item, mode) {
		return
	}
	if mode == EditName {
		s.fail(s.store.UpdateName(item, text, s.um, s.refresh))
		return
	}
	props := model.ItemToTennAsProps(item, tenn.KindBlockExpr, false)
	if mode == EditValue {
		setCommand(props, fieldName(item), valueNode(text))
		s.fail(s.store.SetItemProperties(s.element, item, props, s.um, s.refresh))
		return
	}
	body := props.GetNamedElement("body")
	if body == nil {
		var v *tenn.Node = tenn.NewStrNode(text)
		if strings.Contains(text, "\n") || strings.Contains(text, `\n`) {
			v = tenn.NewMarkdownNode(text)
		}
		props.Add(tenn.NewCommand("body", v))
	} else {
		block := body.GetChild(1)
		if block == nil {
			return
		}
		// A block body without a "text" entry is left alone, as in Swift setBody.
		target, name := body, "body"
		if block.Kind == tenn.KindBlockExpr {
			target, name = block.GetNamedElement("text"), "text"
		}
		if target == nil {
			return
		}
		target.Children = nil
		target.Add(tenn.NewIdent(name), tenn.NewMarkdownNode(text))
	}
	s.fail(s.store.SetItemProperties(s.element, item, props, s.um, s.refresh))
}
