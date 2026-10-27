//go:build darwin || linux || windows

package app

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// sceneView is a scene element on the canvas, drawn by go/scene.
type sceneView struct {
	el      *model.Element
	node    *scene.Node
	screen  *scene.Screen // nil when the element has errors
	placed  *scene.Placed
	byNode  map[*scene.Node]*scene.Placed // the items one can select, by what is written for them
	root    *gfx.Node                     // moved so that the content starts at the padding
	overlay *gfx.Node
	off     gfx.Point // gfx scene = content + off
	w, h    float64
	drag    sceneDrag
}

// sceneDrag is a press on a selected item that moves it.
type sceneDrag struct {
	pl     *scene.Placed
	from   gfx.Point // scene point of the press
	ox, oy float64   // the place of the node at the press, in its parent
	moved  bool
}

// accent is the colour of the selection frame.
var accent = gfx.RGBA(0, 122, 255, 1)

// realizeScene puts the element on the canvas through go/scene, in place of what was there.
func (v *Viewer) realizeScene() {
	el := v.state.Current
	node := v.elementNode(el)
	if v.lib != nil {
		if n := v.libNode(); n != nil {
			node = n
		} else {
			v.leaveLib()
		}
	}
	oldSV, oldRendered := v.sv, v.rendered
	sv := &sceneView{el: el, node: node, byNode: map[*scene.Node]*scene.Placed{}}
	var err error
	v.scene.Update(func(tx *gfx.Tx) {
		if oldRendered != nil {
			tx.Remove(oldRendered.Root)
		}
		if oldSV != nil {
			tx.Remove(oldSV.root)
		}
		sv.root = gfx.NewNode(1)
		tx.Add(nil, sv.root)
		if node == nil {
			return
		}
		sv.screen, err = v.stree.BuildElement(tx, sv.root, node, scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2})
		next := 2
		if sv.screen != nil {
			sv.placed = sv.screen.Placed()
			next = sv.screen.NextID
		}
		sv.bounds()
		tx.Move(sv.root, sv.off.X, sv.off.Y)
		sv.overlay = gfx.NewNode(next)
		tx.Add(sv.root, sv.overlay)
		tx.SetZ(sv.overlay, overlayZ)
		v.nextID = next + 1
	})
	v.rendered, v.overlay, v.sv = nil, nil, sv
	v.sceneBuilds++
	if sv.placed != nil {
		sv.index(sv.placed, nil)
	}
	switch {
	case err != nil:
		v.setBanner(err.Error())
	default:
		v.setBanner(v.bannerText())
	}
	v.dropStaleSelection()
	if v.shownElem != el || oldSV == nil && oldRendered == nil {
		v.shownElem = el
		v.pendingFit = true
		v.applyInitialCamera()
		v.syncOutline()
	} else if !v.pendingFit && oldSV != nil {
		comp := v.view.Compositor()
		comp.SetCamera(Follow(comp.Camera(), gfx.Point{X: sv.off.X - oldSV.off.X, Y: sv.off.Y - oldSV.off.Y}))
	}
	v.drawSceneOverlay()
	v.rebuildItems()
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

