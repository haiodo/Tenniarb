//go:build darwin || linux || windows

package app

import (
	"math"
	"slices"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// sceneView is a scene element on the canvas, drawn by go/scene.
type sceneView struct {
	el      *model.Element
	classic bool // a classic diagram drawn by the scene: the session keeps the input and the selection
	node    *scene.Node
	screen  *scene.Screen // nil when the element has errors
	placed  *scene.Placed
	byNode  map[*scene.Node]*scene.Placed // the items written in the element that one can select, by what is written for them
	byPath  map[string]*scene.Placed      // every item one can select, by selection path
	pathOf  map[*scene.Placed]string
	inst    map[*scene.Placed]instRef // the items drawn from a component, which the use around overrides
	root    *gfx.Node                 // moved so that the content starts at the padding
	off     gfx.Point                 // gfx scene = content + off
	w, h    float64
	drag    sceneDrag

	// The classic preview, in gfx scene coordinates: the rubber band, the line of a link being drawn and
	// where the dragged boxes were when the press started.
	band     *gfx.Rect
	line     *[2]gfx.Point
	dragBase map[*scene.Placed]gfx.Point
	moved    bool // a drag put the drawn items somewhere the model has not written yet

	// what the gfx tree was built from; a cached view is reused only while all of it holds
	tree *scene.Tree
	ver  int
	next int // the first free node ID
	err  error
}

// sceneCacheSize is how many built element trees stay detached for a quick switch back.
const sceneCacheSize = 4

// cacheScene keeps sv, which was just taken off the canvas, if nothing changed since it was built.
func (v *Viewer) cacheScene(sv *sceneView) {
	v.dropStaleScenes()
	if sv == nil || sv.screen == nil || sv.err != nil || sv.moved {
		return
	}
	v.svCache = append([]*sceneView{sv}, v.svCache...)
	if len(v.svCache) > sceneCacheSize {
		v.svCache = v.svCache[:sceneCacheSize]
	}
}

// dropStaleScenes forgets the cached views of an older version of the document or of its imports.
func (v *Viewer) dropStaleScenes() {
	v.svCache = slices.DeleteFunc(v.svCache, func(sv *sceneView) bool {
		return sv.tree != v.stree || sv.ver != v.version
	})
}

// cachedScene takes the cached view of the node out of the cache.
func (v *Viewer) cachedScene(el *model.Element, node *scene.Node) *sceneView {
	v.dropStaleScenes()
	for i, sv := range v.svCache {
		if sv.el == el && sv.node == node {
			v.svCache = slices.Delete(v.svCache, i, i+1)
			return sv
		}
	}
	return nil
}

// instRef says where a part of an instance is overridden: the use that holds the instance and the at path
// from its root. At is empty when the part has no unique name.
type instRef struct {
	use *scene.Node
	at  string
}

// sceneDrag is a press on a selected item that moves it.
type sceneDrag struct {
	pl     *scene.Placed
	from   gfx.Point // scene point of the press
	ox, oy float64   // the place of the node at the press, in its parent
	moved  bool
}

// realizeScene puts the element on the canvas through go/scene, in place of what was there.
func (v *Viewer) realizeScene() {
	el := v.state.Current
	node := v.elementNode(el)
	oldSV := v.sv
	// Looked up before the old view is put away, so a redraw of the same element builds anew.
	sv := v.cachedScene(el, node)
	cached := sv != nil
	if !cached {
		sv = newSceneView(el, node)
		sv.classic = !v.isScene(el)
	}
	// A classic diagram stays where it is drawn across an edit: the camera and the map do not move
	// with the bounds, so the items that did not change do not jump.
	keep := !cached && sv.classic && oldSV != nil && v.shownElem == el && !v.pendingFit
	v.cacheScene(oldSV)
	err := sv.err
	v.scene.Update(func(tx *gfx.Tx) {
		if oldSV != nil {
			tx.Remove(oldSV.root)
		}
		if cached {
			sv.drag = sceneDrag{}
			tx.Add(nil, sv.root)
			v.nextID = sv.next
			return
		}
		err = v.drawSceneView(tx, sv)
		if keep && sv.root != nil {
			sv.off = oldSV.off
			tx.Move(sv.root, sv.off.X, sv.off.Y)
		}
	})
	v.sv = sv
	v.syncMap(sv)
	v.animStart(sv.screen)
	if !cached {
		v.sceneBuilds++
		if sv.placed != nil {
			sv.index(sv.placed)
		}
	}
	switch {
	case err != nil:
		v.setBanner(err.Error())
	default:
		v.setBanner(v.bannerText())
	}
	v.setTitle()
	v.dropStaleSelection()
	if v.shownElem != el || oldSV == nil {
		first := v.shownElem == nil // the first element of the document is the one to fit
		v.shownElem = el
		v.pendingFit = first
		v.placeCamera(first)
		v.syncOutline()
	} else if !v.pendingFit && oldSV != nil && !sv.classic {
		comp := v.view.Compositor()
		v.setCamera(Follow(comp.Camera(), gfx.Point{X: sv.off.X - oldSV.off.X, Y: sv.off.Y - oldSV.off.Y}))
	}
	v.drawSceneOverlay()
	v.rebuildItems()
	v.syncIndicators()
	v.moveInline()
}

// buildSceneView draws the element of sv into a fresh gfx root, as the canvas does.
func (v *Viewer) buildSceneView(sv *sceneView) error {
	var err error
	v.scene.Update(func(tx *gfx.Tx) { err = v.drawSceneView(tx, sv) })
	return err
}

// drawSceneView is buildSceneView inside the transaction of the caller: the root is added, the element
// laid out, and the screen kept for the selection and the animations.
func (v *Viewer) drawSceneView(tx *gfx.Tx, sv *sceneView) error {
	var err error
	sv.root = gfx.NewNode(1)
	tx.Add(nil, sv.root)
	if sv.node == nil {
		return nil
	}
	sv.screen, err = v.stree.BuildElement(tx, sv.root, sv.node, v.sceneOptions(sv.el))
	next := 2
	if sv.screen != nil {
		sv.placed, next = sv.screen.Placed(), sv.screen.NextID
	}
	sv.bounds()
	tx.Move(sv.root, sv.off.X, sv.off.Y)
	v.nextID = next + 1
	sv.tree, sv.ver, sv.next, sv.err = v.stree, v.version, v.nextID, err
	return err
}

// sceneOptions are what go/scene lays e out with: a classic diagram reads its values from the model.
func (v *Viewer) sceneOptions(e *model.Element) scene.Options {
	// The window keeps the shadows in screen points while the view zooms, as the classic renderer did.
	o := scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2, EvalVars: exec.SceneEvalVars, ScreenShadows: true}
	if !v.isScene(e) {
		// The evaluator is made when the first expression asks for it: most diagrams have none.
		var vals func(scene.ClassicRef, string) ([]string, bool)
		o.Classic = func(at scene.ClassicRef, prop string) ([]string, bool) {
			if vals == nil {
				vals = classic.Values(e, render.NewExecContext(e, render.Options{FontFamily: "Helvetica, Arial, sans-serif"}))
			}
			return vals(at, prop)
		}
	}
	return o
}

