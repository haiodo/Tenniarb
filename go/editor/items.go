package editor

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/tenn"
)

// roundf is Swift roundf of a Float: the value goes through float32, halves round away from zero.
func roundf(v float64) float64 { return math.Round(float64(float32(v))) }

// Select makes items the selection (Swift setActiveItems); readonly: nothing.
func (s *Session) Select(items []*model.DiagramItem) {
	if s.opts.Readonly {
		return
	}
	s.setActive(items)
	s.emit()
}

// SelectAll selects the items of the element, only those of kind when it is not empty.
func (s *Session) SelectAll(kind model.ItemKind) {
	var items []*model.DiagramItem
	for _, i := range s.element.Items {
		if kind == "" || i.Kind == kind {
			items = append(items, i)
		}
	}
	s.Select(items)
}

// CopyText is the selection as .tenn text, the clipboard format of Swift; "" when nothing is
// selected.
func (s *Session) CopyText() string {
	if len(s.selection) == 0 {
		return ""
	}
	return tenn.ToStr(model.StoreItems(s.selection), 0, false)
}

// Cut copies the selection, then deletes it.
func (s *Session) Cut() string {
	text := s.CopyText()
	s.DeleteSelection()
	return text
}

// DeleteSelection removes the selected items with their links, one undo step.
func (s *Session) DeleteSelection() {
	defer s.EndEvent()
	if s.opts.Readonly || len(s.selection) == 0 {
		return
	}
	s.fail(s.store.RemoveItems(s.element, slices.Clone(s.selection), s.um, s.refresh))
	s.emit()
}

// Paste adds the items of a .tenn text as they were written (Swift does not offset them) and
// selects them. False: readonly, or the text holds no items.
func (s *Session) Paste(text string) bool {
	defer s.EndEvent()
	if s.opts.Readonly {
		return false
	}
	node, parser := tenn.Parse(text)
	items := model.ParseItems(node)
	if parser.Errors.HasErrors() || len(items) == 0 {
		return false
	}
	s.addSelected(items)
	return true
}

func (s *Session) addSelected(items []*model.DiagramItem) {
	if len(items) == 0 {
		return
	}
	s.fail(s.store.AddItems(s.element, items, s.um, s.refresh))
	s.setActive(items)
	s.emit()
}

// PasteImage is Swift's paste of an image: `image name data` and a title showing it, into the first
// selected item or a new one at the pivot. data is base64 of a PNG.
func (s *Session) PasteImage(name, data string) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	image := tenn.NewCommand("image", tenn.NewStrNode(name), tenn.NewImageNode(data))
	// Swift looks up an old title in a Statements node, never finds it and appends a second one; kept
	// for identical files.
	title := tenn.NewCommand("title", tenn.NewMarkdownNode("@("+name+"|96)\n${name}"))
	if len(s.selection) > 0 {
		active := s.selection[0]
		props := model.ItemToTennAsProps(active, tenn.KindStatements, false)
		props.Add(image, title)
		s.fail(s.store.SetItemProperties(s.element, active, props, s.um, s.refresh))
		return
	}
	item := model.NewDiagramItem(model.ItemKindItem, s.untitled())
	item.X, item.Y = s.pivot.X, s.pivot.Y
	item.Properties.Append(image)
	item.Properties.Append(title)
	s.fail(s.store.AddItem(s.element, item, s.um, s.refresh))
	s.setActive([]*model.DiagramItem{item})
	s.emit()
}

// AttachImage is Swift attachImage: `image name data` added to the first selected item.
func (s *Session) AttachImage(name, data string) {
	defer s.EndEvent()
	if s.opts.Readonly || len(s.selection) == 0 {
		return
	}
	active := s.selection[0]
	props := model.ItemToTennAsProps(active, tenn.KindBlockExpr, false)
	props.Add(tenn.NewCommand("image", tenn.NewStrNode(name), tenn.NewImageNode(data)))
	s.fail(s.store.SetItemProperties(s.element, active, props, s.um, s.refresh))
}

