package editor

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/tenn"
)

// The typed side of the properties panel: one command read across the selection (the element when
// nothing is selected) and written back to all of it as one undo step.

// Arg is one argument of a command.
type Arg struct {
	// Text is the literal without quotes; for an expression the source with its delimiters.
	Text string
	// Expr: the argument is `$(...)` or `${...}`.
	Expr bool
	// Value is the evaluated text of an expression.
	Value string
}

// Prop is a command as the inspector shows it.
type Prop struct {
	// Set: some target has the command.
	Set bool
	// Mixed: the targets differ (one of them may lack the command).
	Mixed bool
	// Args are the arguments of the first target that has the command.
	Args []Arg
}

// Arg is the i-th argument, empty when there is none.
func (p Prop) Arg(i int) Arg {
	if i < len(p.Args) {
		return p.Args[i]
	}
	return Arg{}
}

// propTarget is the property list of one thing the inspector edits.
type propTarget struct {
	item    *model.DiagramItem
	element *model.Element
	props   *tenn.Node
}

func (s *Session) propTargets() []propTarget {
	if len(s.selection) == 0 {
		return []propTarget{{element: s.element, props: model.ElementToTennAsProps(s.element, tenn.KindBlockExpr, false)}}
	}
	out := make([]propTarget, len(s.selection))
	for i, it := range s.selection {
		out[i] = propTarget{item: it, props: model.ItemToTennAsProps(it, tenn.KindBlockExpr, false)}
	}
	return out
}

func (t propTarget) evalTarget() model.EvalTarget {
	if t.item != nil {
		return t.item
	}
	return t.element
}

// argText is the display text of an argument node.
func argText(n *tenn.Node) Arg {
	switch n.Kind {
	case tenn.KindExpression, tenn.KindExpressionBlock:
		return Arg{Text: tenn.ToStr(n, 0, false), Expr: true}
	case tenn.KindMarkdownLit, tenn.KindImage:
		return Arg{Text: tenn.ToStr(n, 0, false)}
	case tenn.KindFloatLit:
	}
	if n.Token != nil {
		return Arg{Text: n.Token.Literal}
	}
	return Arg{Text: tenn.ToStr(n, 0, true)}
}

// cmdArgs are the arguments of the command name in props.
func cmdArgs(props *tenn.Node, name string) ([]*tenn.Node, bool) {
	for _, c := range props.Children {
		if n, ok := c.GetIdent(0); ok && c.Kind == tenn.KindCommand && n == name {
			return c.Children[1:], true
		}
	}
	return nil, false
}

func argsKey(args []*tenn.Node) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = tenn.ToStr(a, 0, false)
	}
	return strings.Join(parts, " ")
}

// readProp reads name from the first target that has it, mixed when the others differ.
func (s *Session) readProp(targets []propTarget, name string) Prop {
	var p Prop
	var key string
	for i, t := range targets {
		args, ok := cmdArgs(t.props, name)
		k := ""
		if ok {
			k = argsKey(args)
		}
		if i == 0 {
			p.Set, key = ok, k
			if ok {
				p.Args = s.argsOf(t, args)
			}
			continue
		}
		if ok != p.Set || k != key {
			p.Mixed = true
		}
		if ok && !p.Set {
			p.Set = true
			p.Args = s.argsOf(t, args)
		}
	}
	return p
}

// argsOf lists args with the values of the expressions, evaluated in the scope of the target.
func (s *Session) argsOf(t propTarget, args []*tenn.Node) []Arg {
	out := make([]Arg, len(args))
	var expr []*tenn.Node
	for i, a := range args {
		out[i] = argText(a)
		if out[i].Expr {
			expr = append(expr, a)
		}
	}
	if len(expr) == 0 || s.exec == nil {
		return out
	}
	var size *model.DrawableSize
	if t.item != nil {
		if d, ok := s.scene.Drawables[t.item]; ok {
			b := d.SelectorBounds()
			size = &model.DrawableSize{Width: b.Width, Height: b.Height}
		}
	}
	block := tenn.NewBlockExpr(tenn.NewCommand("v", expr...))
	ev := s.exec.GetEvaluated(t.evalTarget(), block, size)
	for i, a := range args {
		if v, ok := ev[a.Token]; ok && out[i].Expr {
			out[i].Value = render.ValueString(v)
		}
	}
	return out
}