// bounds is the extent of the drawn items with the padding of the classic canvas around.
func (sv *sceneView) bounds() {
	pad := render.DefaultPadding
	minX, minY, maxX, maxY := 0.0, 0.0, 0.0, 0.0
	first := true
	var walk func(p *scene.Placed)
	walk = func(p *scene.Placed) {
		for _, k := range p.Kids {
			if first {
				minX, minY, maxX, maxY = k.X, k.Y, k.X+k.W, k.Y+k.H
				first = false
			}
			minX, minY = math.Min(minX, k.X), math.Min(minY, k.Y)
			maxX, maxY = math.Max(maxX, k.X+k.W), math.Max(maxY, k.Y+k.H)
			walk(k)
		}
	}
	if sv.placed != nil {
		walk(sv.placed)
	}
	sv.off = gfx.Point{X: pad - minX, Y: pad - minY}
	sv.w, sv.h = maxX-minX+2*pad, maxY-minY+2*pad
}

func newSceneView(el *model.Element, node *scene.Node) *sceneView {
	return &sceneView{el: el, node: node, byNode: map[*scene.Node]*scene.Placed{},
		byPath: map[string]*scene.Placed{}, pathOf: map[*scene.Placed]string{}, inst: map[*scene.Placed]instRef{}}
}

// index finds the items one can select: those written in the element, and inside an instance only what
// its use passes in, and the parts of the component, named from the use around (see add).
func (sv *sceneView) index(p *scene.Placed) {
	for _, k := range p.Kids {
		sv.add(k)
		sv.index(k)
	}
}

// add makes k selectable. A written item has the path NodePath gives. A part of a component has the
// path of the nearest written use around it, "/", and the names down to it (a repeat of a name among
// siblings gets #n); the parts are overridden by at in that use.
func (sv *sceneView) add(k *scene.Placed) {
	owner := nearestInst(k.Parent)
	if p := NodePath(sv.node, k.Src); p != "" && (owner == nil || sv.written(owner) && under(k.Src, owner.Src)) {
		if _, dup := sv.byNode[k.Src]; !dup {
			sv.byNode[k.Src] = k
			sv.byPath[p], sv.pathOf[k] = k, p
		}
		return
	}
	anchor := k.Parent
	for anchor != nil && !(anchor.Inst && sv.written(anchor)) {
		anchor = anchor.Parent
	}
	if anchor == nil {
		return
	}
	var segs, plain []string
	for c := k; c != anchor; c = c.Parent {
		plain = append(plain, c.Src.Key)
		if sl := c.Src.Parent; sl != nil && sl.Kind == scene.KSlot { // the fallback of a slot is reached through the slot
			plain = append(plain, slotSeg(sl))
		}
		segs = append(segs, nodeSegOf(c))
	}
	slices.Reverse(segs)
	slices.Reverse(plain)
	path := sv.pathOf[anchor] + "/" + strings.Join(segs, ".")
	ref := instRef{use: anchor.Src, at: strings.Join(plain, ".")}
	if strings.Contains(path, "#") || slices.Contains(plain, "") {
		ref.at = ""
	}
	sv.byPath[path], sv.pathOf[k], sv.inst[k] = k, path, ref
}

// instRows are the rows under an item drawn from an instance: what is drawn in it, written in the
// element or a part of the component.
func (sv *sceneView) instRows(pl *scene.Placed) []ItemRow {
	var out []ItemRow
	for _, k := range pl.Kids {
		p, ok := sv.pathOf[k]
		if !ok {
			continue
		}
		kids := len(k.Kids) > 0 || sv.written(k) && len(sceneKids(k.Src)) > 0
		out = append(out, ItemRow{Label: nodeLabel(k.Src), Kind: k.Src.Kind.String(), Path: p, Kids: kids})
	}
	return out
}

// rowChain are the paths of the rows from the top down to the one of path p, as the tree nests them.
func (sv *sceneView) rowChain(p string) []string {
	var chain []string
	for ; p != ""; p = sv.rowParent(p) {
		chain = append(chain, p)
	}
	slices.Reverse(chain)
	return chain
}