// PasteAsItem is Swift pasteAsItem: the text as the `text` of a new item at the pivot.
func (s *Session) PasteAsItem(text string) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	item := model.NewDiagramItem(model.ItemKindItem, "pasted "+strconv.Itoa(s.createIndex))
	s.createIndex++
	item.X, item.Y = s.pivot.X, s.pivot.Y
	item.Properties.Append(tenn.NewCommand("text", tenn.NewMarkdownNode(text)))
	item.Properties.Append(tenn.NewCommand("title", tenn.NewMarkdownNode("${text}")))
	s.fail(s.store.AddItem(s.element, item, s.um, s.refresh))
	s.setActive([]*model.DiagramItem{item})
	s.emit()
}

// PasteAsItemSet is Swift pasteAsItemSet: an item per line, the lines below the first are
// arrow-linked from it.
func (s *Session) PasteAsItemSet(text string) {
	defer s.EndEvent()
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if s.opts.Readonly || len(lines) == 0 {
		return
	}
	var items []*model.DiagramItem
	var root *model.DiagramItem
	for n, line := range lines {
		item := model.NewDiagramItem(model.ItemKindItem, line)
		s.createIndex++
		item.X, item.Y = s.pivot.X, s.pivot.Y-35*float64(n)
		if root == nil {
			root = item
		} else {
			link := model.NewLinkItem(model.ItemKindLink, "", root, item)
			link.Properties.Append(tenn.NewCommand("display", tenn.NewIdent("arrow")))
			items = append(items, link)
		}
		items = append(items, item)
	}
	s.fail(s.store.AddItems(s.element, items, s.um, s.refresh))
	s.emit()
}

// Duplicate is Swift duplicateItem: the selected items shifted right, the links that end at them
// cloned too.
func (s *Session) Duplicate() {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	copies := map[*model.DiagramItem]*model.DiagramItem{}
	var links, items []*model.DiagramItem
	seen := map[*model.DiagramItem]bool{}
	addLink := func(l *model.DiagramItem) {
		if l.Kind != model.ItemKindLink || seen[l] {
			return
		}
		seen[l] = true
		c := l.Clone()
		links = append(links, c)
		items = append(items, c)
	}
	for _, a := range s.selection {
		if a.Kind == model.ItemKindItem {
			c := a.Clone()
			c.X += 75
			copies[a] = c
			items = append(items, c)
		}
		addLink(a)
		for _, r := range s.element.GetRelatedItems(a, false, true) {
			addLink(r)
		}
	}
	for _, l := range links {
		if c, ok := copies[l.Source]; ok && l.Source != nil {
			l.Source = c
		}
		if c, ok := copies[l.Target]; ok && l.Target != nil {
			l.Target = c
		}
	}
	s.addSelected(items)
}

// untitled is a name for a new item that no item of the element has: links refer to items by name,
// so one pasted or made by another editor must not be repeated (Swift would).
func (s *Session) untitled() string {
	name := "Untitled " + strconv.Itoa(s.createIndex)
	s.createIndex++
	for slices.ContainsFunc(s.element.Items, func(i *model.DiagramItem) bool { return i.Kind == model.ItemKindItem && i.Name == name }) {
		name = "Untitled " + strconv.Itoa(s.createIndex)
		s.createIndex++
	}
	return name
}

// AddTopItem is Swift addTopItem (the menu "New item"): a new item at the pivot, selected.
func (s *Session) AddTopItem() { s.addItem(nil, false) }

// AddNewItem is Swift addNewItem (Tab): nothing selected - as AddTopItem; an item selected - a new
// item linked from it (copyProps: with its properties); a link selected - nothing.
func (s *Session) AddNewItem(copyProps bool) {
	if len(s.selection) == 0 {
		s.addItem(nil, false)
	} else if s.selection[0].Kind == model.ItemKindItem {
		s.addItem(s.selection[0], copyProps)
	}
}

