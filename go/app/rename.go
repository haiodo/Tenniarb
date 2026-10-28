package app

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// errRenameScene is the refusal of a rename whose result has scene errors the document did not have.
var errRenameScene = errors.New("Not applied: the change adds scene errors")

// fileEdit is a text edit of one file of the document.
type fileEdit struct {
	File string
	Edit TextEdit
}

// renamePlan is a rename of an element or of a scene node and what refers to it. An element is renamed
// through the model, so its own name is not among the text edits; the name of a node is.
type renamePlan struct {
	name  string
	el    *model.Element
	node  *scene.Node
	own   fileEdit
	refs  []scene.Ref // those the rename changes, edits[i] for refs[i]
	edits []fileEdit
	total int // how many places name the node now, those the rename leaves as they are too
}

// argText is an argument the way the printer writes it: bare when it can be, else quoted.
func argText(a string) string { return strings.TrimPrefix(CommandText("x", a), "x ") }

// refEdit is the edit that makes r name the node by name. False when the text already does: a path
// segment is compared by its normalized form.
func refEdit(r scene.Ref, name string) (TextEdit, bool) {
	text := name
	if r.Seg >= 0 {
		segs := strings.Split(r.Text, ".")
		key := scene.NormalizeName(name)
		if r.Seg >= len(segs) || scene.NormalizeName(segs[r.Seg]) == key {
			return TextEdit{}, false
		}
		segs[r.Seg] = key
		text = strings.Join(segs, ".")
	} else if r.Text == name {
		return TextEdit{}, false
	}
	return TextEdit{r.Arg.Start, r.Arg.End, argText(text)}, true
}

// keyTaken reports whether a sibling of n, which a path can name, has the key.
func keyTaken(n *scene.Node, key string) bool {
	p := n.Parent
	if p == nil {
		return false
	}
	return slices.ContainsFunc(slices.Concat(p.Nodes(), p.Content), func(c *scene.Node) bool {
		return c != n && c.Key == key && key != ""
	})
}

// planElementRename finds what the rename of e changes. Without a scene tree (a diagram document)
// nothing refers to an element.
func (d *docCore) planElementRename(e *model.Element, name string) (*renamePlan, error) {
	p := &renamePlan{name: strings.TrimSpace(name), el: e}
	if err := d.checkRename(); err != nil {
		return nil, err
	}
	if p.name == "" || e.Parent == nil {
		return nil, errors.New("The name cannot be empty.")
	}
	if n := d.elementNode(e); n != nil && d.stree != nil {
		return p, d.planNode(p, n)
	}
	for _, s := range e.Parent.Elements {
		if s != e && s.Name == p.name {
			return nil, fmt.Errorf("There is an element named %q already.", p.name)
		}
	}
	return p, nil
}

// planNodeRename finds what the rename of the item or use n changes.
func (d *docCore) planNodeRename(n *scene.Node, name string) (*renamePlan, error) {
	p := &renamePlan{name: strings.TrimSpace(name), node: n}
	if err := d.checkRename(); err != nil {
		return nil, err
	}
	if p.name == "" {
		return nil, errors.New("The name cannot be empty.")
	}
	if n.Name == "" || n.NameArg == (scene.Range{}) {
		return nil, errors.New("An item without a name cannot be renamed here.")
	}
	return p, d.planNode(p, n)
}

func (d *docCore) checkRename() error {
	if d.session == nil || d.session.Readonly() {
		return errors.New("The document is read-only.")
	}
	return nil
}

// planNode takes the references to n for the plan; the name is checked against the siblings first.
func (d *docCore) planNode(p *renamePlan, n *scene.Node) error {
	p.node = n
	if key := scene.NormalizeName(p.name); key != n.Key && (keyTaken(n, key) || slices.ContainsFunc(sceneKids(n.Parent), func(c *scene.Node) bool { return c != n && c.Key == key })) {
		return fmt.Errorf("There is something named %q here already.", p.name)
	}
	p.own = fileEdit{n.Pos.File, TextEdit{n.NameArg.Start, n.NameArg.End, argText(p.name)}}
	refs, blocked := d.stree.References(n)
	p.total = len(refs)
	for _, r := range refs {
		if e, ok := refEdit(r, p.name); ok {
			p.refs = append(p.refs, r)
			p.edits = append(p.edits, fileEdit{r.File, e})
		}
	}
	if blocked != "" {
		return errors.New("Not renamed: " + blocked + ".")
	}
	return nil
}