// opensParts reports whether the row of p lists the drawn kids of an instance or of one of its parts; the
// other rows list what is written under them (see rowsAt).
func (sv *sceneView) opensParts(p string) bool {
	pl := sv.byPath[p]
	return pl != nil && (pl.Inst || strings.Contains(p, "/"))
}

// rowParent is the path of the row that holds the row of p, "" for a top row. Under an instance a row
// sits under the nearest drawn item above it, so what a use passes in sits under the part of the
// component it is drawn in, not under the use; elsewhere it nests as it is written.
func (sv *sceneView) rowParent(p string) string {
	if pl := sv.byPath[p]; pl != nil {
		for a := pl.Parent; a != nil; a = a.Parent {
			ap, ok := sv.pathOf[a]
			if !ok {
				continue
			}
			if strings.Contains(p, "/") || sv.opensParts(ap) {
				return ap
			}
			break
		}
	}
	if i := strings.LastIndex(p, "."); i >= 0 && !strings.Contains(p, "/") {
		return p[:i]
	}
	return ""
}

func slotSeg(sl *scene.Node) string {
	if sl.Key == "" {
		return "default"
	}
	return sl.Key
}

// nodeSegOf is the key of an item among the drawn siblings, with #n for the n-th repeat.
func nodeSegOf(p *scene.Placed) string {
	k := 0
	if p.Parent != nil {
		for _, c := range p.Parent.Kids {
			if c == p {
				break
			}
			if c.Src.Key == p.Src.Key {
				k++
			}
		}
	}
	if k > 0 {
		return p.Src.Key + "#" + strconv.Itoa(k)
	}
	return p.Src.Key
}

func nearestInst(p *scene.Placed) *scene.Placed {
	for ; p != nil; p = p.Parent {
		if p.Inst {
			return p
		}
	}
	return nil
}

// written reports whether p is an item written in the element (a path without "/").
func (sv *sceneView) written(p *scene.Placed) bool {
	path, ok := sv.pathOf[p]
	return ok && !strings.Contains(path, "/")
}

func under(n, owner *scene.Node) bool {
	for c := n.Parent; c != nil; c = c.Parent {
		if c == owner {
			return true
		}
	}
	return false
}

// hitScene is the deepest item one can select under the content point p, nil on empty space.
func (sv *sceneView) hitScene(p gfx.Point) *scene.Placed {
	var found *scene.Placed
	var walk func(pl *scene.Placed) bool
	walk = func(pl *scene.Placed) bool {
		for i := len(pl.Kids) - 1; i >= 0; i-- {
			k := pl.Kids[i]
			if p.X < k.X || p.Y < k.Y || p.X > k.X+k.W || p.Y > k.Y+k.H {
				continue
			}
			if _, ok := sv.pathOf[k]; ok {
				found = k
			}
			walk(k)
			return true
		}
		return false
	}
	if sv.placed != nil {
		walk(sv.placed)
	}
	return found
}

// pick is what a click on hit selects. An instance is selected whole first: the outermost one above hit.
// A double click goes one level deeper toward hit; a single click never enters an instance, so a component
// is not taken for the text or the icon it is made of.
func (sv *sceneView) pick(hit, cur *scene.Placed, clicks int) *scene.Placed {
	var chain []*scene.Placed // from the top down to hit
	for c := hit; c != nil; c = c.Parent {
		if _, ok := sv.pathOf[c]; ok {
			chain = append(chain, c)
		}
	}
	slices.Reverse(chain)
	top := slices.IndexFunc(chain, func(c *scene.Placed) bool { return c.Inst })
	if top < 0 {
		return hit
	}
	at := slices.Index(chain, cur)
	if clicks >= 2 {
		if at >= 0 && at+1 < len(chain) {
			return chain[at+1]
		}
		if _, ok := sv.pathOf[hit]; ok {
			return hit
		}
	}
	return chain[top]
}

// isInside reports whether n is below ancestor in the placed tree.
func isInside(n, ancestor *scene.Placed) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// sceneDrawn reports whether the canvas draws the element through go/scene: a scene, or a classic
// diagram whose picture the scene draws while the session keeps the input.
func (v *Viewer) sceneDrawn() bool { return v.sv != nil }

// mapOf is the map of a picture the scene drew: the model coordinates of the session go through the
// same offset as the drawn nodes, so a click and the frame it selects cannot drift apart.
func (sv *sceneView) mapOf() SceneMap { return SceneMap{TX: sv.off.X, TY: sv.off.Y} }

// syncMap makes the model coordinates of the session follow the picture of a classic diagram: the map
// is the offset the scene drew it with, not the bounds the old renderer would have given it.
func (v *Viewer) syncMap(sv *sceneView) bool {
	if !sv.classic || v.session == nil {
		return false
	}
	v.smap = sv.mapOf()
	v.selNodes = nil // the paths of the selected scene nodes name nothing here
	return true
}

// drawnRect is where the canvas draws a classic item of the element on it; false when the scene does
// not draw that item, a hidden one or a link.
func (v *Viewer) drawnRect(it *model.DiagramItem) (gfx.Rect, bool) {
	pl := v.classicPlaced(it)
	if pl == nil {
		return gfx.Rect{}, false
	}
	return gfx.R(pl.X+v.sv.off.X, pl.Y+v.sv.off.Y, pl.W, pl.H), true
}

// drawnCovers is editor.Options.Covers of a classic diagram: the session's boxes come from go/render and
// the picture from go/scene, so a press where nothing is drawn must not grab the item.
func (v *Viewer) drawnCovers(r render.Rect, it *model.DiagramItem) bool {
	if v.sv == nil || !v.sv.classic {
		return true
	}
	b, ok := v.drawnRect(it)
	if !ok {
		return true // nothing drawn for it: a link or a hidden item, the picture has no say
	}
	return b.Intersects(v.smap.RectToGfx(r))
}

