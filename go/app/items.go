//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/icons"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// itemsList is the second level of navigation: the items of the element on the canvas, in the sidebar
// under the outline.
type itemsList struct {
	box    *g.Panel
	hidden bool // collapsed by the Items button
	// components shows the component browser in the pane instead of the rows of the canvas: the rows are
	// then what the libraries offer, and a drag of one of them puts it on the canvas.
	components bool
	comps      []Component         // what the browser lists, kept for the drag of a row
	previews   map[string]*g.Image // the picture of a component row, by use path
	itemsBtn   *g.ButtonView       // the two modes of the pane; the pressed one is the one shown
	compsBtn   *g.ButtonView
	tree       *g.TreeView
	open       map[string]bool // expanded scene rows by path, for the canvas of canvasOf
	canvas     any
	sig        string // what the rows show, so a redraw that changes none of them keeps them
	syncing    bool
	grey       *swt.Color // the colour of instance rows in the theme greyDark
	greyDark   bool

	addCol  *g.Column // the column of the hover "+"
	nameCol *g.Column // the column of the name, as wide as the pane
	firstW  int       // the width of the name column
	addW    int       // the width of the "+" column in points
	addX    int       // the x of the "+" in the tree, for the tests
	hover   *g.Node   // the row that shows the "+"
}

// mouseMoveFunc is a MouseMoveListener for the hover "+" of the item rows.
type mouseMoveFunc func(*swt.MouseEvent)

func (f mouseMoveFunc) MouseMove(e *swt.MouseEvent) { f(e) }

// itemsPane is the item navigator: a header with Add, then the list alone, the element above it names
// the canvas.
func (v *Viewer) itemsPane() g.Option {
	l := &v.itemsUI
	return g.VBox(g.Ref(&l.box), g.Grow(), g.Padding(0), g.Gap(0),
		g.HBox(g.GrowX(), g.Padding(4), g.Gap(g.Scaled(4)),
			g.HBox(g.GrowX(), g.Gap(g.Scaled(2)),
				v.itemsModeButton("menu", "The items of this element", "items-mode-items", &l.itemsBtn, false),
				v.itemsModeButton("copy", "What the libraries offer: drag one onto the canvas, or double click it",
					"items-mode-components", &l.compsBtn, true)),
			g.Button("+", g.Small(), g.Size(g.Spacing.ControlHeight, g.Spacing.ControlHeight),
				g.ID("items-add"), g.Tooltip("Add an item"),
				g.Init(func(b *g.ButtonView) {
					m := g.PopupMenu(b)
					// The entries are made for the opening and gone when it closes, so the Add menu of
					// the menu bar stays the one a test finds by its label.
					b.OnClick(func() {
						fillMenu(m, v.addEntries())
						m.Show()
					})
					m.Unwrap().AddMenuListener(swt.MenuListenerMenuHiddenAdapter(func(*swt.MenuEvent) { fillMenu(m, nil) }))
				}))),
		g.Tree(g.NoBorder(), g.Grow(), g.ID("items"), g.Init(func(t *g.TreeView) {
			l.tree = t
			l.firstW = g.Scaled(220)
			l.nameCol = t.Column("", l.firstW)
			l.addCol = t.Column("", g.Scaled(24), g.Right())
			t.ShowHeader(false)
			t.OnSelect(v.onItemSelect)
			t.OnActivate(v.activateItem)
			t.OnExpand(v.onItemExpand)
			t.OnCollapse(func(n *g.Node) {
				if r, ok := n.Data().(*ItemRow); ok {
					delete(v.itemsUI.open, r.Path)
				}
			})
			tree := t.Unwrap()
			tree.AddMouseMoveListener(mouseMoveFunc(func(e *swt.MouseEvent) { v.hoverItemRow(int(e.X), int(e.Y)) }))
			tree.AddMouseTrackListener(swt.MouseTrackListenerMouseExitAdapter(func(*swt.MouseEvent) { v.hoverItemRow(-1, -1) }))
			tree.AddControlListener(swt.ControlListenerControlResizedAdapter(func(*swt.ControlEvent) { v.sizeItemsAdd() }))
			// A right click acts on the row under the pointer.
			t.OnCellClick(func(c g.TreeClick) {
				if c.Button == 3 && c.Node != nil {
					if sel := t.Selected(); len(sel) != 1 || sel[0].Data() != c.Node.Data() {
						t.Select(c.Node)
						v.onItemSelect(c.Node)
					}
				}
			})
			// A click in the "+" column adds a frame into that row, in its content or slot.
			tree.AddMouseListener(swt.MouseListenerMouseUpAdapter(func(e *swt.MouseEvent) {
				if e.Button != 1 {
					return
				}
				n, col := t.CellAt(int(e.X), int(e.Y))
				if col != 1 || !v.rowAdds(n) {
					return
				}
				if sel := t.Selected(); len(sel) != 1 || sel[0].Data() != n.Data() {
					t.Select(n)
					v.onItemSelect(n)
				}
				v.addSceneItem("Frame")
			}))
			v.sizeItemsAdd()
		})))
}

