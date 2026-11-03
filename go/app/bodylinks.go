//go:build darwin || linux || windows

package app

import (
	"fmt"
	"path/filepath"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// bodyLinkAt is the link of a body under the content point p, false on plain text.
func (sv *sceneView) bodyLinkAt(p gfx.Point) (scene.BodyLink, bool) {
	if sv.screen == nil {
		return scene.BodyLink{}, false
	}
	for _, l := range sv.screen.BodyLinks() {
		if p.X >= l.X && p.Y >= l.Y && p.X <= l.X+l.W && p.Y <= l.Y+l.H {
			return l, true
		}
	}
	return scene.BodyLink{}, false
}

// openBodyLink goes where a link of a body points, from the content point at: this text scrolls to the
// heading, another file opens on the element whose body has it, and a missing file or heading shows in the
// banner.
func (v *Viewer) openBodyLink(l scene.BodyLink, at gfx.Point) {
	if l.File == "" {
		if v.revealBodyAnchor(l.Anchor) {
			return
		}
		// The heading is outside the section this page shows: the start of the body it is in.
		c := gfx.Point{X: l.X + v.sv.off.X, Y: l.Y + v.sv.off.Y}
		if pl := v.sv.hitScene(at); pl != nil {
			c = gfx.Point{X: pl.X + v.sv.off.X, Y: pl.Y + v.sv.off.Y}
		}
		v.centerScenePoint(c)
		return
	}
	abs := filepath.Join(v.root, filepath.FromSlash(l.File))
	if !isFile(abs) {
		v.notice("no such file " + l.File)
		return
	}
	v.sh.openPath(abs)
	other, err := v.sh.viewer(abs)
	if err != nil {
		v.notice("Could not open " + l.File)
		return
	}
	if l.Anchor == "" {
		return // a link to a file alone shows it, as Open does
	}
	var path []string
	var ok bool
	if other.session != nil && other.stree != nil {
		path, ok = other.stree.AnchorElement(l.Anchor)
	}
	if !ok {
		other.notice(fmt.Sprintf("no section %q in %s", l.Anchor, l.File))
		return
	}
	e := Resolve(&other.session.Root().Element, path)
	if e == nil {
		other.notice(fmt.Sprintf("no section %q in %s", l.Anchor, l.File))
		return
	}
	other.showSelection(e, nil, nil, true)
	other.revealBodyAnchor(l.Anchor)
}

// revealBodyAnchor centers the canvas on the heading with that anchor when the screen has the body with
// it; false when no body of the screen shows it.
func (v *Viewer) revealBodyAnchor(anchor string) bool {
	if !v.sceneMode() || v.sv.screen == nil {
		return false
	}
	for _, a := range v.sv.screen.BodyAnchors() {
		if a.Anchor == anchor {
			v.centerScenePoint(gfx.Point{X: a.X + v.sv.off.X, Y: a.Y + v.sv.off.Y})
			return true
		}
	}
	return false
}

// centerScenePoint puts the content point c in the middle of the canvas.
func (v *Viewer) centerScenePoint(c gfx.Point) {
	w, h := v.view.Canvas().Size()
	v.setCamera(CenterOn(v.view.Compositor().Camera(), c, float64(w), float64(h)))
}