// sceneMode reports whether the canvas shows a scene element: the pointer, the keys and the edits are
// the scene's. A classic diagram drawn by the scene stays out: it has no scene input yet.
func (v *Viewer) sceneMode() bool { return v.sv != nil && !v.sv.classic }

// canvasSelection is what the scene picture outlines: the selected nodes of a scene, the selected items
// of a classic diagram drawn by the scene.
func (v *Viewer) canvasSelection() []string {
	if v.sceneMode() {
		return v.selNodes
	}
	sel := map[*model.DiagramItem]bool{}
	for _, it := range v.sessionItems() {
		sel[it] = true
	}
	var out []string
	for _, pl := range v.sv.byPath {
		if pl.Src != nil && sel[classicItemAt(v.state.Current, pl.Src)] {
			out = append(out, v.sv.pathOf[pl])
		}
	}
	// The item under the pointer while a link is drawn is outlined too, so the drop target is visible.
	if v.session != nil {
		if t := v.session.LineTarget(); t != nil {
			if pl := v.classicPlaced(t); pl != nil {
				out = append(out, v.sv.pathOf[pl])
			}
		}
	}
	return out
}

// previewClassic draws what the session previews in a classic diagram: the dragged items where the drop
// will write them, the rubber band, and the line of a link being drawn.
func (v *Viewer) previewClassic() {
	sv := v.sv
	sv.band, sv.line = nil, nil
	if v.session == nil {
		return
	}
	if b := v.session.Band(); b != nil {
		r := v.smap.RectToGfx(*b)
		sv.band = &r
		return
	}
	if ln := v.session.Scene().LineTo; ln != nil {
		a := v.smap.ToGfx(render.Point{X: ln.Source.X, Y: ln.Source.Y})
		b := v.smap.ToGfx(render.Point{X: ln.Target.X, Y: ln.Target.Y})
		sv.line = &[2]gfx.Point{a, b}
		return
	}
	if v.dragAt == v.press.scene {
		return
	}
	v.scene.Update(func(tx *gfx.Tx) { v.dragPreview(v.press.scene, v.dragAt, tx) })
}

// dragPreview moves the drawn items of a classic diagram by the pointer from to to, both in session
// coordinates: every item lands where the drop rounds its position, so the picture under the pointer is
// where the commit writes it.
func (v *Viewer) dragPreview(from, to render.Point, tx *gfx.Tx) {
	sv := v.sv
	if sv.dragBase == nil {
		// The drawn place of an item moves with the pointer, so the start is kept once per press.
		sv.dragBase = map[*scene.Placed]gfx.Point{}
		for _, it := range v.sessionItems() {
			if pl := v.classicPlaced(it); pl != nil {
				sv.dragBase[pl] = gfx.Point{X: pl.X, Y: pl.Y}
			}
		}
	}
	dx, dy := to.X-from.X, to.Y-from.Y
	for _, it := range v.sessionItems() {
		if it.Kind != model.ItemKindItem {
			continue
		}
		pl := v.classicPlaced(it)
		if pl == nil || pl.Gfx == nil {
			continue
		}
		base := sv.dragBase[pl]
		x, y := dragRound(it.X+dx), dragRound(it.Y+dy)
		if sv.screen.MoveItem(tx, pl, base.X+x-it.X, base.Y-(y-it.Y)) {
			sv.moved = true
		}
	}
}

// dragRound is the editor's roundCoord: the preview must land on the position the drop commits.
func dragRound(v float64) float64 { return math.Round(v*100) / 100 }

// classicPlaced is where the scene draws the classic item it of the element on the canvas.
func (v *Viewer) classicPlaced(it *model.DiagramItem) *scene.Placed {
	if v.sv == nil || v.state.Current == nil {
		return nil
	}
	for _, pl := range v.sv.byPath {
		if pl.Src != nil && classicItemAt(v.state.Current, pl.Src) == it {
			return pl
		}
	}
	return nil
}

// classicItemAt is the model item of a drawn classic item: both keep file order, so the n-th drawn item
// of its parent is the n-th item of the element, a mounted element's file included.
func classicItemAt(e *model.Element, n *scene.Node) *model.DiagramItem {
	if e == nil || n == nil || n.Parent == nil {
		return nil
	}
	k := 0
	for _, c := range n.Parent.Content {
		if c == n {
			break
		}
		if c.Kind == scene.KItem {
			k++
		}
	}
	for _, it := range e.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		if k == 0 {
			return it
		}
		k--
	}
	return nil
}

// classicNamed is where the scene draws the classic item with the model name name, the way a comment or a
// mark names an item.
func (v *Viewer) classicNamed(name string) *scene.Placed {
	if v.state.Current == nil {
		return nil
	}
	for _, it := range v.state.Current.Items {
		if it.Name == name && !it.IsLink() {
			return v.classicPlaced(it)
		}
	}
	return nil
}

// selectedScene is the first selected scene node and where it is drawn (nil when not drawn). A part of
// a component has no node of its own: nil and where it is drawn, see selectedInst.
func (v *Viewer) selectedScene() (*scene.Node, *scene.Placed) {
	if !v.sceneMode() || len(v.selNodes) == 0 {
		return nil, nil
	}
	if _, ok := v.selectedInst(); ok {
		return nil, v.sv.byPath[v.selNodes[0]]
	}
	n := NodeAt(v.sv.node, v.selNodes[0])
	if n == nil {
		return nil, nil
	}
	return n, v.sv.byNode[n]
}