// index finds the items one can select: those written in the element, and inside an instance only what
// its use passes in. owner is the use of the instance around, nil outside one.
func (sv *sceneView) index(p *scene.Placed, owner *scene.Node) {
	for _, k := range p.Kids {
		if NodePath(sv.node, k.Src) != "" && (owner == nil || under(k.Src, owner)) {
			if _, dup := sv.byNode[k.Src]; !dup {
				sv.byNode[k.Src] = k
			}
		}
		o := owner
		if k.Inst {
			o = k.Src
		}
		sv.index(k, o)
	}
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
			if _, ok := sv.byNode[k.Src]; ok && sv.byNode[k.Src] == k {
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

// sceneMode reports whether the canvas shows a scene element.
func (v *Viewer) sceneMode() bool { return v.sv != nil && v.rendered == nil }

// selectedScene is the first selected scene node and where it is drawn (nil when not drawn).
func (v *Viewer) selectedScene() (*scene.Node, *scene.Placed) {
	if !v.sceneMode() || len(v.selNodes) == 0 {
		return nil, nil
	}
	n := NodeAt(v.sv.node, v.selNodes[0])
	if n == nil {
		return nil, nil
	}
	return n, v.sv.byNode[n]
}

// dropStaleSelection forgets the selected paths that name nothing after a change.
func (v *Viewer) dropStaleSelection() {
	kept := v.selNodes[:0:0]
	for _, p := range v.selNodes {
		if NodeAt(v.sv.node, p) != nil {
			kept = append(kept, p)
		}
	}
	v.selNodes = kept
}

func (v *Viewer) drawSceneOverlay() {
	sv := v.sv
	if !v.sceneMode() || sv.overlay == nil {
		return
	}
	zoom := v.view.Compositor().Camera().Zoom
	if zoom <= 0 {
		zoom = 1
	}
	r := gfx.NewRecorder()
	for _, p := range v.selNodes {
		pl := sv.byNode[NodeAt(sv.node, p)]
		if pl == nil {
			continue
		}
		x, y := pl.X, pl.Y
		if d := sv.drag; d.pl == pl && d.moved && pl.Gfx != nil {
			t := pl.Gfx.Transform()
			x, y = x+t[4]-d.ox, y+t[5]-d.oy
		}
		r.StrokeRect(gfx.R(x, y, pl.W, pl.H), gfx.StrokeStyle{Width: 2 / zoom}, accent)
	}
	pic := r.Finish()
	ov := sv.overlay
	v.scene.Update(func(tx *gfx.Tx) { tx.SetPicture(ov, pic) })
}

// selectScene makes paths the selection of the scene element on the canvas.
func (v *Viewer) selectScene(paths []string) {
	v.selNodes = paths
	v.drawSceneOverlay()
	v.refreshProps()
	v.syncItems()
	if v.lib == nil {
		v.sh.publish(DocEvent{Path: v.state.Doc.Path, Kind: EventSelect, Version: v.version})
	}
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
		v.view.Compositor().SetCamera(CenterOn(cam, c, float64(w), float64(h)))
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
		v.press = press{active: true, cam: v.view.Compositor().Camera(), pos: p.Pos, from: p.Pos}
		hit := sv.hitScene(at)
		_, cur := v.selectedScene()
		switch {
		case hit != nil && hit == cur && hit.Free && hit.Gfx != nil:
			t := hit.Gfx.Transform()
			sv.drag = sceneDrag{pl: hit, from: at, ox: t[4], oy: t[5]}
		case hit != nil:
			v.selectScene([]string{NodePath(sv.node, hit.Src)})
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
			return
		}
		if v.press.pan {
			v.view.Pan(p.Pos.X-v.press.pos.X, p.Pos.Y-v.press.pos.Y)
			v.press.pos = p.Pos
			return
		}
		if d := &sv.drag; d.pl != nil {
			dx, dy := at.X-d.from.X, at.Y-d.from.Y
			d.moved = d.moved || math.Abs(dx)+math.Abs(dy) >= 1
			n := d.pl.Gfx
			v.scene.Update(func(tx *gfx.Tx) { tx.Move(n, d.ox+dx, d.oy+dy) })
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
	p := NodePath(v.sv.node, hit.Src)
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
	if e, ok := SetPropEdit(v.text, pl.Src, "pos", numText(nx), numText(ny)); ok {
		v.sceneEdit(e)
	}
}

// sceneEdit applies a text edit made by the window; a refused one shows in the banner.
func (v *Viewer) sceneEdit(e TextEdit) bool {
	if v.lib != nil {
		v.setBanner("The library is read only here: open it in its own window to edit it.")
		return false
	}
	// An edit from the window must not add scene errors: autosave would write a broken file. Errors are
	// counted, not compared, because their positions move with the edit.
	if e.Start >= 0 && e.Start <= e.End && e.End <= len(v.text) {
		after := v.text[:e.Start] + e.New + v.text[e.End:]
		if was, now := v.sceneErrorCount(v.text), v.sceneErrorCount(after); now > was || (now < 0 && was >= 0) {
			v.setBanner("Not applied: the change adds scene errors")
			return false
		}
	}
	if err := v.applyText(e.Start, e.End, e.New); err != nil {
		v.setBanner(err.Error())
		return false
	}
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
	v.sceneEdit(DeleteEdit(v.text, n))
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
	e := InsertEdit(v.text, target, cmd)
	at := strings.Index(e.New, head)
	if at < 0 || !v.sceneEdit(e) {
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
	e, starts, ok := DuplicateEdit(v.text, srcs)
	if !ok || !v.sceneEdit(e) {
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
	if e == nil || len(e.Items) > 0 {
		v.setBanner("Screens and components go into a scene element: choose one with frames, or an empty element.")
		return false
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
	v.view.Compositor().SetCamera(CenterOn(gfx.Camera{Zoom: z}, c, float64(w), float64(h)))
	v.drawSceneOverlay()
}
