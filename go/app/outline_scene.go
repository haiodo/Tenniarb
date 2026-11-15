//go:build darwin || linux || windows

package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenn/go"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// library is a file of components the document mounts with --from-file.
type library struct {
	lib  string // the name of the mounting element, the start of a use path
	file string // relative to the workspace root
}

// libraries are the files the document mounts with --from-file, each once.
func (v *Viewer) libraries() []*library {
	f := v.file()
	if f == nil {
		return nil
	}
	var out []*library
	seen := map[*scene.Node]bool{}
	var walk func(n *scene.Node)
	walk = func(n *scene.Node) {
		for _, c := range n.Elems {
			if c.Mount != nil && !seen[c.Mount] {
				seen[c.Mount] = true
				out = append(out, &library{lib: c.Key, file: c.Mount.Pos.File})
			}
			walk(c)
		}
	}
	walk(f)
	return out
}

// leaveMount puts the last element of the document that is not mounted on the canvas, when a mounted
// element itself is there: a use of a component belongs to the document, not to the file it comes from.
// A screen inside a mounted file is a target of its own: Board settings of Tracker is not the library.
func (v *Viewer) leaveMount() bool {
	if v.state.Current == nil || v.state.Current.FromFile == "" {
		return true
	}
	e := v.docElem
	if e == nil || Resolve(&v.session.Root().Element, PathOf(e)) != e {
		v.setBanner("Open an element of the document first: the component is inserted there.")
		return false
	}
	v.session.SetElement(e)
	if v.sceneMode() {
		v.selNodes = v.docSel
		v.dropStaleSelection()
		v.selectScene(v.selNodes)
	}
	return true
}

// insertFromMount is Insert Component on a row of a mounted element: the document comes back on the
// canvas and gets a use of the node.
func (v *Viewer) insertFromMount(m *model.Element, r *ItemRow) {
	n := NodeAt(v.sv.node, r.Path)
	p, ok := UsePath(scene.NormalizeName(m.Name), n)
	if !ok {
		v.setBanner("Only an element or an item of the library, not inside an instance or a slot, can be inserted.")
		return
	}
	if v.leaveMount() {
		v.insertComponent(p, n.Name)
	}
}

// components are what the imported libraries offer for the query, then what the document itself reuses:
// grouped by the branch they are defined in, the order of the branches as the files write them.
func (v *Viewer) components(q string, limit int) []Component {
	var out []Component
	for _, r := range v.libraries() {
		if n := v.stree.File(r.file); n != nil {
			out = append(out, Components(r.lib, n, q, limit-len(out))...)
		}
	}
	return byGroup(append(out, v.ownComponents(q, limit-len(out))...))
}

// byGroup puts the components of one group side by side, the groups in the order they first appear: the
// browser and the picker draw a heading where the group changes.
func byGroup(in []Component) []Component {
	var order []string
	members := map[string][]Component{}
	for _, c := range in {
		if _, ok := members[c.Group]; !ok {
			order = append(order, c.Group)
		}
		members[c.Group] = append(members[c.Group], c)
	}
	out := make([]Component, 0, len(in))
	for _, g := range order {
		out = append(out, members[g]...)
	}
	return out
}

