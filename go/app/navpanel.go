//go:build darwin || linux || windows

package app

import (
	"slices"
	"strings"

	g "github.com/haiodo/gowt"
)

// navUI is Back and Forward over what the Code panel showed: the history, the buttons of the panel
// header and the Navigate menu items.
type navUI struct {
	hist      navHistory
	restoring bool // an entry is being shown again: it is not a new one
	back, fwd *g.ButtonView
	path      *g.LabelView
	backItem  *g.MenuItem
	fwdItem   *g.MenuItem
}

// codeHeader is the slim row over the code: Back, Forward and the path of what is shown.
func (v *Viewer) codeHeader() g.Option {
	return g.HBox(g.Padding(g.Scaled(2)), g.Gap(2), g.GrowX(),
		g.Button("‹", g.Flat(), g.Tooltip("Back"), g.Disabled(), g.Ref(&v.nav.back), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { v.navGo(-1) })
		})),
		g.Button("›", g.Flat(), g.Tooltip("Forward"), g.Disabled(), g.Ref(&v.nav.fwd), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { v.navGo(1) })
		})),
		g.Label("", g.GrowX(), g.Ref(&v.nav.path)))
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

// navSync records the selection as an entry unless an entry is being restored, and shows the path and
// which of Back and Forward can go.
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
	if n.path != nil {
		if t := v.navTitle(); n.path.Unwrap().GetText() != t {
			n.path.SetText(t)
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

// navTitle is the path of the selected node as its names, or the selected item; else the element.
func (v *Viewer) navTitle() string {
	e := v.state.Current
	if e == nil {
		return ""
	}
	if v.sceneMode() && len(v.selNodes) > 0 {
		var names []string
		for _, p := range v.sv.rowChain(v.selNodes[0]) {
			names = append(names, v.pathLabel(p))
		}
		return strings.Join(names, " › ")
	}
	if !v.sceneMode() && v.session != nil {
		if sel := v.session.Selection(); len(sel) == 1 && sel[0].Name != "" {
			return sel[0].Name
		}
	}
	return e.Name
}

// pathLabel names the node of a path: as it is written, or as a part of an instance is drawn.
func (v *Viewer) pathLabel(p string) string {
	if n := NodeAt(v.sv.node, p); n != nil {
		return nodeLabel(n)
	}
	if pl := v.sv.byPath[p]; pl != nil && pl.Src != nil {
		return nodeLabel(pl.Src)
	}
	return p[strings.LastIndexAny(p, "./")+1:]
}