// selectedInst is the first selected node when it is a part of a component.
func (v *Viewer) selectedInst() (instRef, bool) {
	if !v.sceneMode() || len(v.selNodes) == 0 {
		return instRef{}, false
	}
	pl := v.sv.byPath[v.selNodes[0]]
	ref, ok := v.sv.inst[pl]
	return ref, pl != nil && ok
}

// shownNode is the node the inspector describes: the selected one, for a part of a component the node it
// is drawn from.
func (v *Viewer) shownNode() *scene.Node {
	n, pl := v.selectedScene()
	if n == nil && pl != nil {
		return pl.Src
	}
	return n
}

// goToComponent shows the element that defines the component the selected use takes: the library is a
// mount of this document, so the window moves to it instead of opening the file on its own.
func (v *Viewer) goToComponent() { v.goToComponentNode(v.shownNode()) }

// goToComponentNode is goToComponent for a node taken from a row of the Items tree, which the canvas
// selection need not follow.
func (v *Viewer) goToComponentNode(n *scene.Node) {
	if n == nil || n.Kind != scene.KUse || v.stree == nil {
		return
	}
	comp, err := v.stree.Resolve(n.Parent, n.Path)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	if comp == nil || comp.Pos.File == "" {
		v.setBanner("the component " + n.Path + " has no file of its own")
		return
	}
	if !v.showMount(comp.Pos.File) {
		v.setBanner("the component is in " + comp.Pos.File + ", which this document does not mount")
		return
	}
	v.selectComponent(comp)
}

// selectComponent selects the node a component is defined in, once the window shows the element that
// mounts its file, and scrolls it into view.
func (v *Viewer) selectComponent(comp *scene.Node) {
	if v.sv == nil || comp == nil {
		return
	}
	for path, pl := range v.sv.byPath {
		if pl.Src == comp {
			v.selectPaths([]string{path}, true)
			return
		}
	}
}

// editingPath is the path of the item whose text is being edited, so its frame turns green as Swift's
// SelectorBox does.
func (v *Viewer) editingPath() string {
	if v.session == nil || v.sv == nil {
		return ""
	}
	it := v.session.Editing()
	if it == nil {
		return ""
	}
	for p, pl := range v.sv.byPath {
		if pl.Src != nil && classicItemAt(v.state.Current, pl.Src) == it {
			return p
		}
	}
	return ""
}

// dropStaleSelection forgets the selected paths that name nothing after a change.
func (v *Viewer) dropStaleSelection() {
	kept := v.selNodes[:0:0]
	for _, p := range v.selNodes {
		if NodeAt(v.sv.node, p) != nil || v.sv.byPath[p] != nil {
			kept = append(kept, p)
		}
	}
	v.selNodes = kept
}

// The selector frame of the Swift app (SelectorBox): the box of the item grown by 5 points, rounded by 9,
// stroked by a dash of 5 on and 5 off with a glow of its own colour. Blue, or green over the item that is
// being edited.
const (
	selectorGrow  = 5
	selectorRound = 9
	selectorBlur  = 5
)

var (
	selectorBlue  = gfx.RGB(0, 0, 255)
	selectorGreen = gfx.RGB(0, 255, 0)
)

// selectorPath is the rounded rectangle of Swift's SelectorBox, its arcs the addArc of the corners.
func selectorPath(r gfx.Rect, radius float64) *gfx.Path {
	p := &gfx.Path{}
	p.MoveTo((r.Min.X+r.Max.X)/2, r.Min.Y)
	p.ArcTo(r.Max.X, r.Min.Y, r.Max.X, r.Max.Y, radius)
	p.ArcTo(r.Max.X, r.Max.Y, r.Min.X, r.Max.Y, radius)
	p.ArcTo(r.Min.X, r.Max.Y, r.Min.X, r.Min.Y, radius)
	p.ArcTo(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y, radius)
	p.Close()
	return p
}

// selectorStroke is the dashed glow of the selector frame, the one of Swift's SelectorBox.draw.
func selectorStroke(r *gfx.Recorder, path *gfx.Path, color gfx.Color) {
	r.StrokeShadowPath(path, gfx.StrokeStyle{Width: 1, MiterLimit: 10, Dash: []float64{5, 5}, DashPhase: 5},
		color, gfx.Shadow{Blur: selectorBlur, Color: color, Screen: true})
}

// selectionPicture records the frames of the selected paths in the coordinates of the gfx scene, so a
// pan or a zoom moves them with the content instead of leaving them a frame behind. green is the path of
// the item being edited, if any. The classic preview - the rubber band, the line of a link being drawn -
// comes after them, in the same coordinates.
func (sv *sceneView) selectionPicture(sel []string, green string) *gfx.Picture {
	r := gfx.NewRecorder()
	for _, p := range sel {
		pl := sv.byPath[p]
		if pl == nil {
			continue
		}
		c := selectorBlue
		if p == green {
			c = selectorGreen
		}
		box := gfx.R(pl.X+sv.off.X-selectorGrow, pl.Y+sv.off.Y-selectorGrow,
			pl.W+2*selectorGrow, pl.H+2*selectorGrow)
		selectorStroke(r, selectorPath(box, selectorRound), c)
	}
	if sv.band != nil {
		selectorStroke(r, selectorPath(*sv.band, 0), selectorBlue)
	}
	if sv.line != nil {
		p := &gfx.Path{}
		p.MoveTo(sv.line[0].X, sv.line[0].Y)
		p.LineTo(sv.line[1].X, sv.line[1].Y)
		r.StrokeShadowPath(p, gfx.StrokeStyle{Width: 1, MiterLimit: 10}, selectorBlue,
			gfx.Shadow{Blur: selectorBlur, Color: selectorBlue, Screen: true})
	}
	return r.Finish()
}

