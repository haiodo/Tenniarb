//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/icons"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// itemsList is the second level of navigation: the items of the element on the canvas, in the sidebar
// under the outline.
type itemsList struct {
	box     *g.Panel
	title   *g.LabelView
	hidden  bool // collapsed by the Items button
	tree    *g.TreeView
	open    map[string]bool // expanded scene rows by path, for the canvas of canvasOf
	canvas  any
	sig     string // what the rows show, so a redraw that changes none of them keeps them
	syncing bool
}

// itemsPane is the item navigator: the element name and the list.
func (v *Viewer) itemsPane() g.Option {
	return g.VBox(g.Ref(&v.itemsUI.box), g.Grow(), g.Padding(0), g.Gap(0),
		g.Label("", g.Ref(&v.itemsUI.title), g.GrowX(), g.Padding(g.Scaled(8)), g.Typeface(sectionFont())),
		g.Tree(g.NoBorder(), g.Grow(), g.ID("items"), g.Init(func(t *g.TreeView) {
			v.itemsUI.tree = t
			t.OnSelect(v.onItemSelect)
			t.OnExpand(v.onItemExpand)
			t.OnCollapse(func(n *g.Node) {
				if r, ok := n.Data().(*ItemRow); ok {
					delete(v.itemsUI.open, r.Path)
				}
			})
			// A right click acts on the row under the pointer.
			t.OnCellClick(func(c g.TreeClick) {
				if c.Button == 3 && c.Node != nil {
					if sel := t.Selected(); len(sel) != 1 || sel[0].Data() != c.Node.Data() {
						t.Select(c.Node)
						v.onItemSelect(c.Node)
					}
				}
			})
		})))
}

// toggleItems is the Items toolbar button: it collapses or expands the item pane under the outline.
func (v *Viewer) toggleItems() {
	l := &v.itemsUI
	l.hidden = !l.hidden
	l.box.SetVisible(!l.hidden)
	v.win.Unwrap().Layout()
}

// itemsTitle names the canvas the navigator shows.
func (v *Viewer) itemsTitle() string {
	if v.state.Current != nil {
		return v.state.Current.Name
	}
	return ""
}

// canvasOf names what is on the canvas, so the open rows are forgotten when it changes.
func (v *Viewer) canvasOf() any {
	return v.state.Current
}

// rebuildItems fills the list for the canvas: the nodes of a scene, the items of a diagram.
func (v *Viewer) rebuildItems() {
	l := &v.itemsUI
	if l.tree == nil {
		return
	}
	l.title.SetText(v.itemsTitle())
	if c := v.canvasOf(); c != l.canvas || l.open == nil {
		l.canvas, l.open = c, map[string]bool{}
	}
	rows := v.itemRows()
	sig := rowsSig(rows)
	for p := range l.open {
		sig += "\x01" + p + rowsSig(v.rowsAt(p))
	}
	if sig == l.sig {
		v.syncItems()
		return
	}
	l.sig = sig
	l.syncing = true
	l.tree.Batch(func() {
		l.tree.Clear()
		v.addItemRows(l.tree.Node, rows)
	})
	l.syncing = false
	v.syncItems()
}

func rowsSig(rows []ItemRow) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.Label + "\x02" + r.Kind + "\x02" + r.Path)
		if r.Item != nil {
			fmt.Fprintf(&b, "%p", r.Item) // a text edit makes new items with the same names
		}
		if r.Kids {
			b.WriteByte('+')
		}
		b.WriteByte('\x03')
	}
	return b.String()
}

// itemRows are the top rows for the canvas.
func (v *Viewer) itemRows() []ItemRow {
	switch {
	case v.sceneMode():
		if v.sv.node == nil {
			return nil
		}
		return SceneRows(v.sv.node, "")
	case v.session != nil && v.rendered != nil:
		return DiagramRows(v.session.Element())
	}
	return nil
}

