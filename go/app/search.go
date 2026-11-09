//go:build darwin || linux || windows

package app

import (
	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/look"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/model"
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

// showSearch is Cmd+F: the focus goes to the Goto Item field, the toolbar one on macOS where it sits
// in the window bar, else the popover one.
func (v *Viewer) showSearch() {
	v.search.asked++
	if !look.FocusSearch(v.win) {
		v.openSearch()
	}
}

// openSearch is the Goto Item field over the canvas: Windows and Linux draw the toolbar items at the
// top of the panel, where a field made the panel wider than its buttons and hung off the edge.
func (v *Viewer) openSearch() {
	v.closePopover()
	p := v.win.Popover(g.Gap(6), g.Padding(8))
	p.Add(g.Text(g.Search(), g.ID("goto-field"), g.Focused(), g.Hint("Goto Item"),
		g.Size(g.Scaled(280), g.Spacing.ControlHeight),
		g.Init(func(t *g.TextView) {
			t.OnChange(v.searchTo)
			t.OnActivate(func() { v.searchTo(t.Text()) }) // Return goes to the next result
		})))
	v.openPopover(p)
	w, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), w/2, g.Scaled(8), g.PopoverBelow)
}

// gotoItem selects the item and scrolls the canvas so that it is in the middle.
func (v *Viewer) gotoItem(item *model.DiagramItem) {
	v.session.Reveal(item)
	b, ok := v.drawnRect(item)
	if !ok {
		d := v.session.Scene().Drawables[item]
		if d == nil {
			return
		}
		b = v.smap.RectToGfx(d.Bounds())
	}
	c := gfx.Point{X: b.Min.X + b.W()/2, Y: b.Min.Y + b.H()/2}
	w, h := v.view.Canvas().Size()
	comp := v.view.Compositor()
	v.setCamera(CenterOn(comp.Camera(), gfx.Point{X: c.X, Y: c.Y}, float64(w), float64(h)))
}

// searchTo is the Goto Item field of the toolbar: it shows the first item whose name or text has the
// query, and the next one each time the same query comes again (Return). An empty query clears it.
// A link in the field is opened instead.
func (v *Viewer) searchTo(q string) {
	if v.session == nil {
		return
	}
	if link.IsLink(q) { // a pasted link goes to its place, in this or another file
		v.sh.openLink(q, v)
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
