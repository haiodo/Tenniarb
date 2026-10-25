//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/render"
)

// registerQA exposes the application to the QA driver (build with -tags gowtqa); without the tag Handle
// does nothing. The handlers act on the active window.
func (sh *Shell) registerQA() {
	qa.HandleValue("state", func(qa.Args) (any, error) { return sh.qaState(), nil })
	qa.Handle("zoom", func(a qa.Args) error {
		v := sh.active
		f := 1.0
		if a.Has("factor") {
			if err := a.Decode("factor", &f); err != nil {
				return err
			}
		}
		v.zoomBy(f)
		return nil
	})
	qa.Handle("reset-zoom", func(qa.Args) error { sh.active.resetZoom(); return nil })
	qa.Handle("fit", func(qa.Args) error { sh.active.fitView(); return nil })
	qa.Handle("pan", func(a qa.Args) error {
		v := sh.active
		var dx, dy float64
		if err := a.Decode("dx", &dx); err != nil {
			return err
		}
		if err := a.Decode("dy", &dy); err != nil {
			return err
		}
		v.view.Pan(dx, dy)
		return nil
	})
	// command: what the QA driver cannot reach: the application menu (quit), the outline drag, and
	// closing the windows a test is done with (the question of a dirty document is skipped).
	qa.Handle("command", func(a qa.Args) error {
		switch a.String("name") {
		case "drop":
			return sh.active.qaDrop(a)
		case "quit":
			sh.app.Unwrap().Close()
			return nil
		case "open-document":
			// what the system sends for a double click in Finder, through the display's event
			e := swt.NewEvent()
			e.Text = a.String("path")
			sh.app.Unwrap().SendEvent(swt.OpenDocument, e)
			return nil
		case "close-others":
			for _, v := range slices.Clone(sh.viewers) {
				if v != sh.active {
					v.force = true
					v.win.Close()
				}
			}
			return nil
		}
		return fmt.Errorf("no command %q", a.String("name"))
	})
}

// qaState is the state of the active window with what the application keeps for all of them.
func (sh *Shell) qaState() map[string]any {
	v := sh.active
	out := v.qaState()
	out["windows"] = len(sh.viewers)
	out["recent"] = sh.recent
	out["prefs"] = sh.prefs
	out["configDir"] = sh.dir
	out["untitled"] = v.state.Doc.Path == ""
	out["stash"] = v.id
	out["prefsOpen"] = sh.prefsWin != nil
	out["helpOpen"] = sh.helpWin != nil
	b := v.win.Unwrap().GetBounds()
	out["frame"] = []int{int(b.X), int(b.Y), int(b.Width), int(b.Height)}
	out["scale"] = v.exportScale()
	out["autosavePending"] = v.autosave.Pending()
	return out
}

func (v *Viewer) qaState() map[string]any {
	cam := v.view.Compositor().Camera()
	w, h := v.view.Canvas().Size()
	out := map[string]any{
		"path": v.state.Doc.Path, "error": v.state.Error, "canvas": []int{w, h},
		"zoom": cam.Zoom, "originX": cam.Origin.X, "originY": cam.Origin.Y,
		"dirty": v.isDirty(), "title": Title(v.state.Doc.Path, v.isDirty()),
		"inline": v.inline != nil, "popover": v.popover != nil, "search": v.search.box.Unwrap().GetVisible(),
	}
	if v.props.text != nil {
		st := v.props.text
		out["props"], out["propsValues"], out["propsStyles"] = st.Text(), v.props.values, len(st.Styles())
		out["propsFocus"] = v.propsFocused()
	}
	// outline: "parent/child" paths in row order; rows: name -> [x, y, width, height] in the outline's coordinates
	outline, rows := []string{}, map[string][]int{}
	var walk func(ns []*g.Node, prefix string)
	walk = func(ns []*g.Node, prefix string) {
		for _, n := range ns {
			path := prefix + n.Text()
			outline = append(outline, path)
			b := n.Unwrap().GetBounds()
			rows[n.Text()] = []int{int(b.X), int(b.Y), int(b.Width), int(b.Height)}
			walk(n.Children(), path+"/")
		}
	}
	walk(v.tree.Roots(), "")
	out["outline"], out["rows"] = outline, rows
	if v.state.Current != nil {
		out["element"] = PathOf(v.state.Current)
	}
	if v.rendered != nil {
		out["size"] = []float64{v.rendered.Width, v.rendered.Height}
	}
	if v.session != nil {
		out["canUndo"], out["canRedo"] = v.session.UndoManager().CanUndo(), v.session.UndoManager().CanRedo()
		var sel []string
		for _, i := range v.session.Selection() {
			sel = append(sel, i.Name)
		}
		out["selection"] = sel
		var links []string
		for _, i := range v.session.Scene().Items() {
			if i.IsLink() && i.Source != nil && i.Target != nil {
				links = append(links, i.Source.Name+" -> "+i.Target.Name)
			}
		}
		out["links"] = links
		// items: name -> [x, y, width, height] in the editor scene
		items := map[string][]float64{}
		for _, i := range v.session.Scene().Items() {
			if d := v.session.Scene().Drawables[i]; d != nil && i.Name != "" {
				b := d.Bounds()
				items[i.Name] = []float64{b.X, b.Y, b.Width, b.Height}
			}
		}
		out["items"] = items
		out["itemCount"] = len(v.session.Scene().Items())
		// screen: name -> [x, y, width, height] in view points, the place of the item in the window
		screen := map[string][]float64{}
		for name, b := range items {
			r := v.smap.RectToGfx(render.Rect{X: b[0], Y: b[1], Width: b[2], Height: b[3]})
			screen[name] = []float64{(r.Min.X - cam.Origin.X) * cam.Zoom, (r.Min.Y - cam.Origin.Y) * cam.Zoom, (r.Max.X - r.Min.X) * cam.Zoom, (r.Max.Y - r.Min.Y) * cam.Zoom}
		}
		out["screen"] = screen
		out["pivot"] = []float64{v.session.Pivot().X, v.session.Pivot().Y}
	}
	return out
}

// qaDrop ends a drag of the outline row "from" at (x, y): the QA pointer events cannot start the
// native drag session, so the drop handler is called as the Tree's DropTarget calls it.
func (v *Viewer) qaDrop(a qa.Args) error {
	var x, y int
	if err := a.Decode("x", &x); err != nil {
		return err
	}
	if err := a.Decode("y", &y); err != nil {
		return err
	}
	n := v.findNode(a.String("from"))
	if n == nil {
		return fmt.Errorf("no outline row %q", a.String("from"))
	}
	v.dragging, v.dropAt = elementOf(n), [2]int{x, y}
	v.dropElement()
	return nil
}
