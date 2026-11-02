//go:build darwin || linux || windows

package app

import (
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/icons"

	"github.com/haiodo/tenniarb/go/diff"
	"github.com/haiodo/tenniarb/go/editor"
)

// The comparison of the file with a version from git: the outline lists what changed node by node, the
// canvas draws the new version with a frame over every changed node, and a node can go back to the
// version of the last commit as one ordinary edit.

// compare is compare mode of a window.
type compare struct {
	repo  string       // the top folder of the repository
	rev   string       // the version compared with, HEAD
	old   string       // the text of the file at rev
	rep   *diff.Report // the comparison of old with the text of the document
	sig   string       // what the outline was last built from
	about string       // what the last build showed on the canvas, for the QA state
}

// compareColors is the frame colour of a status on the canvas.
var compareColors = map[diff.Status]gfx.Color{
	diff.Added:   gfx.RGBA(22, 163, 74, 1),
	diff.Removed: gfx.RGBA(220, 38, 38, 1),
	diff.Changed: gfx.RGBA(217, 119, 6, 1),
	diff.Moved:   gfx.RGBA(37, 99, 235, 1),
}

// compareTints are the tints of the row icons for a status, in the light appearance.
var compareTints = map[diff.Status]g.RGB{
	diff.Added:   {R: 22, G: 163, B: 74},
	diff.Removed: {R: 220, G: 38, B: 38},
	diff.Changed: {R: 217, G: 119, B: 6},
	diff.Moved:   {R: 37, G: 99, B: 235},
}

// gitRepo is the repository of the document, or false when the file is not in one.
func (v *Viewer) gitRepo() (root string, ok bool) {
	dir := ""
	if p := v.state.Doc.Path; p != "" {
		dir = filepath.Dir(p)
	}
	if dir == "" {
		return "", false
	}
	if dir == v.repoDir {
		return v.repoRoot, v.repoOK
	}
	root, ok = diff.Repo(dir)
	v.repoDir, v.repoRoot, v.repoOK = dir, root, ok
	return root, ok
}

// compareToggle turns compare mode on for the document, or off.
func (v *Viewer) compareToggle() {
	if v.cmp != nil {
		v.closeCompare()
		return
	}
	v.startCompare()
}

// startCompare reads the file at HEAD and compares it with the text of the document.
func (v *Viewer) startCompare() {
	if v.state.Doc.Path == "" {
		v.setBanner("Save the document first: a comparison needs a file in a repository")
		return
	}
	root, ok := v.gitRepo()
	if !ok {
		v.setBanner("The file is not in a git repository: there is no version to compare with")
		return
	}
	rel, err := filepath.Rel(root, realPath(v.state.Doc.Path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		v.setBanner("The file is outside the repository: there is no version to compare with")
		return
	}
	old, err := diff.Show(root, diff.RevHead, rel)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	rep, err := diff.Compare(v.root, v.rel(), old, v.text)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.cmp = &compare{repo: root, rev: diff.RevHead, old: old, rep: rep}
	v.paintCompare()
	v.drawSceneOverlay()
	v.updateMenus()
}

// closeCompare leaves compare mode and puts the ordinary outline back.
func (v *Viewer) closeCompare() {
	if v.cmp == nil {
		return
	}
	v.cmp = nil
	v.rebuildOutline(nil)
	v.drawSceneOverlay()
	v.updateMenus()
	v.setBanner("")
}

// refreshCompare reads the comparison again after the text changed, and redraws the rows and the frames.
func (v *Viewer) refreshCompare() {
	if v.cmp == nil {
		return
	}
	rep, err := diff.Compare(v.root, v.rel(), v.cmp.old, v.text)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.cmp.rep = rep
	v.paintCompare()
	v.drawSceneOverlay()
}

// paintCompare fills the outline with the comparison when the nodes or their statuses changed.
func (v *Viewer) paintCompare() {
	if v.cmp == nil || v.cmp.rep == nil {
		return
	}
	sig := compareSig(v.cmp.rep)
	if sig == v.cmp.sig {
		return
	}
	expanded := v.compareExpanded()
	first := v.cmp.sig == ""
	v.cmp.sig = sig
	v.buildCompareTree(expanded, first)
	v.cmp.about = compareSummary(v.cmp.rep)
	v.setNote(v.cmp.about)
}

// compareSig is what the rows look like: a path and a status per node, and the properties of a changed one.
func compareSig(r *diff.Report) string {
	var b strings.Builder
	var walk func(n *diff.Node)
	walk = func(n *diff.Node) {
		b.WriteString(n.Path)
		b.WriteByte(' ')
		b.WriteString(string(n.Status))
		b.WriteByte(' ')
		b.WriteString(n.Note)
		for _, p := range n.Props {
			b.WriteString(" " + p.Name + "=" + p.Old + ">" + p.New)
		}
		b.WriteString("\n")
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(r.Root)
	return b.String()
}

// compareSummary is the line of a comparison: how many nodes changed, with the version.
func compareSummary(r *diff.Report) string {
	if !r.Changed() {
		return "No changes against " + diff.RevHead
	}
	var parts []string
	for _, s := range []diff.Status{diff.Changed, diff.Added, diff.Removed, diff.Moved} {
		if n := r.Count(s); n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+string(s))
		}
	}
	return "Against " + diff.RevHead + ": " + strings.Join(parts, ", ")
}

