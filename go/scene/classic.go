package scene

import "golang.org/x/text/unicode/norm"

// ClassicRef names a node of a classic diagram as go/model sees it: the element, an item by name and
// index, or a link by its two ends. Nth numbers the nodes of that name in file order from 0, as
// model.PrepareItemRefs does.
type ClassicRef struct {
	Kind     RKind  // RItem: the element or an item; RLink: a link
	Name     string // RItem: the name
	From, To string // RLink: the names of the ends
	Nth      int
	Root     bool // RItem: the element itself, not an item
}

// canEval reports whether an expression can become a value: an evaluator of the scene, or the values
// of the element model in a classic diagram.
func (p *prep) canEval() bool { return p.eval != nil || p.classic && p.opts.Classic != nil }

// classicRefs numbers the nodes of a classic diagram the way go/model names them, so Options.Classic
// can find the item or link each drawn node stands for.
func (p *prep) classicRefs(root *RNode) {
	p.classicAt = map[*RNode]ClassicRef{}
	items := map[string]int{}
	links := map[[2]string]int{}
	var walk func(rn *RNode, isRoot bool)
	walk = func(rn *RNode, isRoot bool) {
		switch rn.Kind {
		case RItem:
			key := norm.NFC.String(rn.Name)
			at := ClassicRef{Kind: RItem, Name: rn.Name, Root: isRoot}
			if !isRoot { // the element is not one of the items the model numbers
				at.Nth = items[key]
				items[key]++
			}
			p.classicAt[rn] = at
		case RLink:
			key := [2]string{norm.NFC.String(rn.From), norm.NFC.String(rn.To)}
			p.classicAt[rn] = ClassicRef{Kind: RLink, From: rn.From, To: rn.To, Nth: links[key]}
			links[key]++
		default:
			return
		}
		for _, k := range rn.Kids {
			walk(k, false)
		}
	}
	walk(root, true)
}

// modelVals is the property of a classic diagram as Options.Classic gives it; a node or a property the
// model does not hold gives nothing, and the arguments fall back to the usual evaluation.
func (p *prep) modelVals(rn *RNode, pr *Prop) ([]string, bool) {
	if p.opts.Classic == nil {
		return nil, false
	}
	if usesCtx(pr) {
		return nil, false
	}
	at, ok := p.classicAt[rn]
	if !ok {
		return nil, false
	}
	vals, ok := p.opts.Classic(at, pr.Name)
	if !ok || len(vals) != len(pr.Args) {
		return nil, false
	}
	return vals, true
}
