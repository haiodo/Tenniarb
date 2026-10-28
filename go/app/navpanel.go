//go:build darwin || linux || windows

package app

import (
	"slices"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/icons"
	"github.com/haiodo/gowt/look"
)

// navUI is Back and Forward over what the Code panel showed: the history, the buttons of the panel
// header and the Navigate menu items.
type navUI struct {
	hist      navHistory
	restoring bool // an entry is being shown again: it is not a new one
	back, fwd *g.ButtonView
	backItem  *g.MenuItem
	fwdItem   *g.MenuItem
}

// codeHeader is the slim row over the code: Back and Forward as icon buttons.
func (v *Viewer) codeHeader() g.Option {
	button := func(icon, tip string, ref **g.ButtonView, dir int) g.Option {
		return g.Button("", look.GlassButton(), g.Tooltip(tip), g.Disabled(), g.Ref(ref), g.Init(func(b *g.ButtonView) {
			b.SetImage(icons.Get(v.app, icon, icons.Size(12)))
			b.OnClick(func() { v.navGo(dir) })
		}))
	}
	return g.HBox(g.Padding(g.Scaled(2)), g.Gap(2), g.GrowX(),
		button("back", "Back", &v.nav.back, -1),
		button("forward", "Forward", &v.nav.fwd, 1))
}

// navSelection is what is selected now, in the form the history keeps.
func (v *Viewer) navSelection() (Selection, bool) {
	e := v.state.Current
	if e == nil || v.session == nil {
		return Selection{}, false
	}
	s := Selection{Element: PathOf(e)}
	if v.sceneMode() {
		s.Nodes = slices.Clone(v.selNodes)
		return s, true
	}
	for _, it := range v.session.Selection() {
		if !it.IsLink() { // a link has no name to find it by
			s.Items = append(s.Items, it.Name)
		}
	}
	return s, true
}

func (v *Viewer) navValid(s Selection) bool {
	_, _, err := v.resolveSelection(s)
	return err == nil
}

// navSync records the selection as an entry unless an entry is being restored, and shows which of
// Back and Forward can go.
func (v *Viewer) navSync() {
	n := &v.nav
	if s, ok := v.navSelection(); ok && !n.restoring {
		n.hist.push(s)
	}
	back, fwd := n.hist.can(-1, v.navValid), n.hist.can(1, v.navValid)
	for _, c := range []struct {
		b  *g.ButtonView
		it *g.MenuItem
		on bool
	}{{n.back, n.backItem, back}, {n.fwd, n.fwdItem, fwd}} {
		if c.b != nil && c.b.Enabled() != c.on {
			c.b.SetEnabled(c.on)
		}
		if c.it != nil {
			c.it.SetEnabled(c.on)
		}
	}
}

// navGo shows the nearest valid entry in dir (-1 Back, +1 Forward) without recording it.
func (v *Viewer) navGo(dir int) {
	if v.session == nil || v.activeText != nil {
		return
	}
	s, ok := v.nav.hist.move(dir, v.navValid)
	if !ok {
		return
	}
	e, items, err := v.resolveSelection(s)
	if err != nil {
		return
	}
	v.nav.restoring = true
	v.showSelection(e, slices.Clone(s.Nodes), items, true)
	if !v.sceneMode() && len(items) > 0 {
		v.revealItem(items[0])
	}
	v.nav.restoring = false
	v.navSync()
}