func (s *Session) addItem(from *model.DiagramItem, copyProps bool) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	item := model.NewDiagramItem(model.ItemKindItem, s.untitled())
	item.X, item.Y = s.pivot.X, s.pivot.Y
	if copyProps && from != nil {
		for _, p := range from.Properties.Nodes() {
			item.Properties.Append(p.Clone())
		}
	}
	if from == nil {
		s.fail(s.store.AddItem(s.element, item, s.um, s.refresh))
	} else {
		s.fail(s.store.AddLink(s.element, from, item, s.um, s.refresh, nil))
	}
	s.setActive([]*model.DiagramItem{item})
	s.emit()
}

// MoveBy is the arrow keys of Swift keyDown: every selected item one grid step in the direction (dy
// up is positive) and snapped to the grid, one undo step.
func (s *Session) MoveBy(dx, dy float64) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	g := s.scene.Style.GridSpan
	// Swift drops the remainder of the truncated value, so negative coordinates snap toward zero.
	snap := func(v, grid float64) float64 {
		r := roundf(v)
		gi := int(grid)
		if gi == 0 {
			gi = 1 // Swift would trap on a zero step
		}
		return r - float64(int(r)%gi)
	}
	var ops []model.ElementOperation
	for _, i := range s.selection {
		pos := model.Point{X: i.X, Y: i.Y}
		if dx != 0 {
			pos.X = snap(i.X+dx*g.X, g.X)
		}
		if dy != 0 {
			pos.Y = snap(i.Y+dy*g.Y, g.Y)
		}
		ops = append(ops, s.store.CreateUpdatePosition(i, pos))
	}
	s.commit(ops)
}

// ResizeBy is Swift handleResizeItem: one grid step of width (left, right) or height (up, down) of
// the single selected item; fromCenter also shifts the item. Swift computes in Float, so does this.
func (s *Session) ResizeBy(key string, fromCenter bool) {
	defer s.EndEvent()
	if s.opts.Readonly || len(s.selection) != 1 {
		return
	}
	item := s.selection[0]
	var bw, bh float32 = 100, 50
	if d, ok := s.scene.Drawables[item]; ok {
		b := d.SelectorBounds()
		if rb, isBox := d.(*render.RoundBox); isBox {
			lw := rb.Style.LineWidth
			b = render.Rect{X: b.X + lw, Y: b.Y + lw, Width: b.Width - 2*lw, Height: b.Height - 2*lw}
		}
		bw, bh = float32(b.Width), float32(b.Height)
	}
	w, h := bw, bh
	if v, ok := propFloat(item, "width"); ok {
		w = v
	}
	if v, ok := propFloat(item, "height"); ok {
		h = v
	}
	gx, gy := float32(s.scene.Style.GridSpan.X), float32(s.scene.Style.GridSpan.Y)
	nw, nh, nx, ny := w, h, item.X, item.Y
	switch key {
	case "ArrowLeft":
		nw = max(10, w-gx)
		if fromCenter {
			nx = item.X + float64((w-nw)/2)
		}
	case "ArrowRight":
		nw = w + gx
		if fromCenter {
			nx = item.X - float64((nw-w)/2)
		}
	case "ArrowUp":
		nh = max(10, h-gy)
		if fromCenter {
			ny = item.Y - float64((h-nh)/2)
		}
	case "ArrowDown":
		nh = h + gy
		if fromCenter {
			ny = item.Y + float64((nh-h)/2)
		}
	default:
		return
	}
	var ops []model.ElementOperation
	if nw != w || nh != h {
		props := model.ItemToTennAsProps(item, tenn.KindBlockExpr, false)
		if nw != w {
			setCommand(props, "width", tenn.NewFloatNode(float64(nw)))
		}
		if nh != h {
			setCommand(props, "height", tenn.NewFloatNode(float64(nh)))
		}
		ops = append(ops, s.store.CreateProperties(s.element, item, props))
	}
	if fromCenter {
		ops = append(ops, s.store.CreateUpdatePosition(item, model.Point{X: nx, Y: ny}))
	}
	s.commit(ops)
}

func propFloat(item *model.DiagramItem, name string) (float32, bool) {
	n := item.Properties.Get(name)
	if n == nil {
		return 0, false
	}
	v, ok := n.GetFloat(1)
	return float32(v), ok
}

