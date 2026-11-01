package scene

import "strings"

// inheritFont are the text properties an item takes from its parent when it sets none of its own, as
// font properties inherit in CSS: a component sets the font once on its root (spec 5b).
var inheritFont = []string{"font-family", "font-size", "font-weight", "font-style", "letter-spacing"}

// applyStyles gives every node the properties of the styles it names with use-style: a style comes where
// its use-style is written, so properties above it are defaults and properties below it win; kid blocks
// reach the kids as their defaults.
func applyStyles(r *RNode) {
	if r.Src != nil {
		if scope := styleScope(r.Src); scope != nil {
			var props []*Prop
			for _, name := range styleRefs(r) { // the last style named wins per property
				st := scope.Styles[name]
				if st == nil { // a group declaration is no style
					continue
				}
				props = append(props, st.Props...)
				applyKidStyles(r, st)
			}
			r.Props = afterUseStyle(r.Props, props)
		}
	}
	for _, k := range r.Kids {
		inheritProps(k, r)
		applyStyles(k)
	}
}

// afterUseStyle puts the properties of the styles right after the last use-style of the node: what is
// written before is a default the style overrides, what is written after overrides the style.
func afterUseStyle(props, add []*Prop) []*Prop {
	if len(add) == 0 {
		return props
	}
	at := 0
	for i, p := range props {
		if p.Name == "use-style" {
			at = i + 1
		}
	}
	out := make([]*Prop, 0, len(props)+len(add))
	out = append(out, props[:at]...)
	out = append(out, add...)
	return append(out, props[at:]...)
}

// applyKidStyles gives the kids a style names by name the properties of that block, as a CSS descendant
// selector does; they go first, so a kid's own and style properties win.
func applyKidStyles(r *RNode, st *Style) {
	for _, sk := range st.Kids {
		for _, k := range r.Kids {
			if k.Key == sk.Key {
				k.Props = append(append([]*Prop{}, sk.Style.Props...), k.Props...)
				applyKidStyles(k, sk.Style)
			}
		}
	}
}

// inheritProps gives an item the text properties of its parent when it sets none itself. They come
// first, so own and style properties win over them.
func inheritProps(k, parent *RNode) {
	if k.Kind != RItem {
		return
	}
	for _, name := range inheritFont {
		if k.prop(name) != nil {
			continue
		}
		if pr := parent.prop(name); pr != nil {
			k.Props = append([]*Prop{pr}, k.Props...)
		}
	}
}

// styleScope is the element whose styles a node may name: the nearest one above it in its own file.
func styleScope(n *Node) *Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if len(p.Styles) > 0 || len(p.StyleGroups) > 0 {
			return p
		}
	}
	return nil
}

// StyleScope is the element whose styles n may name, the one the inspector offers the groups of: the
// nearest element above n with a styles block; nil when there is none.
func StyleScope(n *Node) *Node { return styleScope(n) }

// StyleRefs are the styles props name with use-style, in order; "-name" drops the one named before.
func StyleRefs(props []*Prop) []string {
	var names []string
	for _, p := range props {
		if p.Name != "use-style" {
			continue
		}
		for i := range p.Args {
			name, _ := p.text(i)
			if rest, ok := strings.CutPrefix(name, "-"); ok {
				names = dropName(names, rest)
				continue
			}
			names = append(names, NormalizeName(name))
		}
	}
	return names
}

// styleRefs are the styles a node names, in order.
func styleRefs(r *RNode) []string { return StyleRefs(r.Props) }

func dropName(names []string, name string) []string {
	key := NormalizeName(name)
	out := names[:0]
	for _, n := range names {
		if n != key {
			out = append(out, n)
		}
	}
	return out
}
