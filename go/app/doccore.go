package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// untitledName is the file name an untitled document has for the scene loader.
const untitledName = "Untitled.tenn"

// docCore is the document of a window without the window, so the API for the MCP service and the
// tests can use it without one.
type docCore struct {
	state   *State
	watch   Watcher
	session *editor.Session
	tracker *Tracker

	// text is what Save writes. A text edit splices it, so the rest of the file stays as written. A
	// classic edit prints the model (Swift printer) while printed, else only the elements it changed.
	text    string
	textSet bool // a text edit has set text; the session's change callback keeps it
	printed bool // a diagram document as read: the Swift printer owns its text
	version int
	kind    model.DocKind
	shape   shape // the element tree the text holds
	restore restore

	root     string      // workspace root: mounted files may not leave it
	stree    *scene.Tree // nil for a diagram document
	sceneErr string      // the errors go/scene finds in this file, for the banner
	libs     map[string]Stamp
	selNodes []string // selected scene nodes of the current element, by node path

	// Mounted files (mountdoc.go): the text held for each by relative path, the mounting elements, the
	// ones edited since the tree was read, and the error of the last write to a window that has the file.
	libDocs map[string]*libDoc
	mounts  map[*model.Element]*mountDoc
	keep    []*model.Element
	libErr  error
	libBusy bool // a window that has a mounted file is being edited by this document

	// peer gives the document another window holds for the mounted file rel; nil when none.
	peer func(self *docCore, rel string) *docCore

	// others gives the text of the other open documents under root, by slash path relative to it.
	others func(root string) map[string]string
}

// WorkspaceRoot is the root mounted files resolve in: the nearest folder up from the file that holds .git or
// a tenniarb.tenn, else the folder of the file.
func WorkspaceRoot(path string) string {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "."
		}
		return home
	}
	dir := filepath.Dir(path)
	for d := dir; ; {
		for _, mark := range []string{".git", "tenniarb.tenn"} {
			if _, err := os.Stat(filepath.Join(d, mark)); err == nil {
				return d
			}
		}
		up := filepath.Dir(d)
		if up == d {
			return dir
		}
		d = up
	}
}

// rel is the path of the document relative to the workspace root, with slashes.
func (d *docCore) rel() string {
	if d.state.Doc.Path == "" {
		return untitledName
	}
	r, err := filepath.Rel(d.root, d.state.Doc.Path)
	if err != nil {
		return filepath.Base(d.state.Doc.Path)
	}
	return filepath.ToSlash(r)
}

// adopt takes the text of a document that was just read into a new session. A diagram document keeps
// the text of the session, as before: the file is written by the Swift printer from the first edit on.
func (d *docCore) adopt(raw string) {
	d.root = WorkspaceRoot(d.state.Doc.Path)
	d.kind = model.KindOfModel(d.session.Root())
	d.text = raw
	d.printed = d.kind == model.DocDiagram
	if d.printed {
		d.text = d.session.Text()
	}
	d.shape = shapeOf(&d.session.Root().Element)
	d.restore = restore{}
	d.textSet = false
	d.selNodes = nil
	d.tracker = NewTracker(d.text)
	d.version++
	d.session.Store().MountWritable, d.session.Store().OnMountEdit = d.mountWritable, d.mountEdited
	d.loadScene()
}

// sessionChanged follows a change of the model: a classic edit, a text edit, undo or redo.
func (d *docCore) sessionChanged() {
	switch {
	case d.textSet:
		d.textSet = false
	case d.printed:
		d.text = d.session.Text()
	default:
		text, ok := d.spliceClassic()
		if !ok {
			// The text no longer pairs with the model: print it whole rather than lose a change.
			text = d.session.Text()
		}
		d.text = text
	}
	if d.tracker != nil {
		d.tracker.Changed(d.text)
	}
	d.version++
	d.kind = model.KindOfModel(d.session.Root())
	if d.kind != model.DocDiagram {
		d.printed = false
	}
	d.shape = shapeOf(&d.session.Root().Element)
	d.loadScene()
}

// loadScene reads the scene tree anew, with the unsaved text of the other open documents.
func (d *docCore) loadScene() {
	d.stree, d.sceneErr, d.libs = nil, "", nil
	keep := d.keep
	d.keep = nil
	if d.kind == model.DocDiagram || d.session == nil {
		d.mounts = nil
		return
	}
	rel := d.rel()
	overlay := d.overlay()
	overlay[rel] = d.text
	tree, err := scene.LoadOverlay(d.root, rel, overlay)
	if tree == nil {
		d.sceneErr = errString(err)
		d.mounts = nil
		return
	}
	d.stree = tree
	d.adoptMounts(model.MountFiles(d.session.Root(), tree, rel, keep...))
	d.sceneErr = sceneErrors(tree.Diagnostics())
	d.libs = map[string]Stamp{}
	for _, f := range tree.Files()[1:] {
		abs := filepath.Join(tree.Root(), filepath.FromSlash(f))
		s, _ := StampOf(abs)
		d.libs[abs] = s
	}
}

