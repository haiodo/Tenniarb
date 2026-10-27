package app

import (
	"slices"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// A classic edit outside a Swift-printed document reprints only the elements whose own content changed;
// the others keep their bytes, so scene elements in the same file stay as written.

// shape is the element tree of the model when the text was last in step with it.
type shape struct {
	e    *model.Element
	kids []shape
}

// mountShape is the shape of the content of a mounted element, which is the file's.
func mountShape(m *model.Element) shape {
	s := shape{e: m}
	for _, c := range m.Elements {
		s.kids = append(s.kids, shapeOf(c))
	}
	return s
}

func shapeOf(e *model.Element) shape {
	s := shape{e: e}
	if e.FromFile != "" { // the text holds only the --from-file line; the children are the file's
		return s
	}
	for _, c := range e.Elements {
		s.kids = append(s.kids, shapeOf(c))
	}
	return s
}

// srcElem is an element command of the text (nil node: the file) and the element commands of its
// block, picked as go/model.ParseTenn picks them.
type srcElem struct {
	e     *model.Element
	node  *tenn.Node
	block *tenn.Node // the block go/model reads the content from; nil when it has none
	kids  []*srcElem
}

func srcElems(cmds []*tenn.Node, top bool) []*srcElem {
	var out []*srcElem
	for _, c := range cmds {
		if !elementCmd(c, top) {
			continue
		}
		s := &srcElem{node: c}
		if len(c.Children) > 2 && c.Children[2].Kind == tenn.KindBlockExpr {
			s.block = c.Children[2]
			s.kids = srcElems(s.block.Children, false)
		}
		out = append(out, s)
	}
	return out
}

func elementCmd(c *tenn.Node, top bool) bool {
	name := cmd(c)
	return c.Count() >= 2 && (name == "element" || top && name == "model")
}

func match(sh shape, src *srcElem, m map[*model.Element]*srcElem) bool {
	if len(sh.kids) != len(src.kids) {
		return false
	}
	src.e = sh.e
	m[sh.e] = src
	for i := range sh.kids {
		if !match(sh.kids[i], src.kids[i], m) {
			return false
		}
	}
	return true
}

// restore keeps the text that classic edits replaced, so undo brings back the same bytes.
type restore struct {
	names map[*model.Element]map[string]string // element name -> the token as written
	elems map[*model.Element][]keptElem
}

type keptElem struct {
	name, own string
	kids      []*model.Element
	kidText   []string
	text      string
}

type splicer struct {
	text string
	old  map[*model.Element]*srcElem
	r    *restore
}

type textEdit struct {
	start, end int
	text       string
}

func pad(ind int) string { return strings.Repeat("    ", ind) }

// spliceClassic is the text after a classic edit; ok is false when the text and the model before the
// edit do not pair up, or the result does not read back as the model's element tree.
func (d *docCore) spliceClassic() (string, bool) {
	return spliceText(d.text, d.shape, &d.session.Root().Element, &d.restore)
}

// spliceText is text brought up to root, an element tree that was in step with text as sh says: the
// root of the document, or an element that shows a mounted file as its own text.
func spliceText(text string, sh shape, root *model.Element, r *restore) (string, bool) {
	ast, p := tenn.Parse(text)
	if p.Errors.HasErrors() {
		return "", false
	}
	src := &srcElem{kids: srcElems(ast.Children, true)}
	sp := &splicer{text: text, old: map[*model.Element]*srcElem{}, r: r}
	if !match(sh, src, sp.old) {
		return "", false
	}
	out := sp.root(root, src, ast.Children)
	ast, p = tenn.Parse(out)
	if p.Errors.HasErrors() || !sameTree(root.Elements, srcElems(ast.Children, true)) {
		return "", false
	}
	return out, true
}

func sameTree(es []*model.Element, src []*srcElem) bool {
	if len(es) != len(src) {
		return false
	}
	for i, e := range es {
		if name, _ := src[i].node.GetIdent(1); name != e.Name || e.FromFile == "" && !sameTree(e.Elements, src[i].kids) {
			return false
		}
	}
	return true
}

func (sp *splicer) root(e *model.Element, src *srcElem, cmds []*tenn.Node) string {
	var old []*tenn.Node
	for _, c := range cmds {
		if !elementCmd(c, true) {
			old = append(old, c)
		}
	}
	if printNodes(e.Properties.Nodes()) == printNodes(old) {
		return sp.splice(e, src, 0, len(sp.text), 0)
	}
	// Ceiling: a classic edit of the file's own commands prints them all again, scene content at the
	// root included; no classic operation of the app changes them today.
	var parts []string
	for _, p := range e.Properties.Nodes() {
		parts = append(parts, tenn.ToStr(p, 0, false))
	}
	for _, k := range e.Elements {
		parts = append(parts, sp.emit(k, 0))
	}
	out := strings.Join(parts, "\n")
	if strings.HasSuffix(sp.text, "\n") {
		out += "\n"
	}
	return out
}

func printNodes(ns []*tenn.Node) string {
	var sb strings.Builder
	for _, n := range ns {
		sb.WriteString(tenn.ToStr(n, 0, false))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ownBlock is the own content of e (no child elements) as the Swift printer writes its block.
func ownBlock(e *model.Element) string {
	c := model.ToTenn(e, false, true).Children[0]
	return tenn.ToStr(c.Children[len(c.Children)-1], 0, false)
}

// oldElement is the element of command c without its child elements.
func oldElement(c *tenn.Node) *model.Element {
	sh := *c
	sh.Children = slices.Clone(c.Children)
	if len(sh.Children) > 2 && sh.Children[2].Kind == tenn.KindBlockExpr {
		b := *sh.Children[2]
		b.Children = slices.DeleteFunc(slices.Clone(b.Children), func(x *tenn.Node) bool { return elementCmd(x, false) })
		sh.Children[2] = &b
	}
	return model.ParseTenn(&sh).Elements[0]
}

// emit is the text of the element command of e at print indent ind, from its first token on.
func (sp *splicer) emit(e *model.Element, ind int) string {
	src := sp.old[e]
	if src == nil {
		return sp.print(e, ind)
	}
	if ownBlock(oldElement(src.node)) == ownBlock(e) && (src.block != nil || len(e.Elements) == 0) {
		return sp.splice(e, src, src.node.Token.Pos, src.node.End, ind)
	}
	out := sp.print(e, ind)
	sp.remember(src)
	return out
}

// remember keeps the text of src and of the elements in it for print.
func (sp *splicer) remember(src *srcElem) {
	old := oldElement(src.node)
	k := keptElem{name: old.Name, own: ownBlock(old), text: sp.text[src.node.Token.Pos:src.node.End]}
	for _, c := range src.kids {
		k.kids = append(k.kids, c.e)
		k.kidText = append(k.kidText, sp.text[c.node.Token.Pos:c.node.End])
		sp.remember(c)
	}
	if sp.r.elems == nil {
		sp.r.elems = map[*model.Element][]keptElem{}
	}
	if !slices.ContainsFunc(sp.r.elems[src.e], func(o keptElem) bool { return o.text == k.text }) {
		sp.r.elems[src.e] = append(sp.r.elems[src.e], k)
	}
}

// print writes e with the Swift printer and its child elements by emit; text a classic edit replaced
// before comes back when e is again as it was then.
func (sp *splicer) print(e *model.Element, ind int) string {
	var kids []string
	if e.FromFile == "" { // the children of a mounted element are its file's
		kids = make([]string, len(e.Elements))
		for i, k := range e.Elements {
			kids[i] = sp.emit(k, ind+1)
		}
	}
	if kept := sp.r.elems[e]; len(kept) > 0 {
		own := ownBlock(e)
		for _, k := range kept {
			if k.name == e.Name && k.own == own && slices.Equal(k.kids, e.Elements) && slices.Equal(k.kidText, kids) {
				return k.text
			}
		}
	}
	n := model.ToTenn(e, false, true).Children[0]
	out := strings.TrimPrefix(tenn.ToStr(n, ind, false), pad(ind))
	if len(kids) == 0 {
		return out
	}
	closing := pad(ind) + "}"
	var sb strings.Builder
	sb.WriteString(strings.TrimSuffix(out, closing))
	for _, k := range kids {
		sb.WriteString(pad(ind + 1))
		sb.WriteString(k)
		sb.WriteByte('\n')
	}
	sb.WriteString(closing)
	return sb.String()
}

// splice is text[start:end), the command of e (the file for the root), with its name and its child
// elements brought up to the model.
func (sp *splicer) splice(e *model.Element, src *srcElem, start, end, ind int) string {
	var edits []textEdit
	kidInd := ind + 1
	if src.node == nil {
		kidInd = 0
	} else if name, _ := src.node.GetIdent(1); name != e.Name {
		edits = append(edits, sp.rename(e, src.node.Children[1], name))
	}
	// The children of a mounted element are its file's, not written here.
	if src.node == nil || e.FromFile == "" {
		edits = append(edits, sp.kidEdits(e, src, kidInd)...)
	}
	// an insert goes before a replacement that starts at the same place
	sort.SliceStable(edits, func(i, j int) bool {
		a, b := edits[i], edits[j]
		return a.start < b.start || a.start == b.start && a.end < b.end
	})
	var sb strings.Builder
	at := start
	for _, ed := range edits {
		sb.WriteString(sp.text[at:ed.start])
		sb.WriteString(ed.text)
		at = ed.end
	}
	sb.WriteString(sp.text[at:end])
	return sb.String()
}

func (sp *splicer) rename(e *model.Element, n *tenn.Node, oldName string) textEdit {
	a, b := n.Token.Pos, n.Token.EndPos()
	if n.Token.Type == tenn.TokStringLit {
		a--
	}
	if sp.r.names == nil {
		sp.r.names = map[*model.Element]map[string]string{}
	}
	names := sp.r.names[e]
	if names == nil {
		names = map[string]string{}
		sp.r.names[e] = names
	}
	names[oldName] = sp.text[a:b]
	text, ok := names[e.Name]
	if !ok {
		text = tenn.ToStr(tenn.NewStrNode(e.Name), 0, false)
	}
	return textEdit{a, b, text}
}

// kidEdits cut out the child elements that left e, bring the kept ones up to the model and insert the
// new ones next to the kept ones before them. When the kept ones changed order, all go to the end.
func (sp *splicer) kidEdits(e *model.Element, src *srcElem, ind int) []textEdit {
	at := map[*model.Element]int{}
	for i, k := range e.Elements {
		at[k] = i
	}
	var kept []*srcElem
	for _, k := range src.kids {
		if _, ok := at[k.e]; ok {
			kept = append(kept, k)
		}
	}
	if !slices.IsSortedFunc(kept, func(a, b *srcElem) int { return at[a.e] - at[b.e] }) {
		kept = nil
	}
	isKept := map[*model.Element]*srcElem{}
	for _, k := range kept {
		isKept[k.e] = k
	}
	var edits []textEdit
	for _, k := range src.kids {
		if isKept[k.e] != nil {
			if t := sp.emit(k.e, ind); t != sp.text[k.node.Token.Pos:k.node.End] {
				edits = append(edits, textEdit{k.node.Token.Pos, k.node.End, t})
			}
			continue
		}
		sp.remember(k)
		edits = append(edits, sp.cut(k.node))
	}
	// runs of new elements, each after the kept one before it (nil: before the first kept one)
	var prev *srcElem
	runs := map[*srcElem][]string{}
	var order []*srcElem
	for _, k := range e.Elements {
		if s := isKept[k]; s != nil {
			prev = s
			continue
		}
		if _, ok := runs[prev]; !ok {
			order = append(order, prev)
		}
		runs[prev] = append(runs[prev], sp.emit(k, ind))
	}
	for _, p := range order {
		var sb strings.Builder
		switch {
		case p != nil:
			for _, t := range runs[p] {
				sb.WriteString("\n" + sp.indentAt(p.node.Token.Pos, ind) + t)
			}
			edits = append(edits, textEdit{p.node.End, p.node.End, sb.String()})
		case len(kept) > 0:
			for _, t := range runs[p] {
				sb.WriteString(t + "\n" + sp.indentAt(kept[0].node.Token.Pos, ind))
			}
			pos := kept[0].node.Token.Pos
			edits = append(edits, textEdit{pos, pos, sb.String()})
		default:
			edits = append(edits, sp.atEnd(src, runs[p], ind))
		}
	}
	return edits
}

// indentAt is the indent of the line of pos when only it stands before pos, else the printer's.
func (sp *splicer) indentAt(pos, ind int) string {
	a := pos
	for a > 0 && (sp.text[a-1] == ' ' || sp.text[a-1] == '\t') {
		a--
	}
	if a == 0 || sp.text[a-1] == '\n' {
		return sp.text[a:pos]
	}
	return pad(ind)
}

// cut removes the command n with the line break and indent before it.
func (sp *splicer) cut(n *tenn.Node) textEdit {
	a, b := n.Token.Pos, n.End
	for a > 0 && (sp.text[a-1] == ' ' || sp.text[a-1] == '\t') {
		a--
	}
	switch {
	case a > 0 && sp.text[a-1] == '\n':
		a--
	case b < len(sp.text) && sp.text[b] == '\n':
		b++
	}
	return textEdit{a, b, ""}
}

// atEnd inserts texts as the last commands of the block of src (the file for the root).
func (sp *splicer) atEnd(src *srcElem, texts []string, ind int) textEdit {
	var sb strings.Builder
	if src.node == nil {
		q := len(strings.TrimRight(sp.text, " \t\r\n"))
		for i, t := range texts {
			if q > 0 || i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(t)
		}
		return textEdit{q, q, sb.String()}
	}
	brace := src.node.End - 1
	q := brace
	for q > 0 && (sp.text[q-1] == ' ' || sp.text[q-1] == '\t') {
		q--
	}
	for _, t := range texts {
		sb.WriteString("\n" + pad(ind) + t)
	}
	if q > 0 && sp.text[q-1] == '\n' {
		return textEdit{q - 1, q - 1, sb.String()}
	}
	sb.WriteString("\n" + pad(ind-1))
	return textEdit{brace, brace, sb.String()}
}