// ownComponents are the nodes the document's own use statements target: a path from the root names them
// wherever the insertion goes. A node the file never uses is not offered, as nothing tells it apart from a
// part of a screen.
func (v *Viewer) ownComponents(q string, limit int) []Component {
	f := v.file()
	if f == nil || v.stree == nil || limit <= 0 {
		return nil
	}
	seen := map[*scene.Node]bool{}
	var nodes []*scene.Node
	var walk func(n *scene.Node)
	walk = func(n *scene.Node) {
		for _, k := range n.Nodes() {
			if k.Mount != nil {
				continue // a mounted file is a library: its own list covers it
			}
			if k.Kind != scene.KUse {
				walk(k)
				continue
			}
			target, err := v.stree.Resolve(k.Parent, k.Path)
			if err != nil || seen[target] || v.fileOf(target) != v.rel() {
				continue
			}
			if _, ok := UsePath("", target); !ok || target.Name == "" {
				continue
			}
			seen[target] = true
			nodes = append(nodes, target)
		}
	}
	walk(f)
	root := commonAncestor(nodes)
	q = strings.ToLower(q)
	var out []Component
	for _, n := range nodes {
		if !strings.Contains(strings.ToLower(n.Name), q) && !strings.Contains(strings.ToLower(n.Key), q) {
			continue
		}
		p, _ := UsePath("", n)
		out = append(out, Component{Path: p, Name: n.Name, Group: branchOf(root, n), Node: n})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// commonAncestor is the deepest node that holds every one of nodes; nil for none.
func commonAncestor(nodes []*scene.Node) *scene.Node {
	if len(nodes) == 0 {
		return nil
	}
	for a := nodes[0].Parent; a != nil; a = a.Parent {
		all := true
		for _, n := range nodes[1:] {
			held := false
			for p := n.Parent; p != nil; p = p.Parent {
				if p == a {
					held = true
					break
				}
			}
			if !held {
				all = false
				break
			}
		}
		if all {
			return a
		}
	}
	return nil
}

// branchOf is the name of the child of root that n sits in: the top branch under the shared root.
func branchOf(root, n *scene.Node) string {
	if root == nil {
		return ""
	}
	for p := n; p != nil; p = p.Parent {
		if p.Parent == root {
			if p == n {
				return "" // the component is the branch itself: a heading over one row says nothing
			}
			return p.Name
		}
	}
	return ""
}

// addLibrary is Element > Add Library...: a file of components becomes a top-level element of the
// document, and Insert Component offers what it holds.
func (v *Viewer) addLibrary() {
	if v.session == nil {
		return
	}
	dir := v.root
	if v.state.Doc.Path != "" {
		dir = filepath.Dir(v.state.Doc.Path)
	}
	paths := v.win.FileDialog(g.FileDialog{Title: "Add Library", Dir: dir, Filters: tennFilters})
	if len(paths) != 1 {
		return
	}
	if err := v.addLibraryFile(paths[0]); err != nil {
		v.setBanner(err.Error())
		return
	}
	v.setBanner(v.bannerText())
}

// addLibraryFile writes the element that mounts the library at the end of the document, as one undo
// step. The path is relative to the document; a file outside the workspace root is refused.
func (v *Viewer) addLibraryFile(path string) error {
	if v.session == nil {
		return errors.New("no document")
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(v.root)
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(target); err == nil {
		target = r
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	out, err := filepath.Rel(root, target)
	if err != nil || out == ".." || strings.HasPrefix(out, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is outside the workspace root %s: move it into the folder of the document, then add it again",
			filepath.Base(path), root)
	}
	base := v.root
	if v.state.Doc.Path != "" {
		base = filepath.Dir(v.state.Doc.Path)
	}
	if r, err := filepath.EvalSymlinks(base); err == nil {
		base = r
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return errors.New("the document cannot mount itself as a library")
	}
	name := strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
	if name == "" {
		name = "library"
	}
	taken := map[string]bool{}
	for _, e := range v.session.Root().Elements {
		taken[e.Name] = true
	}
	first := name
	for i := 2; taken[name]; i++ {
		name = first + " " + strconv.Itoa(i)
	}
	cmd := "element " + tenn.ToStr(tenn.NewStrNode(name), 0, false) + " " + scene.FromFileFlag + " " +
		tenn.ToStr(tenn.NewStrNode(rel), 0, false)
	at := len(v.text)
	pre := ""
	if at > 0 && v.text[at-1] != '\n' {
		pre = "\n"
	}
	return v.applyText(at, at, pre+cmd+"\n")
}

// showComponentPicker is Insert Component...: a filtered list of the components of the libraries and of the
// document itself, and a preview of the one chosen; Return or a double click inserts it.
func (v *Viewer) showComponentPicker() {
	if len(v.components("", 1)) == 0 {
		v.setBanner("No component to insert: add a library with Element > Add Library...")
		return
	}
	if !v.leaveMount() {
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4), g.Padding(6))
	var field *g.TextView
	var list *g.ListView
	var shot *g.Image
	var box *g.CanvasView
	var shown []Component
	var rows []int // the component of each row of the list, or -1 for a heading
	preview := func(row int) {
		if shot != nil {
			shot.Dispose()
			shot = nil
		}
		if row >= 0 && row < len(rows) && rows[row] >= 0 {
			img, err := v.componentPreviewImage(shown[rows[row]].Node, previewW, previewH, 4)
			if err != nil {
				// A component of slots is empty until it is filled: show the sign it is a component.
				if p := placeholderPreview(previewW, previewH, 4); p != nil {
					img = v.app.ImageFromProvider(p)
				}
			}
			shot = img
		}
		if box != nil {
			box.Redraw()
		}
	}
	fill := func(q string) {
		shown = v.components(q, 200)
		labels := make([]string, 0, len(shown)+8)
		rows = rows[:0]
		for i, c := range shown {
			if c.Group != "" && (i == 0 || c.Group != shown[i-1].Group) {
				labels = append(labels, c.Group)
				rows = append(rows, -1)
			}
			labels = append(labels, c.Name)
			rows = append(rows, i)
		}
		list.SetItems(labels)
		first := -1
		for i, c := range rows {
			if c >= 0 {
				first = i
				break
			}
		}
		if first >= 0 {
			list.Select(first)
			list.Unwrap().SetTopIndex(0)
		}
		preview(first)
	}
	pick := func(row int) {
		if row < 0 || row >= len(rows) || rows[row] < 0 {
			return
		}
		c := shown[rows[row]]
		p.Close()
		v.insertComponent(c.Path, c.Name)
	}
	paint := func(gc *g.GC) {
		w, h := gc.Size()
		raw := gc.Unwrap()
		raw.SetBackground(swt.DeviceGetDevice().GetSystemColor(swt.COLOR_LIST_BACKGROUND))
		raw.FillRectangle(0, 0, int32(w), int32(h))
		if shot == nil {
			return
		}
		iw, ih := shot.Size()
		gc.Image(shot, (w-iw)/2, (h-ih)/2)
	}
	p.Add(
		g.Text(g.Search(), g.Focused(), g.Hint("Component"), g.GrowX(), g.ID("component-filter"), g.Ref(&field), g.Init(func(t *g.TextView) {
			t.OnChange(fill)
			t.OnActivate(func() { pick(list.Index()) })
		})),
		g.HBox(
			g.List(nil, g.Size(340, previewH), g.ID("component-list"), g.Ref(&list), g.Init(func(l *g.ListView) {
				l.OnActivate(pick)
				l.OnSelect(preview)
			})),
			g.Canvas(paint, g.Size(previewW, previewH), g.ID("component-preview"), g.Ref(&box))))
	fill("")
	// The filter holds the focus, so the arrows would move its caret: they move the selection instead.
	field.Unwrap().AddKeyListener(swt.KeyListenerKeyPressedAdapter(func(e *swt.KeyEvent) {
		step := 0
		switch e.KeyCode {
		case int32(swt.ARROW_UP):
			step = -1
		case int32(swt.ARROW_DOWN):
			step = 1
		case int32(swt.PAGE_UP):
			step = -10
		case int32(swt.PAGE_DOWN):
			step = 10
		case int32(swt.HOME):
			step = -len(rows)
		case int32(swt.END):
			step = len(rows)
		default:
			return
		}
		e.Doit = false
		if len(rows) == 0 {
			return
		}
		i := max(0, min(list.Index()+step, len(rows)-1))
		dir := 1
		if step < 0 {
			dir = -1
		}
		for i >= 0 && i < len(rows) && rows[i] < 0 {
			i += dir // a heading is not a choice: the next component in the way
		}
		if i < 0 || i >= len(rows) {
			return
		}
		list.Select(i)
		list.Unwrap().ShowSelection()
		preview(i)
	}))
	v.openPopover(p)
	v.activeText = field.Unwrap()
	w, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), w/2, 0, g.PopoverBelow)
	// The preview is drawn once the popover is on screen: a canvas made before that never paints.
	v.app.Async(func() {
		if box != nil {
			box.Redraw()
		}
	})
}

// The preview pane of the component picker: a component is drawn whole, as large as this allows.
const (
	previewW = 320
	previewH = 280
)
