//go:build darwin || linux || windows

package app

import (
	"fmt"
	"github.com/haiodo/tenniarb/go/servicewire"
	"slices"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/comments"
)

func (v *Viewer) toggleCommentMode() { v.setCommentMode(!v.cm.mode) }

func (v *Viewer) setCommentMode(on bool) {
	if on && (v.session == nil || v.view == nil) {
		return
	}
	if on && v.state.Doc.Path == "" {
		v.notice("Save the file to comment on it.")
		return
	}
	v.cm.mode = on
	if v.view != nil {
		cur := view.CursorArrow
		if on {
			cur = view.CursorCrosshair
		}
		v.view.SetCursor(cur)
	}
	v.syncPins()
}

// commentKey: C toggles the comment mode and Escape leaves it. It reports whether it took the key.
func (v *Viewer) commentKey(k *view.Key) bool {
	if v.cm.mode && int32(k.Code) == int32(swt.ESC) {
		v.setCommentMode(false)
		return true
	}
	if k.Mods.Cmd || k.Mods.Ctrl || k.Mods.Alt || v.userTyping() || KeyName(k.Code, k.Char) != "c" {
		return false
	}
	v.toggleCommentMode()
	return true
}

// commentPointer takes a left press on a pin (always) or, in the comment mode, anywhere on the canvas.
func (v *Viewer) commentPointer(p *view.Pointer) bool {
	if p.Button != 1 || p.Kind != view.PointerDown || v.session == nil {
		return false
	}
	if pin, ok := PinAtView(v.cm.pins, v.view.Compositor().Camera(), p.Pos); ok {
		v.showThread(pin.ID)
		return true
	}
	if !v.cm.mode {
		return false
	}
	v.compose(p.Scene, p.Pos)
	return true
}

// itemAt is the classic item under the gfx point at, the smallest one when they overlap: the boxes are the
// ones the canvas draws, not the ones go/render would have drawn.
func (v *Viewer) itemAt(at gfx.Point) (string, gfx.Rect) {
	name, best := "", gfx.Rect{}
	if v.state.Current == nil {
		return name, best
	}
	for _, it := range v.state.Current.Items {
		if it.IsLink() || it.Name == "" {
			continue
		}
		r, ok := v.drawnRect(it)
		if !ok || at.X < r.Min.X || at.X > r.Max.X || at.Y < r.Min.Y || at.Y > r.Max.Y {
			continue
		}
		if name == "" || r.W()*r.H() < best.W()*best.H() {
			name, best = it.Name, r
		}
	}
	return name, best
}

// anchorAt is the anchor of a click at the gfx scene point at.
func (v *Viewer) anchorAt(at gfx.Point) comments.Anchor {
	element := PathOf(v.state.Current)
	if v.sceneMode() {
		off := v.sv.off
		toElement := func(p gfx.Point) comments.Point { return comments.Point{X: p.X - off.X, Y: p.Y - off.Y} }
		if hit := v.sv.hitScene(gfx.Point{X: at.X - off.X, Y: at.Y - off.Y}); hit != nil {
			return AnchorAt(element, v.sv.pathOf[hit], gfx.R(hit.X+off.X, hit.Y+off.Y, hit.W, hit.H), at, toElement)
		}
		return AnchorAt(element, "", gfx.Rect{}, at, toElement)
	}
	toElement := func(p gfx.Point) comments.Point {
		s := v.smap.ToScene(p)
		return comments.Point{X: s.X, Y: s.Y}
	}
	if v.sceneDrawn() { // a classic diagram drawn by the scene: the anchor names the model item
		off := v.sv.off
		hit := v.sv.hitScene(gfx.Point{X: at.X - off.X, Y: at.Y - off.Y})
		if hit != nil && hit.Src != nil {
			if it := classicItemAt(v.state.Current, hit.Src); it != nil {
				return AnchorAt(element, it.Name, gfx.R(hit.X+off.X, hit.Y+off.Y, hit.W, hit.H), at, toElement)
			}
		}
		return AnchorAt(element, "", gfx.Rect{}, at, toElement)
	}
	name, box := v.itemAt(at)
	return AnchorAt(element, name, box, at, toElement)
}

// placePins are the pins of the comments on the element on the canvas.
func (v *Viewer) placePins() []Pin {
	if v.state.Current == nil || len(v.cm.list) == 0 {
		return nil
	}
	element := PathOf(v.state.Current)
	switch {
	case v.sceneMode():
		off := v.sv.off
		box := func(node string) (gfx.Rect, bool) {
			pl := v.sv.byPath[node]
			if pl == nil {
				return gfx.Rect{}, false
			}
			return gfx.R(pl.X+off.X, pl.Y+off.Y, pl.W, pl.H), true
		}
		return PlacePins(v.cm.list, element, v.cm.resolved, box, func(p comments.Point) gfx.Point {
			return gfx.Point{X: off.X + p.X, Y: off.Y + p.Y}
		})
	case v.sceneDrawn(): // a classic diagram drawn by the scene: the anchor names the model item
		off := v.sv.off
		box := func(node string) (gfx.Rect, bool) {
			pl := v.classicNamed(node)
			if pl == nil {
				return gfx.Rect{}, false
			}
			return gfx.R(pl.X+off.X, pl.Y+off.Y, pl.W, pl.H), true
		}
		return PlacePins(v.cm.list, element, v.cm.resolved, box, func(p comments.Point) gfx.Point {
			return v.smap.ToGfx(renderPoint(p))
		})
	}
	return nil
}