// setCamera moves the camera; the selection frames are part of the scene and follow it.
func (v *Viewer) setCamera(cam gfx.Camera) {
	v.view.Compositor().SetCamera(cam)
	v.drawSceneOverlay()
}

// panView pans the canvas; the selection frames are part of the scene and follow it.
func (v *Viewer) panView(dx, dy float64) {
	v.view.Pan(dx, dy)
	v.drawSceneOverlay()
}

// onScroll pans the canvas sideways with Shift+wheel. GTK reports the wheel on one axis only, so
// without this the canvas moves up and down but never sideways.
func (v *Viewer) onScroll(s *view.Scroll) bool {
	if !s.Mods.Shift || s.DX != 0 {
		return false
	}
	dx := s.DY
	if !s.Precise {
		dx *= wheelLine
	}
	v.panView(dx, 0)
	return true
}

// drawSceneOverlay draws the selection frames on the HUD, so a zoom or a pan does not damage the
// tiles of the scene.
func (v *Viewer) drawSceneOverlay() {
	if v.view == nil {
		return
	}
	v.syncMarks() // the marks sit on the nodes, so they move with the camera
	pic := gfx.NewRecorder().Finish()
	if v.sceneDrawn() {
		pic = v.sv.selectionPicture(v.canvasSelection(), v.editingPath())
	}
	if v.selHUD != nil || pic.Len() > 0 {
		v.scene.Update(func(tx *gfx.Tx) {
			if v.selHUD == nil {
				v.selHUD = gfx.NewNode(3)
				tx.Add(nil, v.selHUD)
				tx.SetShape(v.selHUD, nil, gfx.HitNone) // hit-testing of the selection is done on the scene
				tx.SetZ(v.selHUD, overlayZ)
			}
			tx.SetPicture(v.selHUD, pic)
		})
	}
	v.syncCompareHUD()
}

// selectScene makes paths the selection of the scene element on the canvas.
func (v *Viewer) selectScene(paths []string) {
	v.selNodes = paths
	if v.state.Current == v.docElem {
		v.docSel = paths
	}
	v.drawSceneOverlay()
	v.refreshProps()
	v.syncItems()
	v.updateMenus()
	v.sh.publish(DocEvent{Path: v.state.Doc.Path, Kind: EventSelect, Version: v.version})
}

// revealScene centers the canvas on the first selected node when it is out of view.
func (v *Viewer) revealScene() {
	_, pl := v.selectedScene()
	if pl == nil {
		return
	}
	w, h := v.view.Canvas().Size()
	cam := v.view.Compositor().Camera()
	c := gfx.Point{X: pl.X + pl.W/2 + v.sv.off.X, Y: pl.Y + pl.H/2 + v.sv.off.Y}
	vx, vy := (c.X-cam.Origin.X)*cam.Zoom, (c.Y-cam.Origin.Y)*cam.Zoom
	if vx < 0 || vy < 0 || vx > float64(w) || vy > float64(h) {
		v.setCamera(CenterOn(cam, c, float64(w), float64(h)))
	}
}

// scenePointer selects with a click, moves a selected free item with a drag and pans elsewhere.
func (v *Viewer) scenePointer(p *view.Pointer) {
	sv := v.sv
	at := gfx.Point{X: p.Scene.X - sv.off.X, Y: p.Scene.Y - sv.off.Y}
	switch p.Kind {
	case view.PointerDown:
		if p.Button != 1 {
			return
		}
		// A link of a body is followed by a plain click; a modified one selects and pans as before.
		if p.Mods == (g.Mods{}) {
			if l, ok := sv.bodyLinkAt(at); ok {
				v.openBodyLink(l, at)
				return
			}
		}
		v.press = press{active: true, cam: v.view.Compositor().Camera(), pos: p.Pos, from: p.Pos}
		_, cur := v.selectedScene()
		hit := sv.hitScene(at)
		// A press inside the selected item moves it, even where a child covers all of it: otherwise an
		// item made of one instance could not be dragged at all. A second click goes into the child.
		if hit != nil && cur != nil && hit != cur && cur.Free && cur.Gfx != nil && p.Count < 2 && isInside(hit, cur) {
			hit = cur
		}
		if hit != nil {
			hit = sv.pick(hit, cur, p.Count)
		}
		switch {
		case hit != nil && hit == cur && hit.Free && hit.Gfx != nil:
			t := hit.Gfx.Transform()
			sv.drag = sceneDrag{pl: hit, from: at, ox: t[4], oy: t[5]}
		case hit != nil:
			v.selectScene([]string{sv.pathOf[hit]})
			v.press.pan = true
		default:
			if len(v.selNodes) > 0 {
				v.selectScene(nil)
			}
			v.press.pan = true
		}
		if v.press.pan {
			v.view.SetCursor(view.CursorGrabbing)
		}
	case view.PointerMove:
		if !v.press.active || !p.Drag {
			// A hover: the hand says the text under it is a link, and the arrow says it is not.
			if _, ok := sv.bodyLinkAt(at); ok {
				v.view.SetCursor(view.CursorHand)
			} else {
				v.view.SetCursor(view.CursorArrow)
			}
			return
		}
		if v.press.pan {
			v.panView(p.Pos.X-v.press.pos.X, p.Pos.Y-v.press.pos.Y)
			v.press.pos = p.Pos
			return
		}
		if d := &sv.drag; d.pl != nil {
			dx, dy := at.X-d.from.X, at.Y-d.from.Y
			d.moved = d.moved || math.Abs(dx)+math.Abs(dy) >= 1
			pl := d.pl
			v.scene.Update(func(tx *gfx.Tx) {
				if sv.screen.MoveItem(tx, pl, d.ox+dx, d.oy+dy) {
					sv.moved = true
				}
			})
			v.drawSceneOverlay()
		}
	case view.PointerUp, view.PointerCancel:
		if !v.press.active {
			return
		}
		v.press = press{}
		v.view.SetCursor(view.CursorArrow)
		d := sv.drag
		sv.drag = sceneDrag{}
		if d.pl == nil || !d.moved {
			return
		}
		if p.Kind == view.PointerCancel {
			v.realize()
			return
		}
		v.moveSceneNode(d.pl, at.X-d.from.X, at.Y-d.from.Y)
	}
}