// splice merges the edits of each file into the one that covers them all, so a file is edited once.
func (d *docCore) splice(edits []fileEdit) (map[string]TextEdit, error) {
	by := map[string][]TextEdit{}
	for _, e := range edits {
		by[e.File] = append(by[e.File], e.Edit)
	}
	out := map[string]TextEdit{}
	for file, es := range by {
		text := d.fileText(file)
		slices.SortFunc(es, func(a, b TextEdit) int { return a.Start - b.Start })
		lo, hi := es[0].Start, es[0].End
		var b strings.Builder
		at := lo
		for _, e := range es {
			if e.Start < at || e.End < e.Start || e.End > len(text) {
				return nil, fmt.Errorf("The references in %s overlap.", file)
			}
			b.WriteString(text[at:e.Start])
			b.WriteString(e.New)
			at, hi = e.End, max(hi, e.End)
		}
		b.WriteString(text[at:hi])
		out[file] = TextEdit{lo, hi, b.String()}
	}
	return out, nil
}

// texts are the files after the splices.
func (d *docCore) spliced(sp map[string]TextEdit) map[string]string {
	out := map[string]string{}
	for file, e := range sp {
		text := d.fileText(file)
		out[file] = text[:e.Start] + e.New + text[e.End:]
	}
	return out
}

// checkResult loads the files as the edits make them and refuses a result with scene errors the
// document has not, or one where more paths name the node than before: its new name captured them.
func (d *docCore) checkResult(p *renamePlan, edits []fileEdit) error {
	sp, err := d.splice(edits)
	if err != nil {
		return err
	}
	var was int
	if d.treeShows(d.rel(), d.text) {
		was = errorCount(d.stree)
	} else {
		was = d.sceneErrorCount(d.rel(), d.text)
	}
	pend, now := d.loadFiles(d.spliced(sp))
	if now > was || now < 0 && was >= 0 {
		return errRenameScene
	}
	at := p.own.Edit.Start
	for _, e := range edits {
		if e.File == p.own.File && e.Edit.Start < p.own.Edit.Start {
			at += len(e.Edit.New) - (e.Edit.End - e.Edit.Start)
		}
	}
	if n := pend.tree.NodeNamedAt(p.own.File, at); n != nil {
		if refs, _ := pend.tree.References(n); len(refs) > p.total {
			return fmt.Errorf("Not renamed: the name %q would take over the references to something else.", p.name)
		}
	}
	return nil
}

// applyRename is one undo step: the references when update is set, then the name. A result that
// checkResult refuses changes nothing. Without update the user chose to leave the references, so the
// check is skipped.
func (d *docCore) applyRename(p *renamePlan, update bool) error {
	var edits []fileEdit
	if update {
		edits = p.edits
	}
	own := slices.Concat(edits, []fileEdit{p.own})
	if update && d.stree != nil && p.node != nil {
		if err := d.checkResult(p, own); err != nil {
			return err
		}
	}
	if p.el == nil {
		edits = own
	}
	sp, err := d.splice(edits)
	if err != nil {
		return err
	}
	files := make([]string, 0, len(sp))
	for f := range sp {
		files = append(files, f)
	}
	sort.Strings(files)
	// The tree is loaded again after each edit, and in the middle of the rename it may no longer reach
	// the files left: they are checked and their texts kept now.
	for _, f := range files {
		if f == d.rel() {
			continue
		}
		if d.stree == nil || d.stree.File(f) == nil {
			return fmt.Errorf("%s is not a file of this document", f)
		}
		d.libDoc(f)
	}
	applied := 0
	d.session.Batch(func() {
		for _, f := range files {
			if f == d.rel() {
				err = d.applyText(sp[f].Start, sp[f].End, sp[f].New)
			} else {
				err = d.applyLibText(f, sp[f].Start, sp[f].End, sp[f].New)
			}
			if err != nil {
				return
			}
			applied++
		}
		if p.el != nil {
			d.session.RenameElement(p.el, p.name)
		}
	})
	if err != nil && applied > 0 {
		d.session.Undo()
	}
	return err
}