// sceneErrorCount is how many scene errors (not warnings) the tree would have with text as the text
// of file; -1 when the tree does not load at all.
func (d *docCore) sceneErrorCount(file, text string) int {
	overlay := d.overlay()
	rel := d.rel()
	overlay[rel] = d.text
	overlay[file] = text
	tree, _ := scene.LoadOverlay(d.root, rel, overlay)
	if tree == nil {
		return -1
	}
	n := 0
	for _, e := range tree.Diagnostics() {
		if !e.Warn {
			n++
		}
	}
	return n
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// sceneErrors is the banner text of the scene errors, without warnings, as errorText is for parse errors.
func sceneErrors(es scene.Errors) string {
	var lines []string
	for _, e := range es {
		if e.Warn {
			continue
		}
		if len(lines) == 5 {
			lines = append(lines, "...")
			break
		}
		lines = append(lines, e.Error())
	}
	return strings.Join(lines, "\n")
}

// bannerText is what the banner shows: parse errors first, then the scene errors.
func (d *docCore) bannerText() string {
	if d.state.Error != "" {
		return d.state.Error
	}
	return d.sceneErr
}

// libsChanged reports whether an imported file changed on disk since the tree was read.
func (d *docCore) libsChanged() bool {
	for abs, s := range d.libs {
		if now, err := StampOf(abs); err == nil && now != s {
			return true
		}
	}
	return false
}

// imports reports whether the tree holds the file abs (another open document).
func (d *docCore) imports(abs string) bool {
	_, ok := d.libs[abs]
	return ok
}

// isScene reports whether go/scene draws e.
func (d *docCore) isScene(e *model.Element) bool {
	return d.stree != nil && e != nil && model.SceneElement(e)
}

// file is the scene root of the document's own file.
func (d *docCore) file() *scene.Node {
	if d.stree == nil {
		return nil
	}
	return d.stree.File(d.rel())
}

// elementNode is the scene node of the element e: the file root for the root element.
func (d *docCore) elementNode(e *model.Element) *scene.Node {
	cur := d.file()
	if cur == nil || e == nil {
		return nil
	}
	for _, seg := range PathOf(e) {
		name, n := splitIndex(seg)
		var next *scene.Node
		for _, c := range cur.Elems {
			if c.Name == name {
				if n == 0 {
					next = c
					break
				}
				n--
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// splitIndex reads "name#n" as PathOf writes it.
func splitIndex(seg string) (string, int) {
	if i := strings.LastIndex(seg, "#"); i >= 0 {
		if k, err := strconv.Atoi(seg[i+1:]); err == nil && k > 0 {
			return seg[:i], k
		}
	}
	return seg, 0
}

// sceneKids are the nodes a node path goes through under n: the items and uses of an element, the
// items, uses and slots of an item, everything a use passes in its block. Links have no name to go by.
func sceneKids(n *scene.Node) []*scene.Node {
	var out []*scene.Node
	for _, c := range n.Content {
		switch c.Kind {
		case scene.KItem, scene.KUse:
			out = append(out, c)
		case scene.KSlot:
			if n.Kind == scene.KItem {
				out = append(out, c)
			}
		}
	}
	return out
}

// nodeSeg is the segment of n among the kids of its parent: its key, with "#n" for the n-th repeat.
func nodeSeg(kids []*scene.Node, n *scene.Node) string {
	k := 0
	for _, c := range kids {
		if c == n {
			break
		}
		if c.Key == n.Key {
			k++
		}
	}
	if k > 0 {
		return n.Key + "#" + strconv.Itoa(k)
	}
	return n.Key
}

// NodePath names n by the segments from the element el down, joined by dots; "" when n is not written
// inside el.
func NodePath(el, n *scene.Node) string {
	var segs []string
	for c := n; c != el; c = c.Parent {
		if c == nil || c.Parent == nil {
			return ""
		}
		segs = append(segs, nodeSeg(sceneKids(c.Parent), c))
	}
	slices.Reverse(segs)
	return strings.Join(segs, ".")
}

// NodeAt finds the node of a path that NodePath wrote under el; nil when it is gone.
func NodeAt(el *scene.Node, path string) *scene.Node {
	if el == nil || path == "" {
		return nil
	}
	cur := el
	for _, seg := range strings.Split(path, ".") {
		key, k := splitIndex(seg)
		var next *scene.Node
		for _, c := range sceneKids(cur) {
			if c.Key == key {
				if k == 0 {
					next = c
					break
				}
				k--
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// ErrStale is the error of an edit made against a version of the text that is not the current one.
var ErrStale = errors.New("the document changed since that version")

// applyText replaces text[start:end) by repl as one undo step of the session; the model follows the
// new text. A text that does not parse is refused.
func (d *docCore) applyText(start, end int, repl string) error {
	if d.session == nil {
		return errors.New("no document")
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
	before, _ := tenn.Parse(d.text)
	op := &textEditOp{OpBase: model.OpBase{Store: d.session.Store(), UndoCalled: true}, d: d, before: d.text, after: after}
	op.plan(&d.session.Root().Element, model.ParseTenn(ast), elementIndexAt(before, start, end))
	return d.session.Execute(op)
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
	before, after string
	swaps         []elemSwap
	notify        *model.Element
}

func (o *textEditOp) Name() string                { return "Edit text" }
func (o *textEditOp) GetNotifier() *model.Element { return o.notify }

func (o *textEditOp) Apply() error {
	for _, s := range o.swaps {
		put(s.e, s.new)
	}
	o.d.text, o.d.textSet = o.after, true
	return nil
}

func (o *textEditOp) Undo() error {
	for i := len(o.swaps) - 1; i >= 0; i-- {
		put(o.swaps[i].e, o.swaps[i].old)
	}
	o.d.text, o.d.textSet = o.before, true
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
