//go:build darwin || linux || windows

package app

import (
	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
)

// overlayZ keeps the selection node above every drawable.
const overlayZ = 1 << 20

// press is the left button held on the canvas.
type press struct {
	active bool
	pan    bool // the window pans: the session refused the press, or the middle button
	cam    gfx.Camera
	pos    gfx.Point      // view point of the last event
	from   gfx.Point      // view point of the press
	scene  geometry.Point // the press in the session coordinates, for the classic drag preview
}

// realize puts the element on the canvas through go/scene and draws the selection over it.
func (v *Viewer) realize() {
	v.realizeScene()
}

// onRedraw is the session's repaint request: a new scene, or only the pictures that changed.
func (v *Viewer) onRedraw(r editor.Redraw) {
	if v.session == nil {
		return
	}
	if r.Rebuilt {
		defer v.refreshProps()
		v.realize()
		return
	}
	// The session moved the drawables of a classic diagram the scene draws: the frames follow the
	// selection, and the scene itself is rebuilt on the commit as it always was.
	if v.sv != nil && v.sv.classic {
		v.previewClassic()
		defer v.syncItems()
		defer v.syncIndicators()
		v.drawSceneOverlay()
		v.refreshProps()
	}
}

// onElement runs when the session puts another element on the canvas; the camera of the old one is kept.
func (v *Viewer) onElement() {
	// The paths of the marks name nodes of the element that was on the canvas: they mean nothing here.
	v.marks.list = nil
	v.saveCamera()
	v.state.Current = v.session.Element()
	v.trackDocElem()
}

// trackDocElem remembers the element on the canvas when it is written in the document, for the
// components inserted from a mounted file.
func (v *Viewer) trackDocElem() {
	if v.state.Current != nil && model.MountOf(v.state.Current, false) == nil {
		if v.docElem != v.state.Current {
			v.docSel = nil
		}
		v.docElem = v.state.Current
	}
}

// onChange runs after every change of the model, undo and redo included.
func (v *Viewer) onChange() {
	v.sessionChanged()
	if v.libErr != nil {
		v.setBanner(v.libErr.Error())
	}
	v.updateTitle()
	v.scheduleAutosave()
	// The undo stacks change after the store's callback returns, so Undo and Redo are set again then.
	v.app.Async(v.updateMenus)
	v.refreshOutline()
	v.sh.docChanged(v)
	v.sh.publish(DocEvent{Path: v.state.Doc.Path, Kind: EventChange, Version: v.version})
}

func (v *Viewer) sceneEvent(p *view.Pointer, at gfx.Point) editor.MouseEvent {
	return editor.MouseEvent{P: v.smap.ToScene(at), Mods: ModsOf(p.Mods), Clicks: p.Count}
}

// onPointer sends the left button to the session and pans when the session does not take the press.
// The middle button always pans; the right one opens the context menu.
func (v *Viewer) onPointer(p *view.Pointer) {
	if v.commentPointer(p) {
		return
	}
	if p.Button == 3 {
		if p.Kind == view.PointerDown {
			v.rightClick = v.smap.ToScene(p.Scene)
			v.sceneRightClick(p.Scene)
		}
		return
	}
	if p.Button == 2 {
		v.middlePan(p)
		return
	}
	if v.sceneMode() {
		v.scenePointer(p)
		return
	}
	switch p.Kind {
	case view.PointerDown:
		if p.Button != 1 {
			return
		}
		v.press = press{active: true, cam: v.view.Compositor().Camera(), pos: p.Pos, from: p.Pos, scene: v.smap.ToScene(p.Scene)}
		v.dragAt = v.press.scene
		owned := v.session != nil && v.session.MouseDown(v.sceneEvent(p, p.Scene))
		if !owned {
			v.press.pan = true
			v.view.SetCursor(view.CursorGrabbing)
		}
	case view.PointerMove:
		if !v.press.active || !p.Drag {
			return
		}
		if v.press.pan {
			v.panView(p.Pos.X-v.press.pos.X, p.Pos.Y-v.press.pos.Y)
			v.press.pos = p.Pos
			// The pointer is followed on the scene as it was at the press: panning must not hide the movement.
			if v.session != nil {
				v.session.MouseMove(v.sceneEvent(p, ViewToScene(v.press.cam, p.Pos)))
			}
			return
		}
		ev := v.sceneEvent(p, p.Scene)
		v.dragAt = ev.P
		v.session.MouseMove(ev)
		v.dragAt = v.press.scene // no preview without a move in flight
	case view.PointerUp, view.PointerCancel:
		if !v.press.active {
			return
		}
		at := p.Scene
		if v.press.pan {
			at = ViewToScene(v.press.cam, p.Pos)
		}
		ev := v.sceneEvent(p, at)
		ev.Cancel = p.Kind == view.PointerCancel
		pressed := v.press
		v.press = press{}
		v.dragAt = geometry.Point{} // the preview is over; the commit rebuilds the scene
		v.previewBoxes = nil        // and the boxes are the model's again
		v.view.SetCursor(view.CursorArrow)
		if v.session != nil {
			v.session.MouseUp(ev)
			if !pressed.pan && !ev.Cancel {
				v.maybeQuickPanel(pressed, p.Pos)
			}
		}
	}
}