// Prop reads the command name from the selected items, or from the element when nothing is selected.
// The item name is `name`, the label of a link `label`.
func (s *Session) Prop(name string) Prop { return s.readProp(s.propTargets(), name) }

// argNode makes the node of one argument from the text the inspector holds: an expression keeps its
// delimiters, a number stays a number, text with blanks is a string, the rest a symbol.
func argNode(text string, forceString bool) (*tenn.Node, bool) {
	if forceString {
		return tenn.NewStrNode(text), true
	}
	if strings.HasPrefix(text, "$(") || strings.HasPrefix(text, "${") {
		node, parser := tenn.Parse("x " + text)
		if parser.Errors.HasErrors() || node.Count() != 1 || node.Children[0].Count() != 2 {
			return nil, false
		}
		arg := node.Children[0].Children[1]
		if arg.Kind != tenn.KindExpression && arg.Kind != tenn.KindExpressionBlock {
			return nil, false
		}
		return arg, true
	}
	if text == "" || strings.ContainsAny(text, "{}\"") {
		return nil, false
	}
	return valueNode(text), true
}

// stringProps are the commands whose argument is always a string.
var stringProps = map[string]bool{"name": true, "label": true, "title": true, "marker": true}

// setArgs gives the command name the args, adding it when absent; no args removes it.
func setArgs(props *tenn.Node, name string, args []*tenn.Node) {
	if len(args) == 0 {
		props.RemoveNamed(name)
		return
	}
	if cur, ok := findCmd(props, name); ok {
		cur.Children = []*tenn.Node{tenn.NewIdent(name)}
		cur.Add(args...)
		return
	}
	props.Add(tenn.NewCommand(name, args...))
}

func findCmd(props *tenn.Node, name string) (*tenn.Node, bool) {
	for _, c := range props.Children {
		if n, ok := c.GetIdent(0); ok && c.Kind == tenn.KindCommand && n == name {
			return c, true
		}
	}
	return nil, false
}

// edit applies change to the property list of every target and stores the result as one undo step.
// change reports whether it altered the list.
func (s *Session) edit(change func(t propTarget) bool) bool {
	defer s.EndEvent()
	if s.opts.Readonly {
		return false
	}
	var ops []model.ElementOperation
	for _, t := range s.propTargets() {
		if !change(t) {
			continue
		}
		if t.item != nil {
			ops = append(ops, s.store.CreateProperties(s.element, t.item, t.props))
			continue
		}
		s.fail(s.store.SetProperties(t.element, t.props, s.um, s.refresh))
	}
	s.commit(ops)
	return true
}

// SetProp sets the command name on every target; no args removes it. An argument is a number, a
// symbol, a string when it has blanks, or `${...}`; false when one is not valid.
func (s *Session) SetProp(name string, args ...string) bool {
	nodes := make([]*tenn.Node, len(args))
	for i, a := range args {
		n, ok := argNode(a, stringProps[name])
		if !ok {
			return false
		}
		nodes[i] = n
	}
	return s.edit(func(t propTarget) bool {
		setArgs(t.props, name, nodes)
		return true
	})
}

// SetPropArg changes the i-th argument and keeps the others, so `pos x y` of items that differ is
// edited one number at a time. Missing arguments before i are 0; empty text drops i and the rest.
func (s *Session) SetPropArg(name string, i int, text string) bool {
	var node *tenn.Node
	if text != "" {
		n, ok := argNode(text, stringProps[name])
		if !ok {
			return false
		}
		node = n
	}
	return s.edit(func(t propTarget) bool {
		args, _ := cmdArgs(t.props, name)
		args = slices.Clone(args)
		if node == nil {
			if i < len(args) {
				args = args[:i]
			}
		} else {
			for len(args) <= i {
				args = append(args, tenn.NewIntNode(0))
			}
			args[i] = node.Clone()
		}
		setArgs(t.props, name, args)
		return true
	})
}

// stylesGroup is the block `styles { group { ... } }` of the element, nil when absent.
func (s *Session) stylesGroup(props *tenn.Node, group string) *tenn.Node {
	styles, ok := findCmd(props, "styles")
	if !ok || styles.GetChild(1) == nil {
		return nil
	}
	g := styles.GetChild(1).GetNamedElement(group)
	if g == nil {
		return nil
	}
	return g.GetChild(1)
}

