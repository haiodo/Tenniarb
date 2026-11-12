//go:build darwin || linux || windows

package app

import (
	"errors"
	"fmt"
	"github.com/haiodo/tenniarb/go/scene"
	"strings"

	"github.com/haiodo/tenn/go"
	"github.com/haiodo/tenniarb/go/model"
)

// applyText replaces text[start:end) by repl as one undo step of the session; the model follows the
// new text. A text that does not parse is refused.
func (d *docCore) applyText(start, end int, repl string) error {
	if d.session == nil {
		return errors.New("no document")
	}
	if d.builtin {
		return errLibraryReadOnly(d.state.Doc.Path)
	}
	if start < 0 || end < start || end > len(d.text) {
		return fmt.Errorf("range %d..%d is outside the text of %d bytes", start, end, len(d.text))
	}
	after := d.text[:start] + repl + d.text[end:]
	ast, p := tenn.Parse(after)
	if p.Errors.HasErrors() {
		e := p.Errors.Errors[0]
		return fmt.Errorf("%d:%d: %s", e.Line+1, e.Col, e.Message)
	}
	idxOld := d.elementIndexAt(start, end)
	op := &textEditOp{OpBase: model.OpBase{Store: d.session.Store(), UndoCalled: true}, d: d, after: after}
	if d.kind == model.DocScene {
		// No classic step holds the objects a swap would keep alive: remember the splice, not the trees.
		op.lean, op.fresh, op.start = true, true, start
		op.oldText, op.newText = strings.Clone(d.text[start:end]), strings.Clone(repl)
		op.idxOld, op.idxNew = idxOld, elementIndexAt(ast, start, start+len(repl))
	} else {
		op.before = d.text
	}
	op.plan(&d.session.Root().Element, model.ParseTenn(ast), idxOld)
	return d.session.Execute(op)
}

// elementIndexAt is the path of the innermost element that holds [start, end) of the document text. The
// loaded scene tree numbers the element commands of a block as the parser does, so it answers without a
// second parse; a document with no scene tree, a diagram, is parsed.
func (d *docCore) elementIndexAt(start, end int) []int {
	if n := d.file(); n != nil {
		if src, ok := d.stree.Source(d.rel()); ok && src == d.text {
			return elementIndexIn(n, start, end)
		}
	}
	ast, _ := tenn.Parse(d.text)
	return elementIndexAt(ast, start, end)
}

// elementIndexIn is elementIndexAt over the scene nodes of a loaded file.
func elementIndexIn(n *scene.Node, start, end int) []int {
	var path []int
	for {
		var next *scene.Node
		k := 0
		for _, e := range n.Elems {
			if start > e.Body.Start && end < e.Body.End {
				next = e
				break
			}
			k++
		}
		if next == nil {
			return path
		}
		path = append(path, k)
		n = next
	}
}

// diffRange is the length of the common prefix and of the common suffix after it.
func diffRange(x, y string) (prefix, suffix int) {
	for prefix < len(x) && prefix < len(y) && x[prefix] == y[prefix] {
		prefix++
	}
	for suffix < len(x)-prefix && suffix < len(y)-prefix && x[len(x)-1-suffix] == y[len(y)-1-suffix] {
		suffix++
	}
	return
}

// elementIndexAt is the path, by index among the element commands of each level, of the innermost element
// whose block holds [start, end); empty for the root.
func elementIndexAt(ast *tenn.Node, start, end int) []int {
	var path []int
	cmds := ast.Children
	for {
		found := false
		k := 0
		for _, c := range cmds {
			if cmd(c) != "element" {
				continue
			}
			if b := blockOf(c); b != nil && b.Token != nil && start > b.Token.Pos && end < c.End {
				path = append(path, k)
				cmds = b.Children
				found = true
				break
			}
			k++
		}
		if !found {
			return path
		}
	}
}

func cmd(n *tenn.Node) string {
	if n.Kind != tenn.KindCommand {
		return ""
	}
	s, _ := n.GetIdent(0)
	return s
}

func blockOf(c *tenn.Node) *tenn.Node {
	if n := len(c.Children); n > 1 && c.Children[n-1].Kind == tenn.KindBlockExpr {
		return c.Children[n-1]
	}
	return nil
}

// elemData is what a text edit swaps in an element; the children only with withElems.
type elemData struct {
	items     []*model.DiagramItem
	props     *model.ModelProperties
	desc      *string
	elems     []*model.Element
	withElems bool
}

type elemSwap struct {
	e        *model.Element
	old, new elemData
}