// sceneRightClick selects the node under a right click when it is not selected, so the menu acts on it.
func (v *Viewer) sceneRightClick(at gfx.Point) {
	if !v.sceneMode() {
		return
	}
	hit := v.sv.hitScene(gfx.Point{X: at.X - v.sv.off.X, Y: at.Y - v.sv.off.Y})
	if hit == nil {
		return
	}
	_, cur := v.selectedScene()
	p := v.sv.pathOf[v.sv.pick(hit, cur, 1)]
	if !slices.Contains(v.selNodes, p) {
		v.selectScene([]string{p})
	}
}

// moveSceneNode writes the new pos of an item dragged by dx, dy (pos is y up, the canvas y down).
func (v *Viewer) moveSceneNode(pl *scene.Placed, dx, dy float64) {
	x, y := 0.0, 0.0
	if pr := pl.Prop("pos"); pr != nil {
		vals := pr.Values()
		if len(vals) == 2 {
			x, y = parseNum(vals[0]), parseNum(vals[1])
		}
	}
	nx, ny := math.Round(x+dx), math.Round(y-dy)
	if ref, ok := v.sv.inst[pl]; ok {
		v.setInstProp(ref, "pos", numText(nx), numText(ny))
		return
	}
	if e, ok := SetPropEdit(v.fileText(v.ownFileOf(pl.Src)), pl.Src, "pos", numText(nx), numText(ny)); ok {
		v.sceneEditIn(v.ownFileOf(pl.Src), pl.Src, e)
	}
}

// setInstProp writes a property of a part of a component as an at override in the use around it; the
// component and the other instances stay as they are.
func (v *Viewer) setInstProp(ref instRef, name string, args ...string) bool {
	if ref.at == "" {
		v.setBanner("Not applied: the item has no unique name inside the instance, edit it in the use")
		return false
	}
	e, ok := AtPropEdit(v.fileText(v.ownFileOf(ref.use)), ref.use, ref.at, name, args...)
	return ok && v.sceneEditIn(v.ownFileOf(ref.use), ref.use, e)
}

// sceneEdit applies a text edit of the file of n, made by the window; a refused one shows in the banner.
func (v *Viewer) sceneEdit(n *scene.Node, e TextEdit) bool { return v.sceneEditIn(v.fileOf(n), n, e) }

// sceneEditIn applies an edit of file, about n: the file of a node's own block may differ from the file
// of what it shows (an element with --from-file).
func (v *Viewer) sceneEditIn(file string, n *scene.Node, e TextEdit) bool {
	text := v.fileText(file)
	if v.editAddsErrors(file, text, e) {
		v.setBanner("Not applied: the change adds scene errors")
		return false
	}
	var path string
	if v.sv != nil {
		path = NodePath(v.sv.node, n) // before the apply: it rebuilds the tree of the element
	}
	if err := v.applyFileText(file, e.Start, e.End, e.New); err != nil {
		v.pending = nil
		v.setBanner(err.Error())
		return false
	}
	v.markNode(path, markPulse, "")
	return true
}

// sceneKey: Delete removes the selected node, Escape clears the selection.
func (v *Viewer) sceneKey(name string) {
	switch name {
	case "Delete", "Backspace":
		v.deleteSceneNode()
	case "Escape":
		v.selectScene(nil)
	}
}

func (v *Viewer) deleteSceneNode() {
	n, _ := v.selectedScene()
	if n == nil {
		return
	}
	v.selNodes = nil
	v.sceneEditIn(v.ownFileOf(n), n, DeleteEdit(v.fileText(v.ownFileOf(n)), n))
}

// freeSpot is a place right of the content of the element for a new top level node, as pos x y.
func (v *Viewer) freeSpot() (float64, float64) {
	if v.sv == nil || v.sv.placed == nil || len(v.sv.placed.Kids) == 0 {
		return 0, 0
	}
	maxX, minY := math.Inf(-1), math.Inf(1)
	for _, k := range v.sv.placed.Kids {
		maxX, minY = math.Max(maxX, k.X+k.W), math.Min(minY, k.Y)
	}
	return math.Round(maxX + topGap), math.Round(-minY)
}

// insertComponent adds `use <path> as <name>` where InsertTarget says and selects it; for an instance
// with named slots the slot is asked for in a menu.
func (v *Viewer) insertComponent(path, name string) {
	if !v.ensureSceneElement() || v.sv.node == nil {
		return
	}
	sel, _ := v.selectedScene()
	into, slots := InsertTarget(v.stree, sel)
	if into == nil || len(slots) == 0 {
		v.insertUse(into, "", path, name)
		return
	}
	var entries []editor.Entry
	for _, s := range slots {
		label := "Into slot " + s
		if s == "" {
			label = "Into the default slot"
		}
		entries = append(entries, editor.Entry{Label: label, Run: func() { v.insertUse(into, s, path, name) }})
	}
	entries = append(entries, editor.Entry{Sep: true}, editor.Entry{Label: "On the canvas", Run: func() { v.insertUse(nil, "", path, name) }})
	v.app.Async(func() {
		v.menuOverride = entries
		v.canvasMenu.Show()
	})
}

