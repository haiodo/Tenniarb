//go:build darwin || linux || windows

package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
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
// element is there: a use of a component belongs to the document, not to the file it comes from.
func (v *Viewer) leaveMount() bool {
	if model.MountOf(v.state.Current, false) == nil {
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
		v.setBanner("Only an item of the library, not inside an instance or a slot, can be inserted.")
		return
	}
	if v.leaveMount() {
		v.insertComponent(p, n.Name)
	}
}

// components are what the imported libraries offer for the query.
func (v *Viewer) components(q string, limit int) []Component {
	var out []Component
	for _, r := range v.libraries() {
		if n := v.stree.File(r.file); n != nil {
			out = append(out, Components(r.lib, n, q, limit-len(out))...)
		}
	}
	return out
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

// showComponentPicker is Insert Component...: a filtered list of the components of the libraries;
// Return or a double click inserts the one chosen.
func (v *Viewer) showComponentPicker() {
	if len(v.libraries()) == 0 {
		v.setBanner("The document mounts no library: add one with Element > Add Library...")
		return
	}
	if !v.leaveMount() {
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4), g.Padding(6))
	var field *g.TextView
	var list *g.ListView
	var shown []Component
	fill := func(q string) {
		shown = v.components(q, 200)
		labels := make([]string, len(shown))
		for i, c := range shown {
			labels[i] = c.Label
		}
		list.SetItems(labels)
		if len(labels) > 0 {
			list.Select(0)
		}
	}
	pick := func(i int) {
		if i < 0 || i >= len(shown) {
			return
		}
		c := shown[i]
		p.Close()
		v.insertComponent(c.Path, c.Name)
	}
	p.Add(
		g.Text(g.Search(), g.Focused(), g.Hint("Component"), g.ID("component-filter"), g.Ref(&field), g.Init(func(t *g.TextView) {
			t.OnChange(fill)
			t.OnActivate(func() { pick(list.Index()) })
		})),
		g.List(nil, g.Size(340, 280), g.ID("component-list"), g.Ref(&list), g.Init(func(l *g.ListView) {
			l.OnActivate(pick)
		})))
	fill("")
	v.openPopover(p)
	v.activeText = field.Unwrap()
	w, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), w/2, 0, g.PopoverBelow)
}
