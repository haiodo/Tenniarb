package editor

import (
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// QuickStyle is a quick-style menu of SceneDrawView (createQuickStyleMenu): the property, its
// caption and the values it offers.
type QuickStyle struct {
	Prop    string
	Label   string
	Options []string
	// Only is the kind the entry is for; empty for items and links.
	Only model.ItemKind
	// Sep: a separator goes before the entry, as in the Swift menu.
	Sep bool
}

// MarkerGroup is a submenu of the marker menu.
type MarkerGroup struct {
	Label string
	Items []string
}

// Markers are the submenus of the Swift marker menu.
var Markers = []MarkerGroup{
	{"😀 Emoji", []string{"😀", "😛", "😱", "😵", "😷", "🐶", "🐱", "🐭", "🐰", "🦊", "🌻", "🌧", "🌎", "🔥", "❄️", "💦", "☂️"}},
	{"🔢 Numbers", []string{"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}},
	{"🖥 Objects", []string{"⌚️", "🖥", "🖨", "⌛️", "⏰", "⚒", "🧲", "💣", "🔒", "✂️", "🧸", "🎁"}},
	{"🔠 Symbols", []string{"🆗", "🆖", "#️⃣", "🔤", "ℹ️", "🚻", "🔃", "➕", "➖", "➗", "✖️", "♾", "💲", "✔️", "♠️", "♣️", "♥️", "♦️"}},
}

// OptionLabels are the captions that differ from the option value (Swift prefixes colors and
// display and line styles with a glyph).
var OptionLabels = map[string]string{
	"red": "🔴red", "green": "🟢green", "blue": "🔵blue", "yellow": "🟡yellow", "orange": "🟠orange", "brown": "🟤brown", "black": "⚫️black", "purple": "🟣purple",
	"rect": "■ rect", "no-fill": "□ no-fill", "circle": "● circle", "stack": "❐ stack", "text": "≣ text",
	"solid": "– solid", "arrow": "→ arrow", "arrows": "↔︎ arrows", "arrow-source": "← arrow-source", "dashed": "⤍ dashed", "dotted": "⤑ dotted",
}

// OptionLabel is the caption of an option.
func OptionLabel(option string) string {
	if l, ok := OptionLabels[option]; ok {
		return l
	}
	return option
}

var colors = []string{"red", "green", "blue", "yellow", "orange", "brown", "black", "purple"}

// Display values of items and links.
var (
	ItemDisplay = []string{"rect", "no-fill", "circle", "stack", "text"}
	LinkDisplay = []string{"solid", "arrow", "arrows", "arrow-source"}
)

func markerOptions() []string {
	var out []string
	for _, g := range Markers {
		out = append(out, g.Items...)
	}
	return out
}

// QuickStyles is the quick-style list in Swift menu order; the options of "display" depend on the
// kind (QuickStylesFor fills them).
func quickStyles() []QuickStyle {
	return []QuickStyle{
		{Prop: "color", Label: "Color", Options: colors},
		{Prop: "border-color", Label: "Border color", Options: colors, Only: model.ItemKindItem},
		{Prop: "text-color", Label: "Text Color", Options: colors},
		{Prop: "font-size", Label: "Font", Sep: true, Options: []string{"8", "10", "12", "14", "16", "18", "20", "22", "26", "32", "36"}},
		{Prop: "marker", Label: "Markers", Only: model.ItemKindItem, Options: markerOptions()},
		{Prop: "display", Label: "Display", Sep: true},
		{Prop: "layout", Label: "Layout", Options: []string{"left", "right", "center", "top", "top right", "bottom", "bottom right"}},
		{Prop: "line-style", Label: "Line style", Sep: true, Options: []string{"solid", "dashed", "dotted"}},
		{Prop: "line-width", Label: "Line width", Options: []string{"0.3", "0.5", "1", "1.5", "2", "5"}},
		{Prop: "width", Label: "Width", Options: []string{"10", "50", "100", "300", "500"}, Only: model.ItemKindItem, Sep: true},
		{Prop: "height", Label: "Height", Options: []string{"10", "50", "100", "300", "500"}, Only: model.ItemKindItem},
		{Prop: "layer", Label: "Layer", Sep: true, Options: []string{"background", "hover"}},
		{Prop: "corner-radius", Label: "Corner Radius", Options: []string{"0", "5", "15"}, Only: model.ItemKindItem},
		{Prop: "shadow", Label: "Shadow", Options: []string{"5 -5", "5 5"}},
	}
}

// QuickStylesFor are the quick styles of an item or a link kind.
func QuickStylesFor(kind model.ItemKind) []QuickStyle {
	var out []QuickStyle
	for _, q := range quickStyles() {
		if q.Only != "" && q.Only != kind {
			continue
		}
		if q.Prop == "display" {
			q.Options = LinkDisplay
			if kind == model.ItemKindItem {
				q.Options = ItemDisplay
			}
		}
		out = append(out, q)
	}
	return out
}

// OptionNodes: marker is a string, everything else idents split on spaces (Swift markerMenuAction,
// layoutMenuAction, commandAction).
func OptionNodes(prop, option string) []*tenn.Node {
	if prop == "marker" {
		return []*tenn.Node{tenn.NewStrNode(option)}
	}
	var out []*tenn.Node
	for _, v := range strings.Split(option, " ") {
		out = append(out, tenn.NewIdent(v))
	}
	return out
}

// StyleNames are the named styles of the element (Swift StyleManager.update); the default `item`
// and `line` are not offered.
func (s *Session) StyleNames() []string {
	styles := s.element.Properties.Get("styles")
	if styles == nil {
		return nil
	}
	var out []string
	for _, c := range styles.GetBlock(1) {
		if !c.IsNamedElement() || c.GetChild(1) == nil {
			continue
		}
		if name, _ := c.GetIdent(0); name != "item" && name != "line" {
			out = append(out, name)
		}
	}
	return out
}

// ApplyStyle sets `use-style` of every selected item, one undo step (Swift StyleManager.doApply).
func (s *Session) ApplyStyle(name string) {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	var ops []model.ElementOperation
	for _, item := range s.selection {
		props := model.ItemToTennAsProps(item, tenn.KindBlockExpr, false)
		if cur := props.GetNamedElement("use-style"); cur != nil {
			if v, _ := cur.GetIdent(1); v == name {
				continue
			}
		}
		setCommand(props, "use-style", tenn.NewIdent(name))
		ops = append(ops, s.store.CreateProperties(s.element, item, props))
	}
	s.commit(ops)
}

// Operate is Swift OperationController.commit: text is Tenn commands applied to every selected
// item, one undo step. `name args` replaces or adds a property, `-name` removes it. False:
// readonly, parse errors or nothing selected.
func (s *Session) Operate(text string) bool {
	defer s.EndEvent()
	node, parser := tenn.Parse(text)
	if s.opts.Readonly || parser.Errors.HasErrors() || len(s.selection) == 0 {
		return false
	}
	var ops []model.ElementOperation
	for _, item := range s.selection {
		props := model.ItemToTennAsProps(item, tenn.KindBlockExpr, false)
		changed := false
		model.TraverseBlock(node, func(cmd string, n *tenn.Node) {
			if strings.HasPrefix(cmd, "-") {
				if props.RemoveNamed(cmd[1:]) {
					changed = true
				}
				return
			}
			if cur := props.GetNamedElement(cmd); cur != nil && n.Children != nil {
				cur.Children = nil
				cur.Add(n.Children...)
			} else {
				props.Add(n)
			}
			changed = true
		})
		if changed {
			ops = append(ops, s.store.CreateProperties(s.element, item, props))
		}
	}
	s.commit(ops)
	return true
}

// stylesBlock is the `styles { }` block of the element's properties props, made when absent.
func stylesBlock(props *model.ModelProperties) *tenn.Node {
	styles := props.Get("styles")
	if styles == nil {
		styles = tenn.NewCommand("styles", tenn.NewBlockExpr())
		props.Append(styles)
	}
	if styles.GetChild(1) == nil {
		styles.Add(tenn.NewBlockExpr())
	}
	return styles.GetChild(1)
}

// DefineStyle adds `new_style_N { color white }` to the element's `styles` (Swift
// StyleManager.addStyleConfig) and returns its name; "" when readonly.
func (s *Session) DefineStyle() string {
	defer s.EndEvent()
	if s.opts.Readonly {
		return ""
	}
	props := s.element.Properties.Clone()
	block := stylesBlock(props)
	name := "new_style_" + strconv.Itoa(block.Count()+1)
	block.Add(tenn.NewCommand(name, tenn.NewBlockExpr(tenn.NewCommand("color", tenn.NewStrNode("white")))))
	s.fail(s.store.SetProperties(s.element, props.AsNode(), s.um, s.refresh))
	return name
}

// SetQuickStyle sets one property of the single selected item to a quick-style option (Swift
// changeItemProps).
func (s *Session) SetQuickStyle(prop, option string) {
	defer s.EndEvent()
	if s.opts.Readonly || len(s.selection) != 1 {
		return
	}
	item := s.selection[0]
	props := model.ItemToTennAsProps(item, tenn.KindBlockExpr, false)
	setCommand(props, prop, OptionNodes(prop, option)...)
	s.fail(s.store.SetItemProperties(s.element, item, props, s.um, s.refresh))
}

// EnableShadows puts `item { shadow -5 -5 5 }` into the element's styles (Swift applyShadow without
// a selection).
func (s *Session) EnableShadows() {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	props := s.element.Properties.Clone()
	block := stylesBlock(props)
	item := block.GetNamedElement("item")
	if item == nil {
		item = tenn.NewCommand("item", tenn.NewBlockExpr())
		block.Add(item)
	}
	if item.GetChild(1) == nil {
		item.Add(tenn.NewBlockExpr())
	}
	itemBlock := item.GetChild(1)
	shadow := tenn.NewCommand("shadow", tenn.NewIntNode(-5), tenn.NewIntNode(-5), tenn.NewIntNode(5))
	if old := itemBlock.GetNamedElement("shadow"); old == nil {
		itemBlock.Add(shadow)
	} else {
		old.Children = shadow.Children
	}
	s.fail(s.store.SetProperties(s.element, props.AsNode(), s.um, s.refresh))
}