// insertUse writes the use into the item or instance into, in its slot; into nil puts it on the
// canvas at a free spot.
func (v *Viewer) insertUse(into *scene.Node, slot, path, name string) {
	target := into
	if target == nil {
		target = v.sv.node
	}
	name = FreeName(target, name)
	use := CommandText("use", path, "as", name)
	cmd := SlotCommand(slot, use)
	if into == nil {
		x, y := v.freeSpot()
		use += " { " + CommandText("pos", numText(x), numText(y)) + " }"
		cmd = use
	}
	v.insertAndSelect(target, cmd, use)
}

// ScreenSizes are the frames New Screen offers.
var ScreenSizes = []struct {
	Name string
	W, H int
}{{"Desktop", 1440, 900}, {"Laptop", 1280, 800}, {"Tablet", 834, 1194}, {"Phone", 390, 844}}

// newScreen adds an empty frame of w x h right of the content of the element.
func (v *Viewer) newScreen(w, h int) {
	if !v.ensureSceneElement() {
		return
	}
	target := v.sv.node
	if target == nil {
		return
	}
	name := FreeName(target, "Screen")
	x, y := v.freeSpot()
	head := CommandText("item", name)
	cmd := head + " { " + strings.Join([]string{
		CommandText("pos", numText(x), numText(y)), CommandText("width", numText(float64(w))),
		CommandText("height", numText(float64(h))), "auto-layout column", "color #ffffff"}, "; ") + " }"
	v.selNodes = nil
	v.insertAndSelect(target, cmd, head)
}

// insertAndSelect writes cmd at the end of target and selects the node that starts with head in it.
func (v *Viewer) insertAndSelect(target *scene.Node, cmd, head string) {
	owner := target
	if target.Mount != nil { // the content of an element with --from-file is written in that file
		if f := v.stree.File(v.fileOf(target)); f != nil {
			owner = f
		}
	}
	e := InsertEdit(v.fileText(v.fileOf(owner)), owner, cmd)
	at := strings.Index(e.New, head)
	if at < 0 || !v.sceneEdit(owner, e) {
		return
	}
	if n := nodeStartingAt(v.sv.node, e.Start+at); n != nil {
		v.selectScene([]string{NodePath(v.sv.node, n)})
		v.revealNode()
	}
}

// duplicateScene is Cmd+D on the selected scene nodes: a copy of each beside it in its parent, as one
// undo step, the copies selected.
func (v *Viewer) duplicateScene() {
	if !v.sceneMode() || v.sv.node == nil {
		return
	}
	var srcs []DupSource
	for _, p := range v.selNodes {
		n := NodeAt(v.sv.node, p)
		if n == nil {
			continue
		}
		s := DupSource{Node: n}
		if pl := v.sv.byNode[n]; pl != nil {
			s.At, s.Free = &Box{pl.X, pl.Y, pl.W, pl.H}, pl.Free
			if pr := pl.Prop("pos"); pr != nil {
				if vals := pr.Values(); len(vals) == 2 {
					s.Pos = [2]float64{parseNum(vals[0]), parseNum(vals[1])}
				}
			}
			if pl.Parent != nil {
				for _, k := range pl.Parent.Kids {
					s.Others = append(s.Others, Box{k.X, k.Y, k.W, k.H})
				}
			}
		}
		srcs = append(srcs, s)
	}
	e, starts, ok := DuplicateEdit(v.fileText(v.fileOf(v.sv.node)), srcs)
	if !ok || !v.sceneEdit(v.sv.node, e) {
		return
	}
	var paths []string
	for _, off := range starts {
		if n := nodeStartingAt(v.sv.node, off); n != nil {
			paths = append(paths, NodePath(v.sv.node, n))
		}
	}
	v.selectScene(paths)
	v.revealNode()
}

// ensureSceneElement puts a classic element that is empty on the scene renderer; content of a classic
// element with items stays classic and refuses scene commands.
func (v *Viewer) ensureSceneElement() bool {
	if v.sceneMode() {
		return true
	}
	e := v.state.Current
	if e == nil {
		v.setBanner("Choose an element first: a screen goes into a scene element.")
		return false
	}
	for _, it := range e.Items {
		if it.IsLink() {
			v.setBanner("Screens and components go into a scene element, and this one holds links, which a scene does not draw: use an empty element or a new screen.")
			return false
		}
	}
	if v.stree == nil {
		v.kind = model.DocScene
		v.loadScene()
	}
	if v.stree == nil {
		return false
	}
	v.realizeScene()
	return v.sv != nil && v.sv.node != nil
}

func parseNum(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// zoomToSelection fits the selected scene node into the canvas.
func (v *Viewer) zoomToSelection() {
	_, pl := v.selectedScene()
	if pl == nil || pl.W <= 0 || pl.H <= 0 {
		return
	}
	w, h := v.view.Canvas().Size()
	z := ClampZoom(0.9 * math.Min(float64(w)/pl.W, float64(h)/pl.H))
	c := gfx.Point{X: pl.X + pl.W/2 + v.sv.off.X, Y: pl.Y + pl.H/2 + v.sv.off.Y}
	v.setCamera(CenterOn(gfx.Camera{Zoom: z}, c, float64(w), float64(h)))
	v.drawSceneOverlay()
}