// compareExpanded is the set of comparison rows that are open now.
func (v *Viewer) compareExpanded() map[string]bool {
	out := map[string]bool{}
	var walk func(ns []*g.Node)
	walk = func(ns []*g.Node) {
		for _, n := range ns {
			if d, ok := n.Data().(*diff.Node); ok && n.Expanded() {
				out[d.Path] = true
			}
			walk(n.Children())
		}
	}
	walk(v.tree.Roots())
	return out
}

// buildCompareTree fills the outline with the elements and their changed nodes.
func (v *Viewer) buildCompareTree(expanded map[string]bool, first bool) {
	rep := v.cmp.rep
	v.syncing = true
	defer func() { v.syncing = false }()
	v.tree.Batch(func() {
		v.tree.Clear()
		n := v.tree.Node(v.compareLabel(rep.Root))
		n.SetData(rep.Root)
		n.SetImage(v.compareIcon(rep.Root))
		v.addCompareKids(n, rep.Root.Kids, expanded, first)
		n.SetExpanded(true) // after the rows are there: an item without kids does not expand
	})
	v.outlineSig = ""
	v.sizeOutlineColumn()
}

// addCompareKids adds the rows under parent: every element, and the nodes that changed or hold a change.
func (v *Viewer) addCompareKids(parent *g.Node, kids []*diff.Node, expanded map[string]bool, first bool) {
	for _, k := range kids {
		if k.Kind != "element" && !compareShows(k) {
			continue
		}
		n := parent.Node(v.compareLabel(k))
		n.SetData(k)
		n.SetImage(v.compareIcon(k))
		for _, p := range k.Props {
			n.Node(propLabel(p))
		}
		if k.Note != "" {
			n.Node(k.Note)
		}
		v.addCompareKids(n, k.Kids, expanded, first)
		n.SetExpanded(first || expanded[k.Path] || k.Status != "" && len(k.Props) > 0)
	}
}

// compareShows reports whether a node is listed: it changed, or a node under it did.
func compareShows(n *diff.Node) bool {
	if n.Status != "" {
		return true
	}
	for _, k := range n.Kids {
		if compareShows(k) {
			return true
		}
	}
	return false
}

// compareLabel is the text of a row: the name, the badge of its status and, for an element, how many
// nodes under it changed.
func (v *Viewer) compareLabel(n *diff.Node) string {
	label := n.Name
	if n.Status != "" {
		label += "  " + string(n.Status)
	}
	if n.Kind == "element" {
		if c := compareCount(n); c > 0 {
			label += "  " + strconv.Itoa(c)
		}
	}
	return label
}