// StyleProp reads the command name of the named style group of the element (`item`, `line` or a
// style name from StyleNames), the defaults and named styles the diagram is drawn with.
func (s *Session) StyleProp(group, name string) Prop {
	t := propTarget{element: s.element, props: model.ElementToTennAsProps(s.element, tenn.KindBlockExpr, false)}
	block := s.stylesGroup(t.props, group)
	if block == nil {
		return Prop{}
	}
	return s.readProp([]propTarget{{element: s.element, props: block}}, name)
}

// SetStyleProp is SetProp for the style group of the element; the group is made when absent.
func (s *Session) SetStyleProp(group, name string, args ...string) bool {
	defer s.EndEvent()
	if s.opts.Readonly {
		return false
	}
	nodes := make([]*tenn.Node, len(args))
	for i, a := range args {
		n, ok := argNode(a, stringProps[name])
		if !ok {
			return false
		}
		nodes[i] = n
	}
	props := s.element.Properties.Clone()
	g := stylesBlock(props).GetNamedElement(group)
	if g == nil {
		g = tenn.NewCommand(group, tenn.NewBlockExpr())
		stylesBlock(props).Add(g)
	}
	if g.GetChild(1) == nil {
		g.Add(tenn.NewBlockExpr())
	}
	setArgs(g.GetChild(1), name, nodes)
	s.fail(s.store.SetProperties(s.element, props.AsNode(), s.um, s.refresh))
	return true
}

// Effective is the value the first selected item is drawn with, set by the item or not. Colors are
// #rrggbb; empty is unknown.
func (s *Session) Effective(name string) string {
	if len(s.selection) == 0 {
		return ""
	}
	item := s.selection[0]
	d, ok := s.scene.Drawables[item]
	if !ok {
		return ""
	}
	var st *render.DrawableStyle
	switch b := d.(type) {
	case *render.RoundBox:
		st = &b.Style.DrawableStyle
	case *render.CircleBox:
		st = b.Style
	case *render.EmptyBox:
		st = b.Style
	case *render.DrawableLine:
		st = &b.Style.DrawableStyle
	default:
		return ""
	}
	switch name {
	case "color":
		return hexOf(st.Color)
	case "border-color":
		return hexOf(st.BorderColor)
	case "text-color":
		return hexOf(st.TextColor())
	case "font-size":
		return trimFloat(st.FontSize)
	case "line-width":
		return trimFloat(st.LineWidth)
	case "line-style":
		if st.LineStyle != nil {
			return *st.LineStyle
		}
		return "solid"
	case "display":
		if st.Display != nil {
			return *st.Display
		}
		if item.Kind == model.ItemKindLink {
			return "solid"
		}
		return "rect"
	case "layer":
		if st.Layer != nil {
			return *st.Layer
		}
		return "default"
	case "width":
		return trimFloat(d.SelectorBounds().Width)
	case "height":
		return trimFloat(d.SelectorBounds().Height)
	}
	return ""
}

func hexOf(c render.Color) string {
	to8 := func(f float64) int { return int(math.Round(math.Max(0, math.Min(1, f)) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", to8(c.R), to8(c.G), to8(c.B))
}

// trimFloat is a computed number as Swift prints a Double, cut to 2 decimals: measured sizes carry
// noise like 43.0068359375. A value the user wrote is shown as written, not through here.
func trimFloat(f float64) string { return tenn.SwiftDouble(math.Round(f*100) / 100) }

// Description is the description of the element on the canvas.
func (s *Session) Description() string {
	props := model.ElementToTennAsProps(s.element, tenn.KindBlockExpr, false)
	if args, ok := cmdArgs(props, "description"); ok && len(args) > 0 {
		return argText(args[0]).Text
	}
	return ""
}

// SetDescription stores the description of the element as one undo step; empty text removes it.
func (s *Session) SetDescription(text string) bool {
	defer s.EndEvent()
	if s.opts.Readonly {
		return false
	}
	props := model.ElementToTennAsProps(s.element, tenn.KindBlockExpr, false)
	if text == "" {
		props.RemoveNamed("description")
	} else {
		v := tenn.NewStrNode(text)
		if strings.Contains(text, "\n") {
			v = tenn.NewMarkdownNode(text) // a string literal cannot hold a line break
		}
		setArgs(props, "description", []*tenn.Node{v})
	}
	s.fail(s.store.SetProperties(s.element, props, s.um, s.refresh))
	return true
}

// BodyOf is the body text of an item as the inline editor shows it.
func (s *Session) BodyOf(item *model.DiagramItem) string { return s.editText(item, EditBody) }