// sizeItemsAdd sizes the columns of the Items tree: the name fills the pane, the "+" keeps its narrow
// column at the right edge, clear of a scrollbar. It runs on every resize and after the rows are rebuilt.
func (v *Viewer) sizeItemsAdd() {
	l := &v.itemsUI
	if l.tree == nil || l.nameCol == nil || l.addCol == nil {
		return
	}
	w := int(l.tree.Unwrap().GetClientArea().Width)
	if w <= 0 {
		return
	}
	add := g.Scaled(24)
	name := w - add - g.Scaled(18)
	if name < add {
		name = add
	}
	if name == l.firstW && add == l.addW {
		return
	}
	l.addW, l.addX = add, name+add-8
	l.firstW = name
	l.nameCol.SetWidth(name)
	l.addCol.SetWidth(add)
}

// hoverItemRow puts the "+" on the row under the pointer, on no other row.
func (v *Viewer) hoverItemRow(x, y int) {
	l := &v.itemsUI
	var row *g.Node
	if x >= 0 && l.tree != nil {
		row, _ = l.tree.CellAt(x, y)
	}
	if row != nil && !v.rowAdds(row) {
		row = nil
	}
	if row == l.hover {
		return
	}
	if l.hover != nil {
		l.hover.SetTextAt(1, "")
	}
	l.hover = row
	if row != nil {
		row.SetTextAt(1, "+")
	}
}

// rowAdds reports whether a row takes a new node: a node written in the scene element, but not a use
// whose component has no default slot to put it in.
func (v *Viewer) rowAdds(row *g.Node) bool {
	if !v.sceneMode() || v.sv.node == nil {
		return false
	}
	r, ok := row.Data().(*ItemRow)
	if !ok || r.Path == "" {
		return false
	}
	n := NodeAt(v.sv.node, r.Path)
	if n == nil {
		return false
	}
	if n.Kind == scene.KUse {
		into, _ := InsertTarget(v.stree, n)
		return into != nil
	}
	return true
}

