package scene

import (
	"slices"
	"strings"
)

// inheritFont are the text properties an item takes from its parent when it sets none of its own, as
// font properties inherit in CSS: a component sets the font once on its root (spec 5b).
var inheritFont = []string{"font-family", "font-size", "font-weight", "font-style", "letter-spacing"}

// applyStyles gives every node the properties of the styles it names with use-style: a style comes where
// its use-style is written, so properties above it are defaults and properties below it win; kid blocks
// reach the kids as their defaults.
func applyStyles(r *RNode) {
	if r.Src != nil {
		def := r.Src
		if r.Def != nil { // the root of an instance takes the styles of what it uses
			def = r.Def
		}
		if scope := styleOwner(def); scope != nil {
			names := styleRefs(r)
			var props []*Prop
			for _, name := range names { // the last style named wins per property
				if st := scope.Styles[name]; st != nil {
					props = append(props, st.Props...)
				}
			}
			r.Props = afterUseStyle(r.Props, props)
			// Kid blocks prepend, so the styles go in reverse: the one named last stays nearest the end.
			for i := len(names) - 1; i >= 0; i-- {
				if st := scope.Styles[names[i]]; st != nil {
					applyKidStyles(r, st)
				}
			}
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

// styleOwner is the node whose styles a node may name: the node itself when it holds a styles block (an
// item that is a component), else the nearest element above it.
func styleOwner(n *Node) *Node {
	if len(n.Styles) > 0 || len(n.StyleGroups) > 0 {
		return n
	}
	return styleScope(n)
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

// StyleScope is the element whose styles n may name: the inspector offers its groups.
func StyleScope(n *Node) *Node { return styleScope(n) }

// StyleRefs are the styles props name with use-style, in order; "-name" drops the one named before.
// An expression names its styles only once evaluated: before that its words are not style names.
func StyleRefs(props []*Prop) []string {
	var names []string
	for _, p := range props {
		if p.Name != "use-style" || (p.Val == nil && hasExpr(p)) {
			continue
		}
		for i := range p.Args {
			name, _ := p.text(i)
			// a parameter may hold several styles: "extras outlined rounded" names both
			for _, word := range strings.Fields(name) {
				if rest, ok := strings.CutPrefix(word, "-"); ok {
					names = dropName(names, rest)
					continue
				}
				names = append(names, NormalizeName(word))
			}
		}
	}
	return names
}

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

// classicDefaults gives the nodes of a classic diagram the default look its element declares with
// styles { item { … } } and styles { line { … } }: go/render reads those two names as the look of every
// box and every link of the element, not as styles a node has to name.
func classicDefaults(r *RNode) {
	if r.Src == nil {
		return
	}
	item, line := r.Src.Styles["item"], r.Src.Styles["line"]
	if item == nil && line == nil {
		return
	}
	var walk func(n *RNode)
	walk = func(n *RNode) {
		switch n.Kind {
		case RItem:
			n.Props = append(defaultProps("item", item, n), n.Props...)
		case RLink:
			n.Props = append(defaultProps("line", line, n), n.Props...)
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	for _, k := range r.Kids {
		walk(k)
	}
}

// defaultProps are the properties the default style name gives n. A node that names the same style takes
// it through use-style alone, and a shadow of its own replaces the default one: go/render parsed the
// default first and the node's properties over it.
func defaultProps(name string, st *Style, n *RNode) []*Prop {
	if st == nil || slices.Contains(styleRefs(n), NormalizeName(name)) {
		return nil
	}
	ownShadow := slices.ContainsFunc(n.Props, func(pr *Prop) bool { return pr.Name == "shadow" })
	out := make([]*Prop, 0, len(st.Props))
	for _, pr := range st.Props {
		if ownShadow && pr.Name == "shadow" {
			continue
		}
		out = append(out, pr)
	}
	return out
}
