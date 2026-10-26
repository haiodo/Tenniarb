//go:build darwin || linux || windows

package app

import (
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/look"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// search is the state of the Goto Item field of the toolbar.
type search struct {
	el      *model.Element
	results []*model.DiagramItem
	// last and at are the query and the result shown last.
	last string
	at   int
	// asked counts the Cmd+F presses (QA state).
	asked int
}

// showSearch is Cmd+F: the focus goes to the Goto Item field of the toolbar.
func (v *Viewer) showSearch() {
	v.search.asked++
	look.FocusSearch(v.win)
}

// gotoItem selects the item and scrolls the canvas so that it is in the middle.
func (v *Viewer) gotoItem(item *model.DiagramItem) {
	v.session.Reveal(item)
	d := v.session.Scene().Drawables[item]
	if d == nil || v.rendered == nil {
		return
	}
	b := d.Bounds()
	c := v.smap.ToGfx(render.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2})
	w, h := v.view.Canvas().Size()
	comp := v.view.Compositor()
	comp.SetCamera(CenterOn(comp.Camera(), gfx.Point{X: c.X, Y: c.Y}, float64(w), float64(h)))
}

// searchTo is the Goto Item field of the toolbar: it shows the first item whose name or text has the
// query, and the next one each time the same query comes again (Return). An empty query clears it.
func (v *Viewer) searchTo(q string) {
	if v.session == nil {
		return
	}
	s := &v.search
	el := v.session.Element()
	if q != s.last || s.el != el {
		s.last, s.el, s.at = q, el, -1
		s.results = nil
		if q != "" {
			s.results = editor.SearchItems(el, q)
		}
	}
	if len(s.results) == 0 {
		return
	}
	s.at = (s.at + 1) % len(s.results)
	v.gotoItem(s.results[s.at])
}