// refLines describes up to limit references for a dialog: file:line and the line.
func (d *docCore) refLines(refs []scene.Ref, limit int) string {
	var b strings.Builder
	for i, r := range refs {
		if i == limit {
			fmt.Fprintf(&b, "and %d more\n", len(refs)-limit)
			break
		}
		text := d.fileText(r.File)
		line := strings.Count(text[:min(r.Arg.Start, len(text))], "\n")
		fmt.Fprintf(&b, "%s:%d  %s\n", r.File, line+1, trimLine(nthLine(text, line), 70))
	}
	return strings.TrimRight(b.String(), "\n")
}

func nthLine(text string, n int) string {
	lines := strings.Split(text, "\n")
	if n >= len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[n])
}

func trimLine(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

// movedPath is p when the node at old is renamed so that its path becomes now: a path of the node or
// below it follows.
func movedPath(p, old, now string) (string, bool) {
	if p == old {
		return now, true
	}
	if rest, ok := strings.CutPrefix(p, old); ok && (rest[0] == '.' || rest[0] == '/') {
		return now + rest, true
	}
	return p, false
}

// renamedNodePath is the path of the node at old once its key is key.
func renamedNodePath(old, key string) string {
	if i := strings.LastIndexByte(old, '.'); i >= 0 {
		return old[:i+1] + key
	}
	return key
}

// ClassicRef is a place of a diagram that mentions an item by name; a rename does not update it.
type ClassicRef struct {
	Item *model.DiagramItem
	Elem *model.Element // the element of Item
	Text string
}

// ClassicItemRefs finds the mentions of the item of el by name: an expression of another item of el
// and an `inherit` of an item of an element below. Link ends are not among them, a link holds the
// item itself. Ceiling: an expression is searched as text, so a string with the name is found too.
func ClassicItemRefs(el *model.Element, it *model.DiagramItem) []ClassicRef {
	return append(exprRefs(el, it), inheritRefs(el, it.Name, "../")...)
}

func exprRefs(el *model.Element, it *model.DiagramItem) []ClassicRef {
	var out []ClassicRef
	id := scene.NormalizeName(it.Name)
	for _, o := range el.Items {
		if o == it || o.IsLink() {
			continue
		}
		o.Properties.AsNode().Traverse(func(n *tenn.Node) bool {
			if n.Kind != tenn.KindExpression && n.Kind != tenn.KindExpressionBlock {
				return true
			}
			if text, _ := n.GetIdentText(); mentions(text, id) {
				out = append(out, ClassicRef{o, el, "${" + trimLine(strings.Join(strings.Fields(text), " "), 50) + "}"})
			}
			return true
		})
	}
	return out
}

// inheritRefs are the items below e that inherit the item name of the element up levels above them.
func inheritRefs(e *model.Element, name, up string) []ClassicRef {
	var out []ClassicRef
	for _, c := range e.Elements {
		for _, o := range c.Items {
			inh := o.Properties.Get("inherit")
			if inh == nil {
				continue
			}
			if s, _ := inh.GetIdent(1); s == up+name {
				out = append(out, ClassicRef{o, c, "inherit " + s})
			}
		}
		out = append(out, inheritRefs(c, name, up+"../")...)
	}
	return out
}

// mentions reports whether the script uses id as a name: not as a part of a longer name or a member.
func mentions(script, id string) bool {
	if id == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(script[i:], id)
		if j < 0 {
			return false
		}
		a, b := i+j, i+j+len(id)
		before, _ := utf8.DecodeLastRuneInString(script[:a])
		after, _ := utf8.DecodeRuneInString(script[b:])
		if a == 0 || !nameRune(before) && before != '.' {
			if b == len(script) || !nameRune(after) {
				return true
			}
		}
		i = a + 1
	}
}

func nameRune(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// classicRefLines describes up to limit mentions for a dialog.
func classicRefLines(refs []ClassicRef, limit int) string {
	var b strings.Builder
	for i, r := range refs {
		if i == limit {
			fmt.Fprintf(&b, "and %d more\n", len(refs)-limit)
			break
		}
		fmt.Fprintf(&b, "%s / %s  %s\n", r.Elem.Name, r.Item.Name, r.Text)
	}
	return strings.TrimRight(b.String(), "\n")
}
