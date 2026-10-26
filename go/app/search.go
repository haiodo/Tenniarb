//go:build darwin || linux || windows

package app

import (
	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// search is the Goto Item of Cmd+F: a bar over the canvas, because the toolbar search field cannot be
// focused from code (gowt gap). The field in the toolbar is searchTo.
type search struct {
	box     *g.Panel
	field   *g.TextView
	list    *g.ListView
	el      *model.Element
	results []*model.DiagramItem
	// last and at are the query of the toolbar field and the result it showed last.
	last string
	at   int
}

// searchBar is the bar: a field and the list of results, hidden until Goto Item.
func (v *Viewer) searchBar() g.Option {
	return g.VBox(g.Padding(2), g.Gap(2), g.GrowX(), g.Ref(&v.search.box),
		g.Text(g.GrowX(), g.Hint("Search..."), g.ID("goto-field"), g.Ref(&v.search.field), g.Init(v.initSearchField)),
		g.List(nil, g.GrowX(), g.Size(0, 110), g.Ref(&v.search.list), g.Init(func(l *g.ListView) { l.OnSelect(v.jump) })))
}

func (v *Viewer) initSearchField(t *g.TextView) {
	t.OnChange(v.runSearch)
	t.OnActivate(v.hideSearch)
	t.Unwrap().AddKeyListener(swt.KeyListenerKeyPressedAdapter(func(e *swt.KeyEvent) {
		s, n := &v.search, len(v.search.results)
		switch {
		case e.KeyCode == int32(swt.ESC):
			v.hideSearch()
		case n > 0 && e.KeyCode == swt.ARROW_UP:
			v.jump((s.list.Index() - 1 + n) % n)
		case n > 0 && e.KeyCode == swt.ARROW_DOWN:
			v.jump((s.list.Index() + 1) % n)
		default:
			return
		}
		e.Doit = false
	}))
}

// showSearch is Swift's Goto Item: the items of the element whose name or text has the query; the
// selected result is selected and centered on the canvas as the query changes.
func (v *Viewer) showSearch() {
	if v.session == nil {
		return
	}
	s := &v.search
	s.el = v.session.Element()
	s.box.SetVisible(true)
	s.field.SetText("")
	v.runSearch("")
	v.win.Unwrap().Layout()
	s.field.Focus()
	v.activeText = s.field.Unwrap()
}

func (v *Viewer) hideSearch() {
	if v.search.box.Unwrap().GetVisible() {
		v.search.box.SetVisible(false)
		v.activeText = nil
		v.win.Unwrap().Layout()
		v.view.Canvas().Unwrap().SetFocus()
	}
}

func (v *Viewer) runSearch(q string) {
	s := &v.search
	s.results = nil
	if q != "" && s.el != nil {
		s.results = editor.SearchItems(s.el, q)
	}
	rows := make([]string, len(s.results))
	for i, it := range s.results {
		rows[i] = editor.RowText(it)
	}
	s.list.SetItems(rows)
	v.jump(0)
}

func (v *Viewer) jump(i int) {
	s := &v.search
	if i >= 0 && i < len(s.results) && v.session != nil {
		s.list.Select(i)
		v.gotoItem(s.results[i])
	}
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
// query, and the next one each time the same query comes again (Return).
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