// textEditOp is a text edit as a store operation: the elements it touches swap their content with the
// ones parsed from the new text, so classic undo steps before it keep the objects they hold.
type textEditOp struct {
	model.OpBase
	d             *docCore
	before, after string // the whole texts; a lean op keeps neither
	swaps         []elemSwap
	notify        *model.Element

	// A lean op (scene document) keeps the splice instead of the swapped trees, and undo and redo parse
	// the text again: text[start:start+len(oldText)] became newText, idxOld and idxNew are the element
	// paths of the range in the old and in the new text.
	lean             bool
	fresh            bool // not applied yet: the swaps were planned by applyText
	start            int
	oldText, newText string
	idxOld, idxNew   []int
}

func (o *textEditOp) Name() string { return "Edit text" }

func (o *textEditOp) GetNotifier() *model.Element {
	if !o.lean {
		return o.notify
	}
	idx := o.idxNew
	if o.UndoCalled { // the model shows the old text
		idx = o.idxOld
	}
	a := &o.d.session.Root().Element
	for _, i := range idx {
		if i >= len(a.Elements) {
			break
		}
		a = a.Elements[i]
	}
	return a
}

func (o *textEditOp) Apply() error {
	if o.lean && !o.fresh {
		return o.shift(o.oldText, o.newText, o.idxOld)
	}
	for _, s := range o.swaps {
		put(s.e, s.new)
	}
	o.d.text, o.d.textSet = o.after, true
	if o.lean {
		o.fresh, o.swaps, o.after = false, nil, ""
	}
	return nil
}

func (o *textEditOp) Undo() error {
	if o.lean {
		return o.shift(o.newText, o.oldText, o.idxNew)
	}
	for i := len(o.swaps) - 1; i >= 0; i-- {
		put(o.swaps[i].e, o.swaps[i].old)
	}
	o.d.text, o.d.textSet = o.before, true
	return nil
}

// shift replaces from by to at the start of the splice in the text of the document and brings the
// elements of idx up to the new text.
func (o *textEditOp) shift(from, to string, idx []int) error {
	cur, end := o.d.text, o.start+len(from)
	if end > len(cur) || cur[o.start:end] != from {
		return errors.New("the text changed since this edit")
	}
	next := cur[:o.start] + to + cur[end:]
	ast, p := tenn.Parse(next)
	if p.Errors.HasErrors() {
		e := p.Errors.Errors[0]
		return fmt.Errorf("%d:%d: %s", e.Line+1, e.Col, e.Message)
	}
	o.swaps = nil
	o.plan(&o.d.session.Root().Element, model.ParseTenn(ast), idx)
	for _, s := range o.swaps {
		put(s.e, s.new)
	}
	o.swaps = nil
	o.d.text, o.d.textSet = next, true
	return nil
}

func put(e *model.Element, d elemData) {
	e.Items, e.Properties, e.Description = d.items, d.props, d.desc
	for _, it := range d.items {
		it.Parent = e
	}
	if !d.withElems {
		return
	}
	e.Elements = d.elems
	var adopt func(p *model.Element)
	adopt = func(p *model.Element) {
		for _, c := range p.Elements {
			c.Parent, c.Model = p, p.Model
			adopt(c)
		}
	}
	adopt(e)
}

func dataOf(e *model.Element) elemData {
	return elemData{items: e.Items, props: e.Properties, desc: e.Description}
}

// plan pairs the element at index path idx of the old model with the one of the new model and swaps
// their subtrees: element by element while the trees have the same shape, else the children at once.
func (o *textEditOp) plan(old *model.Element, next *model.ElementModel, idx []int) {
	a, b := old, &next.Element
	for _, i := range idx {
		if i >= len(a.Elements) || i >= len(b.Elements) {
			break
		}
		a, b = a.Elements[i], b.Elements[i]
	}
	o.notify = a
	var pair func(a, b *model.Element)
	pair = func(a, b *model.Element) {
		if a.FromFile != "" && sameShape(a, b) {
			return // its content is the file's, not the text's
		}
		s := elemSwap{e: a, old: dataOf(a), new: dataOf(b)}
		if !sameShape(a, b) {
			s.old.elems, s.new.elems = a.Elements, b.Elements
			s.old.withElems, s.new.withElems = true, true
			o.swaps = append(o.swaps, s)
			return
		}
		o.swaps = append(o.swaps, s)
		for i := range a.Elements {
			pair(a.Elements[i], b.Elements[i])
		}
	}
	pair(a, b)
}

func sameShape(a, b *model.Element) bool {
	if a.Name != b.Name || a.FromFile != b.FromFile {
		return false
	}
	if a.FromFile != "" {
		return true
	}
	if len(a.Elements) != len(b.Elements) {
		return false
	}
	for i := range a.Elements {
		if !sameShape(a.Elements[i], b.Elements[i]) {
			return false
		}
	}
	return true
}