// rowsAt are the rows under the node of path p: the drawn kids of an instance, which hold the parts of
// its component, else the nodes written under it.
func (v *Viewer) rowsAt(p string) []ItemRow {
	if !v.sceneMode() {
		return nil
	}
	if pl := v.sv.byPath[p]; pl != nil && (pl.Inst || strings.Contains(p, "/")) {
		return v.sv.instRows(pl)
	}
	if n := NodeAt(v.sv.node, p); n != nil {
		return v.writtenRows(n, p)
	}
	return nil
}

// writtenRows are SceneRows, an instance opens when it draws anything.
func (v *Viewer) writtenRows(n *scene.Node, p string) []ItemRow {
	rows := SceneRows(n, p)
	for i, r := range rows {
		if pl := v.sv.byPath[r.Path]; r.Kind == "use" && pl != nil && len(pl.Kids) > 0 {
			rows[i].Kids = true
		}
	}
	return rows
}

func (v *Viewer) addItemRows(parent func(string) *g.Node, rows []ItemRow) {
	for i := range rows {
		r := &rows[i]
		n := parent(r.Label)
		n.SetData(r)
		n.SetImage(icons.Get(v.app, itemIcon(r), icons.Size(16)))
		if !r.Kids {
			continue
		}
		if v.itemsUI.open[r.Path] {
			v.fillItemRow(n, r)
			n.SetExpanded(true)
			continue
		}
		n.Node("") // the rows are made when it opens
	}
}

func itemIcon(r *ItemRow) string {
	switch {
	case r.Kind == "use":
		return "copy"
	case r.Kind == "slot":
		return "open"
	case r.Kind == "link":
		return "forward"
	case r.Kids:
		return "menu"
	}
	return "file"
}

func (v *Viewer) fillItemRow(row *g.Node, r *ItemRow) {
	for _, c := range row.Children() {
		c.Remove()
	}
	v.addItemRows(row.Node, v.rowsAt(r.Path))
}

func (v *Viewer) onItemExpand(row *g.Node) {
	r, ok := row.Data().(*ItemRow)
	if !ok || r.Path == "" {
		return
	}
	v.itemsUI.open[r.Path] = true
	if kids := row.Children(); len(kids) == 1 && kids[0].Data() == nil {
		v.itemsUI.syncing = true
		v.fillItemRow(row, r)
		v.itemsUI.syncing = false
	}
}

// onItemSelect selects the item of a row on the canvas and brings it into view.
func (v *Viewer) onItemSelect(n *g.Node) {
	if v.itemsUI.syncing || n == nil {
		return
	}
	r, ok := n.Data().(*ItemRow)
	if !ok {
		return
	}
	switch {
	case r.Path != "" && v.sceneMode():
		v.selectItemPath(r.Path)
	case r.Item != nil && v.session != nil:
		v.session.Select([]*model.DiagramItem{r.Item})
		v.revealItem(r.Item)
	}
}

// selectItemPath selects the scene node of a path and brings it into view.
func (v *Viewer) selectItemPath(p string) {
	v.selectScene([]string{p})
	v.revealNode()
}

// syncItems selects the row of what is selected on the canvas, opening the rows above it.
func (v *Viewer) syncItems() {
	v.navSync()
	l := &v.itemsUI
	if l.tree == nil {
		return
	}
	l.syncing = true
	defer func() { l.syncing = false }()
	var row *g.Node
	switch {
	case v.sceneMode() && len(v.selNodes) > 0:
		row = v.itemRowOf(v.selNodes[0])
	case !v.sceneMode() && v.session != nil && len(v.session.Selection()) == 1:
		it := v.session.Selection()[0]
		for _, n := range l.tree.Roots() {
			if r, ok := n.Data().(*ItemRow); ok && r.Item == it {
				row = n
				break
			}
		}
	}
	if row == nil {
		l.tree.Deselect()
		return
	}
	if sel := l.tree.Selected(); len(sel) == 1 && sel[0].Data() == row.Data() {
		return
	}
	l.tree.Select(row)
}