// compareCount is how many nodes under n, itself included, have a status.
func compareCount(n *diff.Node) int {
	c := 0
	if n.Status != "" {
		c++
	}
	for _, k := range n.Kids {
		c += compareCount(k)
	}
	return c
}

// propLabel is the row of one changed property: "width  240 -> 200".
func propLabel(p diff.Prop) string {
	old, new := oneLine(p.Old), oneLine(p.New)
	switch {
	case old != "" && new != "":
		return p.Name + "  " + old + " -> " + new
	case new != "":
		return p.Name + "  " + new
	case old != "":
		return p.Name + "  was " + old
	}
	return p.Name
}

// oneLine makes a property value one short line: a template or a style can be long.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// compareIcon is the row icon: the kind of the node, tinted by its status.
func (v *Viewer) compareIcon(n *diff.Node) *g.Image {
	name := "stop" // item: a box
	switch n.Kind {
	case "element":
		name = "folder"
	case "use":
		name = "copy"
	case "link":
		name = "forward"
	case "file":
		name = "file"
	}
	if c, ok := compareTints[n.Status]; ok {
		if v.app.Dark() {
			c = g.RGB{R: 255 - c.R/2, G: 255 - c.G/2, B: 255 - c.B/2}
		}
		return icons.Get(v.app, name, icons.Size(16), icons.Color(c))
	}
	return icons.Get(v.app, name, icons.Size(16))
}

// selectCompare puts the element of a row on the canvas, and its node when it is one.
func (v *Viewer) selectCompare(n *diff.Node) {
	if v.session == nil || n.Element == "" {
		return
	}
	e := Resolve(&v.session.Root().Element, strings.Split(n.Element, "/"))
	if e == nil {
		return // the element is gone in this version: nothing to show
	}
	nodes := []string{}
	if n.Sel != "" {
		nodes = []string{n.Sel}
	}
	v.showSelection(e, nodes, nil, true)
}

// compareEntries is the menu of a row in compare mode.
func (v *Viewer) compareEntries() []editor.Entry {
	var entries []editor.Entry
	if sel := v.tree.Selected(); len(sel) == 1 {
		if n, ok := sel[0].Data().(*diff.Node); ok {
			entries = append(entries,
				editor.Entry{Label: "Revert to " + diff.RevHead, Run: func() { v.revertNode(n.Path) }},
				editor.Entry{Label: "Show on Canvas", Run: func() { v.selectCompare(n) }},
				editor.Entry{Sep: true})
		}
	}
	return append(entries, editor.Entry{Label: "Stop Comparison", Run: v.closeCompare})
}

// revertNode writes the node at path as the version from git has it, as one undo step.
func (v *Viewer) revertNode(path string) {
	if v.cmp == nil || v.cmp.rep == nil {
		return
	}
	n := v.cmp.rep.Node(path)
	if n == nil || n.Status == "" {
		v.setBanner("The node is the same as " + diff.RevHead)
		return
	}
	e, ok := v.cmp.rep.Revert(path)
	if !ok {
		v.setBanner("Not applied: the node has no place in this version; revert the node above it first")
		return
	}
	target := v.stree.File(v.rel())
	if n.Sel != "" && v.sv != nil {
		if hit := NodeAt(v.sv.node, n.Sel); hit != nil {
			target = hit // the pulse of the edit lands on the node
		}
	}
	if !v.sceneEdit(target, TextEdit{Start: e.Start, End: e.End, New: e.New}) {
		return
	}
	v.refreshCompare()
}