// syncPins redraws the pins of the HUD after the camera, the view size, the comments or the mode
// changed. The HUD scene is touched only when the picture would differ.
func (v *Viewer) syncPins() {
	if v.view == nil {
		return
	}
	comp := v.view.Compositor()
	w, h, _ := comp.Viewport()
	cam := comp.Camera()
	pins := v.placePins()
	key := pinKey{cam, w, h, v.cm.mode}
	cm := &v.cm
	if cm.node != nil && key == cm.key && slices.Equal(pins, cm.pins) {
		return
	}
	cm.pins, cm.key = pins, key
	if cm.node == nil && len(pins) == 0 && !cm.mode {
		return
	}
	pic := pinsPicture(pins, cam, w, cm.mode)
	comp.HUD().Update(func(tx *gfx.Tx) {
		if cm.node == nil {
			cm.node = gfx.NewNode(2)
			tx.Add(nil, cm.node)
			tx.SetShape(cm.node, nil, gfx.HitNone) // clicks on a pin are found by PinAtView
		}
		tx.SetPicture(cm.node, pic)
	})
}

var (
	pinOpen     = gfx.RGB(255, 149, 0)
	pinResolved = gfx.RGB(142, 142, 147)
	pinFont     = gfx.Font{Size: 11, Weight: 700}
	hintFont    = gfx.Font{Size: 12}
)

// pinsPicture draws the markers in view points: a filled disc with the number; a detached pin is a ring.
func pinsPicture(pins []Pin, cam gfx.Camera, viewW float64, mode bool) *gfx.Picture {
	rec := gfx.NewRecorder()
	sh := gfx.Shadow{DX: 0, DY: 1, Blur: 3, Color: gfx.RGBA(0, 0, 0, 0.35)}
	for _, p := range pins {
		c := pinView(p, cam)
		col := pinOpen
		if p.Resolved {
			col = pinResolved
		}
		disc := gfx.EllipsePath(gfx.R(c.X-pinRadius, c.Y-pinRadius, 2*pinRadius, 2*pinRadius))
		text := gfx.RGB(255, 255, 255)
		if p.Detached {
			rec.FillShadowPath(disc, gfx.RGB(255, 255, 255), sh)
			rec.StrokePath(disc, gfx.StrokeStyle{Width: 2, MiterLimit: 10}, col)
			text = col
		} else {
			rec.FillShadowPath(disc, col, sh)
		}
		s := fmt.Sprint(p.N)
		m := gfx.DefaultMeasurer.MeasureText(s, pinFont)
		rec.Text(s, c.X-m.Width/2, c.Y+(m.Ascent-m.Descent)/2, pinFont, text)
	}
	if mode {
		s := "Comment mode: click to place a comment, Esc to leave"
		m := gfx.DefaultMeasurer.MeasureText(s, hintFont)
		r := gfx.R(viewW/2-m.Width/2-10, 8, m.Width+20, m.Ascent+m.Descent+10)
		rec.FillRRect(r, 8, gfx.RGBA(30, 30, 30, 0.82))
		rec.Text(s, r.Min.X+10, r.Min.Y+5+m.Ascent, hintFont, gfx.RGB(255, 255, 255))
	}
	return rec.Finish()
}

// compose opens the editor of a new comment at the click: at in the gfx scene, pos in the view.
func (v *Viewer) compose(at, pos gfx.Point) {
	anchor := v.anchorAt(at)
	v.closePopover()
	p := v.win.Popover(g.Gap(6), g.Padding(8))
	var field *g.TextView
	p.Add(g.Text(g.Focused(), g.Multiline(), g.Border(), g.Scrollbars(), g.Hint("Comment"), g.ID("comment-text"),
		g.Size(g.Scaled(300), g.Scaled(80)), g.Ref(&field)))
	post := func() {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			return
		}
		p.Close()
		v.commentOp(servicewire.CommentsOp{Op: servicewire.CommentAdd, Text: text, Anchor: &anchor})
	}
	p.Add(g.HBox(g.Gap(6), g.Padding(0),
		g.Label("", g.GrowX()),
		g.Button("Cancel", g.ID("comment-cancel"), g.Init(func(b *g.ButtonView) { b.OnClick(p.Close) })),
		g.Button("Post", g.ID("comment-post"), g.Init(func(b *g.ButtonView) { b.OnClick(post) }))))
	v.openPopover(p)
	v.activeText = field.Unwrap()
	p.ShowAtPoint(v.view.Canvas(), int(pos.X), int(pos.Y), g.PopoverBelow)
}
