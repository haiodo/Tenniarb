//go:build darwin || linux || windows

package app

import (
	"slices"
	"time"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// The marks of the canvas show with what an agent works: a frame over the node it selects or edits, with
// a label, and a short frame pulse over a node the window just changed. They are a picture on the HUD: no
// model, no undo step, no pointer.

// markKind says why a mark is there.
type markKind string

const (
	markSelect markKind = "select" // an agent selected the node
	markEdit   markKind = "edit"   // an agent edited the node
	markPulse  markKind = "pulse"  // the window edited the node: a frame that fades, no label
)

// How long a mark lives and how often the HUD redraws it.
const (
	markTTL   = 4 * time.Second         // an agent mark: a few seconds after the action
	pulseTTL  = 1200 * time.Millisecond // the mark of the window's own edit
	markGlow  = 300 * time.Millisecond  // a fresh mark glows this long, 3 ticks of markTick
	markTick  = 100 * time.Millisecond
	maxMarks  = 16 // the newest marks kept when an agent touches many nodes
	markLabel = "agent"
)

// mark is one mark: the node it names, why, and when it goes.
type mark struct {
	path  string // the node as the selection names it: a scene node path, or the name of a classic item
	kind  markKind
	label string    // the text over the frame; empty for a pulse
	at    time.Time // when it appeared: its frame glows for markGlow after that
	until time.Time
}

// markUI is the HUD layer of the marks of a window and the tick that redraws them.
type markUI struct {
	list    []mark
	node    *gfx.Node
	shown   int  // how many marks the picture on the HUD shows
	ticking bool // a markTick is on its way
}

var markFont = gfx.Font{Size: 11, Weight: 700}

// markNode shows a mark over the node at path of the element on the canvas; a path the canvas does not
// draw is kept anyway, so the QA state tells of it.
func (v *Viewer) markNode(path string, kind markKind, label string) {
	if path == "" {
		return
	}
	now := time.Now()
	ttl := markTTL
	if kind == markPulse {
		ttl = pulseTTL
	}
	m := &v.marks
	m.list = slices.DeleteFunc(m.list, func(k mark) bool { return k.path == path && k.kind == kind })
	m.list = append(m.list, mark{path: path, kind: kind, label: label, at: now, until: now.Add(ttl)})
	if len(m.list) > maxMarks {
		m.list = m.list[len(m.list)-maxMarks:]
	}
	v.syncMarks()
	v.marksTicking()
}

// markNodes shows the mark of an agent's select over the nodes it selected.
func (v *Viewer) markNodes(nodes []string) {
	for _, p := range nodes {
		v.markNode(p, markSelect, markLabel)
	}
}

// markEdited shows the mark of an agent's edit over the node the edit touched at byte off of the text of
// the document; nothing when that node is not on the canvas.
func (v *Viewer) markEdited(off int) {
	if !v.sceneMode() || v.sv.node == nil {
		return
	}
	v.markNode(NodePath(v.sv.node, nodeAtText(v.sv.node, off)), markEdit, markLabel)
}

// nodeAtText is the innermost item, use or element written under n that holds the byte offset off of its
// file; nil when the offset is in n itself or outside it.
func nodeAtText(n *scene.Node, off int) *scene.Node {
	for _, c := range slices.Concat(n.Elems, n.Content) {
		if c.Range.Start <= off && off < c.Range.End {
			if d := nodeAtText(c, off); d != nil {
				return d
			}
			switch c.Kind {
			case scene.KItem, scene.KUse, scene.KElement:
				return c
			}
		}
	}
	return nil
}

// liveMarks drops the marks that are gone from list, which it changes in place.
func liveMarks(list []mark, now time.Time) []mark {
	return slices.DeleteFunc(list, func(m mark) bool { return !now.Before(m.until) })
}

// syncMarks redraws the marks on the HUD: they sit in view points, so a pan or a zoom moves them with the
// nodes. The HUD is touched only while a mark is there or was on the last picture.
func (v *Viewer) syncMarks() {
	if v.view == nil {
		return
	}
	m := &v.marks
	now := time.Now()
	m.list = liveMarks(m.list, now)
	if len(m.list) == 0 && m.shown == 0 {
		return
	}
	m.shown = len(m.list)
	pic := v.marksPicture(now)
	comp := v.view.Compositor()
	comp.HUD().Update(func(tx *gfx.Tx) {
		if m.node == nil {
			m.node = gfx.NewNode(4)
			tx.Add(nil, m.node)
			tx.SetShape(m.node, nil, gfx.HitNone) // the marks never take the pointer
		}
		tx.SetPicture(m.node, pic)
	})
}

// marksTicking keeps the HUD redrawing while a mark glows, and wakes up once when the first one goes. A
// mark is gone from the picture without anybody telling the window: the tick only reports it.
func (v *Viewer) marksTicking() {
	m := &v.marks
	if m.ticking || v.app == nil || len(m.list) == 0 {
		return
	}
	now := time.Now()
	d := time.Duration(0)
	for _, k := range m.list {
		if now.Before(k.at.Add(markGlow)) {
			d = markTick // a fresh mark glows: its next frame
			break
		}
		if left := k.until.Sub(now); d == 0 || left < d {
			d = left
		}
	}
	if d <= 0 {
		d = markTick
	}
	m.ticking = true
	v.app.After(d, v.marksTick)
}

// marksTick redraws the marks and stops when none is left. A window without an app (a test) has no ticks.
func (v *Viewer) marksTick() {
	m := &v.marks
	m.ticking = false
	if v.sh != nil && !slices.Contains(v.sh.viewers, v) { // the window is closed: leave no tick behind
		m.list = nil
		return
	}
	m.list = liveMarks(m.list, time.Now())
	v.syncMarks()
	v.marksTicking()
}

// marksPicture draws the marks in view points, y down from the top left corner of the view: the frame of
// each node, wider and stronger while the mark is fresh, and the label of an agent mark above its frame.
func (v *Viewer) marksPicture(now time.Time) *gfx.Picture {
	cam := v.view.Compositor().Camera()
	if cam.Zoom <= 0 {
		cam.Zoom = 1
	}
	rec := gfx.NewRecorder()
	for _, m := range v.marks.list {
		r, ok := v.markRect(m.path)
		if !ok {
			continue
		}
		x, y := (r.Min.X-cam.Origin.X)*cam.Zoom, (r.Min.Y-cam.Origin.Y)*cam.Zoom
		w, h := r.W()*cam.Zoom, r.H()*cam.Zoom
		width, alpha := 2.0, 1.0
		if age := now.Sub(m.at); age < markGlow {
			k := 1 - float64(age)/float64(markGlow)
			width, alpha = 2+2*k, 0.5+0.5*k
		}
		rec.StrokeRect(gfx.R(x, y, w, h), gfx.StrokeStyle{Width: width}, gfx.RGBA(255, 149, 0, alpha))
		if m.label == "" {
			continue
		}
		t := gfx.DefaultMeasurer.MeasureText(m.label, markFont)
		chip := gfx.R(x, y-t.Ascent-t.Descent-8, t.Width+10, t.Ascent+t.Descent+6)
		rec.FillRRect(chip, 4, gfx.RGBA(30, 30, 30, 0.82))
		rec.Text(m.label, chip.Min.X+5, chip.Min.Y+3+t.Ascent, markFont, gfx.RGB(255, 255, 255))
	}
	return rec.Finish()
}

// markRect is where the canvas draws the node at path, in the gfx scene (y down).
func (v *Viewer) markRect(path string) (gfx.Rect, bool) {
	switch {
	case v.sceneMode():
		pl := v.sv.byPath[path]
		if pl == nil {
			return gfx.Rect{}, false
		}
		return gfx.R(v.sv.off.X+pl.X, v.sv.off.Y+pl.Y, pl.W, pl.H), true
	case v.sceneDrawn(): // a classic diagram drawn by the scene: the mark names the model item
		pl := v.classicNamed(path)
		if pl == nil {
			return gfx.Rect{}, false
		}
		return gfx.R(v.sv.off.X+pl.X, v.sv.off.Y+pl.Y, pl.W, pl.H), true
	}
	return gfx.Rect{}, false
}

// qaMark is one mark for the QA state: the node, why it is there and how long it has left.
type qaMark struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Left int64  `json:"left"` // milliseconds
}

// qaMarks lists the marks that are still there.
func (v *Viewer) qaMarks() []qaMark {
	now := time.Now()
	out := []qaMark{}
	for _, m := range v.marks.list {
		if left := m.until.Sub(now); left > 0 {
			out = append(out, qaMark{Path: m.path, Kind: string(m.kind), Left: left.Milliseconds()})
		}
	}
	return out
}
