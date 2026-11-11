//go:build darwin || linux || windows

package app

import (
	"math"
	"slices"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
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

	// A link drawn in the scene with Ctrl held: the item it leaves and the one under the pointer.
	lineFrom, lineTarget *scene.Placed
	nodeOf               map[*scene.Placed]*scene.Node // the node of an item written in the element
	moved                bool                          // a drag put the drawn items somewhere the model has not written yet

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
				vals = classic.Values(e, v.classicContext(e))
			}
			return vals(at, prop)
		}
	}
	return o
}

// classicContext is the evaluator of a classic diagram: what it asks for the size of an item is
// answered by the scene, and the text is measured by the caller's measurer, as the scene measures it.
func (v *Viewer) classicContext(e *model.Element) model.IExecutionContext {
	path := strings.Join(PathOf(e), ".")
	return classic.SceneContext(v.stree, path, gfx.DefaultMeasurer,
		func(text string, fontSize float64) model.DrawableSize {
			w, h := scene.MeasureText(gfx.DefaultMeasurer, text, fontSize)
			return model.DrawableSize{Width: w, Height: h}
		})(e)
}

// bounds is the extent of the drawn items with the padding of the classic canvas around.
func (sv *sceneView) bounds() {
	pad := geometry.Padding
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
		byPath: map[string]*scene.Placed{}, pathOf: map[*scene.Placed]string{}, inst: map[*scene.Placed]instRef{},
		nodeOf: map[*scene.Placed]*scene.Node{}}
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
			sv.byPath[p], sv.pathOf[k], sv.nodeOf[k] = k, p, k.Src
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