// toggleItems is the Items toolbar button: it collapses or expands the item pane under the outline.
func (v *Viewer) toggleItems() {
	l := &v.itemsUI
	l.hidden = !l.hidden
	l.box.SetVisible(!l.hidden)
	v.win.Unwrap().Layout()
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
	l.hover = nil
	l.tree.Batch(func() {
		l.tree.Clear()
		v.addItemRows(l.tree.Node, rows)
	})
	l.syncing = false
	v.sizeItemsAdd()
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

// itemsModeButton is one mode of the pane: an icon the width of a control, so the header fits the narrowest
// sidebar the window allows.
func (v *Viewer) itemsModeButton(icon, tip, id string, ref **g.ButtonView, components bool) g.Option {
	return g.Button("", g.Small(), g.Toggle(), g.Size(g.Spacing.ControlHeight, g.Spacing.ControlHeight),
		g.ID(id), g.Ref(ref), g.Tooltip(tip),
		g.Init(func(b *g.ButtonView) {
			b.SetImage(icons.Get(v.app, icon, icons.Size(12)))
			b.OnClick(func() { v.setItemsMode(components) })
		}))
}

// setItemsMode switches the pane between the rows of the canvas and the browser of what the libraries
// offer. The browser is a list to drag from: a row carries the use path of its component.
func (v *Viewer) setItemsMode(components bool) {
	l := &v.itemsUI
	if l.components == components {
		return
	}
	l.components = components
	l.sig = "" // the rows are what changed, not only their text
	if components {
		l.comps = v.components("", componentBrowserLimit) // a drop may come before the rows are read
	}
	v.rebuildItems()
}

// componentRows are the rows of the browser: one per component of the mounted libraries, labelled as the
// picker labels them.
func (v *Viewer) componentRows() []ItemRow {
	l := &v.itemsUI
	l.comps = v.components("", componentBrowserLimit)
	rows := make([]ItemRow, 0, len(l.comps))
	for _, c := range l.comps {
		rows = append(rows, ItemRow{Label: c.Label, Kind: "component", Path: c.Path, Component: true})
	}
	if len(rows) == 0 {
		// A hint instead of an empty list: the pane is where a reader looks when nothing can be inserted.
		hint := "The document mounts no library: add one with Element > Add Library..."
		if len(v.libraries()) > 0 {
			hint = "The libraries mount no component"
		}
		rows = append(rows, ItemRow{Label: hint, Kind: "hint"})
	}
	return rows
}

// componentOfPath is the component of a browser row, by the use path the row carries.
func (v *Viewer) componentOfPath(path string) *Component {
	for i := range v.itemsUI.comps {
		if v.itemsUI.comps[i].Path == path {
			return &v.itemsUI.comps[i]
		}
	}
	return nil
}

// itemRows are the top rows for the canvas.
func (v *Viewer) itemRows() []ItemRow {
	switch {
	case v.itemsUI.components:
		return v.componentRows()
	case v.sceneMode():
		if v.sv.node == nil {
			return nil
		}
		return SceneRows(v.sv.node, "")
	case v.session != nil:
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
	if v.sv.opensParts(p) {
		return v.sv.instRows(v.sv.byPath[p])
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
		n.SetImage(v.rowImage(r))
		if instanceRow(r) {
			n.Unwrap().SetForeground(v.greyColor())
		}
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

// instanceRow reports whether the row is a reuse of a component: a use, or a part of an instance, which
// only an override in the use changes.
func instanceRow(r *ItemRow) bool {
	return r.Kind == "use" || strings.Contains(r.Path, "/")
}

// greyColor is the text of instance rows. gowt has no foreground for a tree node, so it is the system
// secondary colour by hand; themeChanged rebuilds the rows to take the other one.
func (v *Viewer) greyColor() *swt.Color {
	l := &v.itemsUI
	if dark := v.app.Dark(); l.grey == nil || l.greyDark != dark {
		c := swt.NewColorRedGreenBlue(128, 128, 134)
		if dark {
			c = swt.NewColorRedGreenBlue(152, 152, 158)
		}
		l.grey, l.greyDark = c, dark
	}
	return l.grey
}

// componentSVG is the icon of a node a use can take (Lucide puzzle, ISC).
const componentSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19.439 7.85c-.049.322.059.648.289.878l1.568 1.568c.47.47.706 1.087.706 1.704s-.235 1.233-.706 1.704l-1.611 1.611a.98.98 0 0 1-.837.276c-.47-.07-.802-.48-.968-.925a2.501 2.501 0 1 0-3.214 3.214c.446.166.855.497.925.968a.979.979 0 0 1-.276.837l-1.61 1.61a2.404 2.404 0 0 1-1.705.707 2.402 2.402 0 0 1-1.704-.706l-1.568-1.568a1.026 1.026 0 0 0-.877-.29c-.493.074-.84.504-1.02.968a2.5 2.5 0 1 1-3.237-3.237c.464-.18.894-.527.967-1.02a1.026 1.026 0 0 0-.289-.877l-1.568-1.568A2.402 2.402 0 0 1 1.998 12c0-.617.236-1.234.706-1.704L4.23 8.77c.24-.24.581-.353.917-.303.515.077.877.528 1.073 1.01a2.5 2.5 0 1 0 3.259-3.259c-.482-.196-.933-.558-1.01-1.073-.05-.336.062-.676.303-.917l1.525-1.525A2.402 2.402 0 0 1 12 1.998c.617 0 1.234.236 1.704.706l1.568 1.568c.23.23.556.338.877.29.493-.074.84-.504 1.02-.968a2.5 2.5 0 1 1 3.237 3.237c-.464.18-.894.527-.967 1.02Z"/></svg>`

// itemImage is the icon of a row: a puzzle for a node a use can take, the named system icon otherwise.
// componentBrowserLimit is how many components the pane of the browser lists.
const componentBrowserLimit = 400

// The size of the preview of a component row in the browser, in points.
const (
	componentRowW = 32
	componentRowH = 22
)

// rowImage is the picture of a row: the preview of a component in the browser, the kind icon elsewhere.
func (v *Viewer) rowImage(r *ItemRow) *g.Image {
	if !v.itemsUI.components || !r.Component {
		return v.itemImage(r)
	}
	l := &v.itemsUI
	if l.previews == nil {
		l.previews = map[string]*g.Image{}
	}
	if img, ok := l.previews[r.Path]; ok {
		return img
	}
	c := v.componentOfPath(r.Path)
	if c == nil || c.Node == nil {
		return v.itemImage(r)
	}
	img, err := v.componentImage(c.Node, componentRowW, componentRowH)
	if err != nil || img == nil {
		return v.itemImage(r)
	}
	l.previews[r.Path] = img
	return img
}

func (v *Viewer) itemImage(r *ItemRow) *g.Image {
	if r.Component {
		c := g.RGB{R: 30, G: 30, B: 30}
		if v.app.Dark() {
			c = g.RGB{R: 235, G: 235, B: 235}
		}
		return icons.FromSVG(v.app, []byte(componentSVG), 16, c)
	}
	return icons.Get(v.app, itemIcon(r), icons.Size(16))
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
	v.setCamera(CenterOn(gfx.Camera{Zoom: z}, c, float64(w), float64(h)))
	v.drawSceneOverlay()
}

// revealItem centres a diagram item that is not all on the canvas.
func (v *Viewer) revealItem(it *model.DiagramItem) {
	pl := v.classicPlaced(it)
	if pl == nil {
		return
	}
	box := gfx.R(pl.X+v.sv.off.X, pl.Y+v.sv.off.Y, pl.W, pl.H)
	w, h := v.view.Canvas().Size()
	comp := v.view.Compositor()
	cam := comp.Camera()
	x, y := (box.Min.X-cam.Origin.X)*cam.Zoom, (box.Min.Y-cam.Origin.Y)*cam.Zoom
	if x >= 0 && y >= 0 && x+box.W()*cam.Zoom <= float64(w) && y+box.H()*cam.Zoom <= float64(h) {
		return
	}
	v.setCamera(CenterOn(cam, box.Center(), float64(w), float64(h)))
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
		{Label: "Add", Sub: []editor.Entry{
			{Label: "Frame", Run: func() { v.addSceneItem("Frame") }},
			{Label: "Row", Run: func() { v.addSceneItem("Row") }},
			{Label: "Text", Run: func() { v.addSceneItem("Text") }},
			{Label: "Box", Run: func() { v.addSceneItem("Box") }},
		}},
		{Sep: true},
	}
	if NodeAt(v.sv.node, r.Path) != nil {
		entries = append(entries,
			editor.Entry{Label: "Move", Sub: []editor.Entry{
				{Label: "Up", Run: func() { v.moveNodeBy(-1) }},
				{Label: "Down", Run: func() { v.moveNodeBy(1) }},
				{Sep: true},
				{Label: "Into Previous", Run: func() { v.moveNodeInto(-1) }},
				{Label: "Into Next", Run: func() { v.moveNodeInto(1) }},
			}},
			editor.Entry{Sep: true})
	}
	entries = append(entries,
		editor.Entry{Label: "Duplicate", Run: v.duplicateScene},
		editor.Entry{Label: "Zoom to Item", Run: v.zoomToSelection},
		editor.Entry{Sep: true},
		editor.Entry{Label: "Delete", Run: v.deleteSceneNode})
	if r.Kind == "use" {
		entries = append([]editor.Entry{{Label: "Go to Component", Run: func() {
			v.goToComponentNode(NodeAt(v.sv.node, r.Path))
		}}, {Sep: true}}, entries...)
	}
	if m := model.MountOf(v.state.Current, false); m != nil {
		entries = append([]editor.Entry{{Label: "Insert Component", Run: func() { v.insertFromMount(m, r) }}, {Sep: true}}, entries...)
	}
	return entries
}
