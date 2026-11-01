package scene

import "strings"

// applyStyles gives every node the properties of the styles it names with use-style: the node's own
// properties win, then the last style, then the ones before it; kid blocks reach the kids.
func applyStyles(r *RNode) {
	if r.Src != nil {
		if scope := styleScope(r.Src); scope != nil {
			names := styleRefs(r)
			for i := len(names) - 1; i >= 0; i-- { // last style first, so its properties end up nearest
				if st := scope.Styles[names[i]]; st != nil { // a group declaration is no style
					applyStyle(r, st)
				}
			}
		}
	}
	for _, k := range r.Kids {
		applyStyles(k)
	}
}

func applyStyle(r *RNode, st *Style) {
	if len(st.Props) > 0 {
		r.Props = append(append([]*Prop{}, st.Props...), r.Props...)
	}
	for _, sk := range st.Kids {
		for _, k := range r.Kids {
			if k.Key == sk.Key {
				applyStyle(k, sk.Style)
			}
		}
	}
}

// styleScope is the element whose styles a node may name: the nearest one above it in its own file.
func styleScope(n *Node) *Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if len(p.Styles) > 0 {
			return p
		}
	}
	return nil
}

// styleRefs are the styles a node names, in order; "-name" drops the one named before.
func styleRefs(r *RNode) []string {
	var names []string
	for _, p := range r.Props {
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