func (v *Viewer) middlePan(p *view.Pointer) {
	switch p.Kind {
	case view.PointerDown:
		v.press.pos = p.Pos
		v.view.SetCursor(view.CursorGrabbing)
	case view.PointerMove:
		if p.Drag {
			v.panView(p.Pos.X-v.press.pos.X, p.Pos.Y-v.press.pos.Y)
			v.press.pos = p.Pos
		}
	case view.PointerUp, view.PointerCancel:
		v.view.SetCursor(view.CursorArrow)
	}
}

// onKey sends the keys of the canvas to the session; Cmd shortcuts belong to the menu.
func (v *Viewer) onKey(k *view.Key) {
	if !k.Down || v.session == nil || v.commentKey(k) {
		return
	}
	name := KeyName(k.Code, k.Char)
	if name == "" || menuOwned(name, k.Mods, cmdIsCtrl) {
		return
	}
	if v.sceneMode() {
		// Tab adds an item, as in a diagram: the frame is the container of a screen.
		if name == "Tab" {
			v.addSceneItem("Frame")
			return
		}
		v.sceneKey(name)
		return
	}
	v.session.Key(editor.KeyEvent{Key: name, Mods: ModsOf(k.Mods)})
}

// onEdit opens the inline editor over an item: a line for the name and the value, a box for the body.
func (v *Viewer) onEdit(item *model.DiagramItem, mode editor.EditMode) {
	t, ok := v.session.BeginEdit(item, mode)
	if !ok {
		return
	}
	s := v.session
	ed := v.view.Edit(view.EditOptions{
		Rect:      v.smap.RectToGfx(t.Rect),
		Text:      t.Text,
		Font:      g.Font{Px: t.FontSize},
		Multiline: mode == editor.EditBody,
		OnCommit: func(text string) {
			v.inline, v.activeText = nil, nil
			s.FinishEdit(text, true)
		},
		OnCancel: func() {
			v.inline, v.activeText = nil, nil
			s.FinishEdit("", false)
		},
	})
	if ed == nil {
		s.FinishEdit("", false)
		return
	}
	v.inline, v.inlineMode = ed, mode
	v.activeText = ed.Widget().Unwrap()
}

// endInline closes the inline editor, keeping its text when commit is set.
func (v *Viewer) endInline(commit bool) {
	if v.inline == nil {
		return
	}
	if commit {
		v.inline.Commit()
	} else {
		v.inline.Cancel()
	}
}

// moveInline puts the editor back over its item after the scene was rebuilt; an item that is gone
// takes the editor with it.
func (v *Viewer) moveInline() {
	if v.inline == nil {
		return
	}
	item := v.session.Editing()
	if item == nil {
		v.inline.Cancel()
		return
	}
	if t, ok := v.session.EditTarget(item, v.inlineMode); ok {
		v.inline.SetRect(v.smap.RectToGfx(t.Rect))
	}
}

// showOperation is the box of Space: a line of operations to apply to the selection.
func (v *Viewer) showOperation() {
	sel := v.session.Selection()
	if len(sel) == 0 {
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4))
	p.Add(g.Text(g.Focused(), g.Hint("Operation"), g.Init(func(t *g.TextView) {
		t.OnActivate(func() {
			text := t.Text()
			p.Close()
			v.session.Operate(text)
		})
	})))
	v.openPopover(p)
	v.popoverAt(p, sel[0])
}

// popoverAt shows p over the top left of the item.
func (v *Viewer) popoverAt(p *g.Popover, item *model.DiagramItem) {
	b, ok := v.drawnRect(item)
	if !ok {
		p.ShowAt(v.view.Canvas(), g.PopoverBelow)
		return
	}
	cam := v.view.Compositor().Camera()
	top := gfx.Point{X: b.Min.X + b.W()/2, Y: b.Min.Y}
	x, y := int((top.X-cam.Origin.X)*cam.Zoom), int((top.Y-cam.Origin.Y)*cam.Zoom)
	p.ShowAtPoint(v.view.Canvas(), x, y, g.PopoverAbove)
}

func (v *Viewer) openPopover(p *g.Popover) {
	v.popover = p
	p.OnClose(func() {
		if v.popover == p {
			v.popover = nil
		}
		v.activeText = nil
		v.app.Async(func() {
			p.Dispose()
			if v.view != nil {
				v.view.Canvas().Unwrap().SetFocus()
			}
		})
	})
}

func (v *Viewer) closePopover() {
	if v.popover != nil {
		v.popover.Close()
	}
}