// itemRowOf is the row of a node path; the rows above it are opened, and made when they were not.
func (v *Viewer) itemRowOf(p string) *g.Node {
	rows := v.itemsUI.tree.Roots()
	chain := v.sv.rowChain(p)
	for i, want := range chain {
		var found *g.Node
		for _, n := range rows {
			if r, ok := n.Data().(*ItemRow); ok && r.Path == want {
				found = n
				break
			}
		}
		if found == nil {
			return nil
		}
		if i == len(chain)-1 {
			return found
		}
		r := found.Data().(*ItemRow)
		if kids := found.Children(); len(kids) == 1 && kids[0].Data() == nil {
			v.fillItemRow(found, r)
		}
		v.itemsUI.open[r.Path] = true
		found.SetExpanded(true)
		rows = found.Children()
	}
	return nil
}

// revealNode brings the first selected scene node into view when it is not all on the canvas: centred
// and zoomed to fit, but not past 100% or the zoom already there, so a small item is not blown up.
func (v *Viewer) revealNode() {
	_, pl := v.selectedScene()
	if pl == nil || pl.W <= 0 || pl.H <= 0 {
		return
	}
	w, h := v.view.Canvas().Size()
	cam := v.view.Compositor().Camera()
	x, y := pl.X+v.sv.off.X, pl.Y+v.sv.off.Y
	vx, vy := (x-cam.Origin.X)*cam.Zoom, (y-cam.Origin.Y)*cam.Zoom
	if vx >= 0 && vy >= 0 && vx+pl.W*cam.Zoom <= float64(w) && vy+pl.H*cam.Zoom <= float64(h) {
		return
	}
	z := ClampZoom(math.Min(0.9*math.Min(float64(w)/pl.W, float64(h)/pl.H), math.Max(cam.Zoom, 1)))
	c := gfx.Point{X: x + pl.W/2, Y: y + pl.H/2}
	v.view.Compositor().SetCamera(CenterOn(gfx.Camera{Zoom: z}, c, float64(w), float64(h)))
	v.drawSceneOverlay()
}

// revealItem centres a diagram item that is not all on the canvas.
func (v *Viewer) revealItem(it *model.DiagramItem) {
	d := v.session.Scene().Drawables[it]
	if d == nil || v.rendered == nil {
		return
	}
	b, vis := d.Bounds(), v.visibleScene()
	if b.X >= vis.X && b.Y >= vis.Y && b.X+b.Width <= vis.X+vis.Width && b.Y+b.Height <= vis.Y+vis.Height {
		return
	}
	c := v.smap.ToGfx(render.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2})
	w, h := v.view.Canvas().Size()
	comp := v.view.Compositor()
	comp.SetCamera(CenterOn(comp.Camera(), gfx.Point{X: c.X, Y: c.Y}, float64(w), float64(h)))
}

// itemsEntries is the context menu of the list: the commands of the selected node, and in a mounted
// element Insert Component.
func (v *Viewer) itemsEntries() []editor.Entry {
	sel := v.itemsUI.tree.Selected()
	if len(sel) != 1 || v.session == nil {
		return nil
	}
	r, ok := sel[0].Data().(*ItemRow)
	if !ok {
		return nil
	}
	if r.Item != nil {
		return []editor.Entry{{Label: "Duplicate", Run: v.session.Duplicate}, {Label: "Delete", Run: v.session.DeleteSelection}}
	}
	entries := []editor.Entry{
		{Label: "Duplicate", Run: v.duplicateScene},
		{Label: "Zoom to Item", Run: v.zoomToSelection},
		{Sep: true},
		{Label: "Delete", Run: v.deleteSceneNode},
	}
	if m := model.MountOf(v.state.Current, false); m != nil {
		entries = append([]editor.Entry{{Label: "Insert Component", Run: func() { v.insertFromMount(m, r) }}, {Sep: true}}, entries...)
	}
	return entries
}