// setCommand gives the command name of props the values, adding it when absent.
func setCommand(props *tenn.Node, name string, values ...*tenn.Node) {
	if cur := props.GetNamedElement(name); cur != nil {
		cur.Children = nil
		cur.Add(tenn.NewIdent(name))
		cur.Add(values...)
		return
	}
	props.Add(tenn.NewCommand(name, values...))
}

// Align is alignLeadingEdges / alignTrailingEdges / alignTopEdges / alignBottomEdges of Swift: the
// selected items go to the extreme edge of the selection; one of "leading", "trailing", "top",
// "bottom".
func (s *Session) Align(edge string) {
	defer s.EndEvent()
	if s.opts.Readonly || len(s.selection) == 0 {
		return
	}
	first := s.selection[0]
	type entry struct {
		i *model.DiagramItem
		b render.Rect
	}
	var list []entry
	for _, i := range s.selection {
		if d, ok := s.scene.Drawables[i]; ok && i.Kind == model.ItemKindItem {
			list = append(list, entry{i, d.Bounds()})
		}
	}
	horizontal := edge == "leading" || edge == "trailing"
	at := func(e entry) float64 {
		switch edge {
		case "leading":
			return e.i.X
		case "trailing":
			return e.i.X + e.b.Width
		case "top":
			return e.i.Y
		}
		return e.i.Y - e.b.Height
	}
	beyond := func(v, t float64) bool { return v > t }
	if edge == "leading" || edge == "bottom" {
		beyond = func(v, t float64) bool { return v < t }
	}
	target := roundf(first.Y)
	if horizontal {
		target = roundf(first.X)
	}
	for _, e := range list {
		if beyond(at(e), target) {
			target = roundf(at(e))
		}
	}
	var ops []model.ElementOperation
	for _, e := range list {
		var pos model.Point
		switch edge {
		case "leading":
			pos = model.Point{X: target, Y: e.i.Y}
		case "trailing":
			pos = model.Point{X: target - e.b.Width, Y: e.i.Y}
		case "top":
			pos = model.Point{X: e.i.X, Y: target}
		default:
			pos = model.Point{X: e.i.X, Y: target + e.b.Height}
		}
		ops = append(ops, s.store.CreateUpdatePosition(e.i, pos))
	}
	s.commit(ops)
}

// Order is Move Forward (last in the item list, drawn on top) or Move Backward (first), for a
// single selected item.
func (s *Session) Order(forward bool) {
	defer s.EndEvent()
	if s.opts.Readonly || len(s.selection) != 1 {
		return
	}
	var at *int
	if !forward {
		at = model.IntPtr(0)
	}
	s.commit(s.store.CreateUpdateOrder(s.selection[0], at))
}

// TestLayout is Swift performSpringLayout ("Test layout"); bounds is the view rectangle centred on
// the origin. The pivot goes back to the origin, as Swift does after the layout.
func (s *Session) TestLayout(bounds render.Rect) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	ctx := model.NewLayoutContext(s.element, s.scene, s.store, model.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height})
	ops, err := model.NewSpringLayout(nil, nil).Apply(ctx, true)
	s.fail(err)
	s.commit(ops)
	if len(ops) > 0 {
		s.pivot = render.Point{}
	}
}

// valueNode is detectSymbolType + setValue: numbers stay numbers, text with blanks is a string, the
// rest a symbol.
func valueNode(value string) *tenn.Node {
	runes := []rune(value)
	if strings.HasPrefix(value, "-") {
		runes = runes[1:]
	}
	dot, digits := false, 0
	for _, c := range runes {
		switch {
		case c == '.':
			if digits == 0 || dot {
				return tenn.NewIdent(value)
			}
			dot = true
		case c == ' ' || c == '\n' || c == '\t':
			return tenn.NewStrNode(value)
		case unicode.IsDigit(c):
			digits++
		default:
			return tenn.NewIdent(value)
		}
	}
	if dot {
		return tenn.NewNode(tenn.KindFloatLit, tenn.NewToken(tenn.TokFloatLit, value))
	}
	return tenn.NewNode(tenn.KindIntLit, tenn.NewToken(tenn.TokIntLit, value))
}
