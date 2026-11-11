//go:build darwin || linux || windows

package app

import (
	"math"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
)

// showQuickPanel opens the panel of quick styles over the selected item (Swift showPopup).
func (v *Viewer) showQuickPanel() {
	if v.session == nil || len(v.session.Selection()) != 1 {
		return
	}
	item := v.session.Selection()[0]
	v.closePopover()
	p := v.win.Popover(g.Gap(2), g.Padding(4))
	var buttons []g.Option
	for _, seg := range editor.QuickPanel(item.Kind) {
		buttons = append(buttons, g.Button(seg.Label, g.Tooltip(seg.Style.Label), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { v.quickMenu(p, seg.Style) })
		})))
	}
	if len(buttons) == 0 {
		p.Dispose()
		return
	}
	p.Add(g.HBox(append([]g.Option{g.Gap(2), g.Padding(0)}, buttons...)...))
	v.openPopover(p)
	v.popoverAt(p, item)
}

// quickMenu closes the panel and opens the options of one style as a menu at the pointer: a menu
// opened inside the panel would close it, and the menu with it.
func (v *Viewer) quickMenu(p *g.Popover, q editor.QuickStyle) {
	entries := v.session.QuickEntries(q)
	p.Close()
	v.app.Async(func() {
		v.menuOverride = entries
		v.canvasMenu.Show()
	})
}

// maybeQuickPanel shows the panel after a click that selected one item.
func (v *Viewer) maybeQuickPanel(pr press, pos gfx.Point) {
	if !v.autoQuick {
		return
	}
	if math.Hypot(pos.X-pr.from.X, pos.Y-pr.from.Y) > 3 || len(v.session.Selection()) != 1 {
		v.closePopover()
		return
	}
	v.showQuickPanel()
}

func (v *Viewer) testLayout() { v.session.TestLayout(v.visibleScene()) }
