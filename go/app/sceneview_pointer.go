//go:build darwin || linux || windows

package app

import (
	"math"
	"slices"
	"strconv"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"

	"github.com/haiodo/tenniarb/go/scene"
)

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
		// A press on the selected item moves it, wherever on it the press lands, even where a child or
		// a layer above covers all of it: otherwise an item made of one instance could not be dragged
		// at all. A node with no place of its own moves the nearest ancestor that has one. A second
		// click still goes into the child.
		var move *scene.Placed
		if cur != nil && p.Count < 2 && cur.X <= at.X && at.X <= cur.X+cur.W && cur.Y <= at.Y && at.Y <= cur.Y+cur.H {
			move = sv.movable(cur)
			if move != nil {
				hit = move
			}
		}
		if hit != nil && move == nil {
			hit = sv.pick(hit, cur, p.Count)
		}
		// Ctrl draws a link, as Swift does: the line leaves the item under the pointer and the item it is
		// released on is the other end.
		if p.Mods.Ctrl && sv.linkable(hit) {
			sv.lineFrom, sv.lineTarget = hit, nil
			sv.line = &[2]gfx.Point{sv.contentToGfx(plCentre(hit)), sv.contentToGfx(at)}
			v.press = press{active: true, cam: v.view.Compositor().Camera(), pos: p.Pos, from: p.Pos}
			v.view.SetCursor(view.CursorCrosshair)
			v.drawSceneOverlay()
			return
		}
		switch {
		case move != nil:
			t := move.Gfx.Transform()
			sv.drag = sceneDrag{pl: move, from: at, ox: t[4], oy: t[5]}
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
		if sv.lineFrom != nil {
			sv.line = &[2]gfx.Point{sv.contentToGfx(plCentre(sv.lineFrom)), sv.contentToGfx(at)}
			sv.lineTarget = nil
			if t := sv.hitScene(at); sv.linkable(t) && t != sv.lineFrom && t.Parent == sv.lineFrom.Parent {
				sv.lineTarget = t
			}
			v.drawSceneOverlay()
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
		if from := sv.lineFrom; from != nil {
			to := sv.lineTarget
			sv.lineFrom, sv.lineTarget, sv.line = nil, nil, nil
			v.drawSceneOverlay()
			if p.Kind != view.PointerCancel && to != nil {
				v.insertSceneLink(from, to)
			}
			return
		}
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

// linkable reports whether a link can end at pl: only an item written in the element can, not a part of a
// component and not the box of the element itself.
func (sv *sceneView) linkable(pl *scene.Placed) bool {
	if pl == nil || pl.W <= 0 || pl.H <= 0 {
		return false
	}
	n := sv.nodeOf[pl]
	return n != nil && n.Kind == scene.KItem
}

// contentToGfx puts a point of the content where the selection picture expects it, which is the gfx scene
// and carries the offset of the padding.
func (sv *sceneView) contentToGfx(p gfx.Point) gfx.Point {
	return gfx.Point{X: p.X + sv.off.X, Y: p.Y + sv.off.Y}
}

// insertSceneLink writes a link from one item to the other, in the item that holds both of them: the file
// takes `link A B`, and the indexes only when a name is written twice among the children of that item.
func (v *Viewer) insertSceneLink(from, to *scene.Placed) {
	sv := v.sv
	a, b := sv.nodeOf[from], sv.nodeOf[to]
	if a == nil || b == nil || a.Parent == nil || a.Parent != b.Parent {
		return
	}
	owner := a.Parent
	cmd := CommandText("link", a.Name, b.Name)
	if i, j, ok := linkIndexes(owner, a, b); ok {
		cmd += " { " + CommandText("source-index", strconv.Itoa(i)) + "; " +
			CommandText("target-index", strconv.Itoa(j)) + " }"
	}
	v.insertAndSelect(owner, cmd, cmd)
}

// linkIndexes are the indexes a link carries when a name is written more than once among the children of
// owner: the place of a and of b among the children of that name. False when both names are unique there.
func linkIndexes(owner, a, b *scene.Node) (int, int, bool) {
	same := func(name string) []*scene.Node {
		var out []*scene.Node
		for _, c := range owner.Content {
			if c.Kind == scene.KItem && c.Key == name {
				out = append(out, c)
			}
		}
		return out
	}
	as, bs := same(a.Key), same(b.Key)
	if len(as) < 2 && len(bs) < 2 {
		return 0, 0, false
	}
	return slices.Index(as, a), slices.Index(bs, b), true
}