// comparePicture draws a frame over every node that changed, added or moved, in the colour of its status.
func (v *Viewer) comparePicture(cam gfx.Camera) *gfx.Picture {
	rec := gfx.NewRecorder()
	if v.cmp == nil || v.cmp.rep == nil {
		return rec.Finish()
	}
	if cam.Zoom <= 0 {
		cam.Zoom = 1
	}
	for _, s := range []diff.Status{diff.Changed, diff.Added, diff.Moved, diff.Removed} {
		for _, p := range v.cmp.rep.Paths(s) {
			r, ok := v.compareRect(p)
			if !ok {
				continue
			}
			x, y := (r.Min.X-cam.Origin.X)*cam.Zoom, (r.Min.Y-cam.Origin.Y)*cam.Zoom
			rec.StrokeRect(gfx.R(x, y, r.W()*cam.Zoom, r.H()*cam.Zoom), gfx.StrokeStyle{Width: 2}, compareColors[s])
		}
	}
	return rec.Finish()
}

// compareRect is where the canvas draws a node of the comparison: the path of the report names it from
// the file root, while the canvas draws the element on it and names the nodes from that element down.
func (v *Viewer) compareRect(path string) (gfx.Rect, bool) {
	if r, ok := v.markRect(path); ok {
		return r, true
	}
	n := v.cmp.rep.Node(path)
	if n == nil || n.Sel == "" || v.sv == nil || v.state.Current == nil {
		return gfx.Rect{}, false
	}
	if strings.Join(PathOf(v.state.Current), "/") != n.Element {
		return gfx.Rect{}, false
	}
	pl := v.sv.byPath[n.Sel]
	if pl == nil {
		return gfx.Rect{}, false
	}
	return gfx.R(v.sv.off.X+pl.X, v.sv.off.Y+pl.Y, pl.W, pl.H), true
}

// syncCompareHUD draws the frames of compare mode on the HUD, or takes the layer away when it is off.
func (v *Viewer) syncCompareHUD() {
	if v.view == nil {
		return
	}
	comp := v.view.Compositor()
	if v.cmp == nil {
		if v.diffHUD == nil {
			return
		}
		node := v.diffHUD
		v.diffHUD = nil
		comp.HUD().Update(func(tx *gfx.Tx) { tx.Remove(node) })
		return
	}
	pic := v.comparePicture(comp.Camera())
	comp.HUD().Update(func(tx *gfx.Tx) {
		if v.diffHUD == nil {
			v.diffHUD = gfx.NewNode(5)
			tx.Add(nil, v.diffHUD)
			tx.SetShape(v.diffHUD, nil, gfx.HitNone)
		}
		tx.SetPicture(v.diffHUD, pic)
	})
}

// compareMarkerPaths lists the changed nodes the canvas draws a frame over, for the QA state.
func (v *Viewer) compareMarkerPaths() []string {
	out := []string{}
	if v.cmp == nil || v.cmp.rep == nil {
		return out
	}
	for _, s := range []diff.Status{diff.Changed, diff.Added, diff.Moved, diff.Removed} {
		for _, p := range v.cmp.rep.Paths(s) {
			if _, ok := v.compareRect(p); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

// qaCompare is compare mode for the QA state of a window.
type qaCompare struct {
	On      bool         `json:"on"`
	Rev     string       `json:"rev"`
	Repo    string       `json:"repo"`
	About   string       `json:"about"`
	Nodes   []qaDiffNode `json:"nodes"`
	Markers []string     `json:"markers"`
}

type qaDiffNode struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Props  int    `json:"props"`
}

func (v *Viewer) qaCompare() qaCompare {
	out := qaCompare{Nodes: []qaDiffNode{}, Markers: v.compareMarkerPaths()}
	if v.cmp == nil {
		return out
	}
	out.On, out.Rev, out.Repo, out.About = true, v.cmp.rev, v.cmp.repo, v.cmp.about
	if v.cmp.rep == nil {
		return out
	}
	var walk func(n *diff.Node)
	walk = func(n *diff.Node) {
		if n.Status != "" {
			out.Nodes = append(out.Nodes, qaDiffNode{Path: n.Path, Kind: n.Kind, Status: string(n.Status), Props: len(n.Props)})
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(v.cmp.rep.Root)
	return out
}
