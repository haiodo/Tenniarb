//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"slices"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/look"
	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/update"
)

// registerQA exposes the application to the QA driver (build with -tags gowtqa); without the tag Handle
// does nothing. The handlers act on the active window.
func (sh *Shell) registerQA() {
	qa.HandleValue("state", func(qa.Args) (any, error) { return sh.qaState(), nil })
	qa.HandleValue("text", func(qa.Args) (any, error) { return sh.active.text, nil })
	// scene: the selectable nodes of the scene element on the canvas, path -> [x, y, w, h] in view points
	qa.HandleValue("scene", func(qa.Args) (any, error) { return sh.active.qaScene(), nil })
	// doc: a window by the path of its document, active or not
	qa.HandleValue("doc", func(a qa.Args) (any, error) {
		v, err := sh.viewer(a.String("path"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"version": v.version, "builds": v.sceneBuilds, "sceneMode": v.sceneMode(),
			"nodes": v.selNodes, "dirty": v.isDirty(), "edited": v.edited(), "error": v.bannerText(), "kind": v.kind,
			"title": Title(v.state.Doc.Path, v.edited()), "agent": v.agent.statusText(), "follow": v.agent.follow}, nil
	})
	// select: what the MCP service will do, for the window of path
	qa.Handle("select", func(a qa.Args) error {
		v, err := sh.viewer(a.String("path"))
		if err != nil {
			return err
		}
		var sel Selection
		if a.Has("element") {
			if err := a.Decode("element", &sel.Element); err != nil {
				return err
			}
		}
		if err := a.Decode("nodes", &sel.Nodes); err != nil {
			return err
		}
		return v.selectAPI(sel)
	})
	// edit: what an agent's edit of the text does to the window of path
	qa.Handle("edit", func(a qa.Args) error {
		var e TextEdit
		if err := a.Decode("start", &e.Start); err != nil {
			return err
		}
		if err := a.Decode("end", &e.End); err != nil {
			return err
		}
		e.New = a.String("new")
		_, _, err := sh.ApplyEdit(a.String("path"), 0, e)
		return err
	})
	qa.Handle("activate", func(a qa.Args) error {
		v, err := sh.viewer(a.String("path"))
		if err != nil {
			return err
		}
		v.win.Unwrap().ForceActive()
		sh.active = v
		return nil
	})
	// style: the choice of a style group of the inspector, which the QA driver cannot make as a click on
	// a segment of a segmented control
	// anim: the playback of the scene animations, paused or resumed, and the frame a scrub shows
	qa.Handle("anim", func(a qa.Args) error {
		v := sh.active
		if a.Has("play") {
			on := a.Bool("play")
			if on {
				v.animPlay()
			} else {
				v.animOff = true
				v.stopAnim()
			}
		}
		if a.Has("at") {
			var at float64
			if err := a.Decode("at", &at); err != nil {
				return err
			}
			v.animScrub(at)
		}
		return nil
	})
	qa.Handle("style", func(a qa.Args) error {
		return sh.active.pressStyle(a.String("group"), a.String("value"))
	})
	// param: the choice of a segmented parameter of the Parameters section, which the QA driver cannot make
	// as a click on a segment of a segmented control
	qa.Handle("param", func(a qa.Args) error {
		return sh.active.pressParam(a.String("name"), a.String("value"))
	})
	// update-check: the check of Help > Check for Updates, against "url" when a test gives one; the
	// answer is in the "update" state, and no dialog is shown.
	qa.Handle("update-check", func(a qa.Args) error {
		if api := a.String("url"); api != "" {
			updateAPI = api
		}
		o := sh.updOptions()
		o.Force = true // a QA check always asks its server; the menu path is what tests the cache
		r, err := update.Latest(sh.ctx, o)
		sh.setUpdate(r, err)
		return nil
	})
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
		v.panView(dx, dy)
		return nil
	})
	// command: what the QA driver cannot reach: the application menu (quit, preferences, about), and
	// closing the windows a test is done with (the question of a dirty document is skipped).
	qa.Handle("command", func(a qa.Args) error {
		switch a.String("name") {
		case "undo":
			sh.active.cmdUndo()
			return nil
		case "preferences":
			sh.showPrefs()
			return nil
		case "about":
			sh.showAbout()
			return nil
		case "quit":
			sh.app.Unwrap().Close()
			return nil
		case "open-document":
			// what the system sends for a double click in Finder, through the display's event
			e := swt.NewEvent()
			e.Text = a.String("path")
			sh.app.Unwrap().SendEvent(swt.OpenDocument, e)
			return nil
		case "toolbar":
			switch a.String("item") {
			case "sidebar":
				sh.active.toggleSidebar()
			case "inspector":
				sh.active.toggleInspector()
			case "code":
				sh.active.toggleCode()
			case "items":
				sh.active.toggleItems()
			case "fit":
				sh.active.fitCommand()
			case "follow":
				sh.active.toggleFollow()
			}
			return nil
		case "appearance":
			if a.String("mode") == "dark" {
				sh.app.SetAppearance(g.AppearanceDark)
			} else {
				sh.app.SetAppearance(g.AppearanceLight)
			}
			return nil
		case "sidebar-width":
			var w int
			if err := a.Decode("width", &w); err != nil {
				return err
			}
			look.SetSidebarWidth(sh.active.win, w)
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

// treeCols are the column widths of a tree in points: a test reads them to see a name column that does
// not fill the tree.
func treeCols(t *g.TreeView) []int {
	var w []int
	for _, c := range t.Unwrap().GetColumns() {
		w = append(w, int(c.GetWidth()))
	}
	return w
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
	out["updateOpen"] = sh.updWin != nil
	out["update"] = sh.update
	b := v.win.Unwrap().GetBounds()
	out["frame"] = []int{int(b.X), int(b.Y), int(b.Width), int(b.Height)}
	out["scale"] = v.exportScale()
	out["export"] = map[string]any{"active": v.exporting.active, "done": v.exporting.done, "total": v.exporting.total, "dest": v.exporting.dest}
	out["autosavePending"] = v.autosave.Pending()
	return out
}

func (v *Viewer) qaState() map[string]any {
	cam := v.view.Compositor().Camera()
	w, h := v.view.Canvas().Size()
	out := map[string]any{
		"path": v.state.Doc.Path, "error": v.state.Error, "canvas": []int{w, h},
		"zoom": cam.Zoom, "originX": cam.Origin.X, "originY": cam.Origin.Y,
		"dirty": v.isDirty(), "edited": v.edited(), "title": Title(v.state.Doc.Path, v.edited()),
		"sidebar": look.SidebarVisible(v.win), "inspector": look.InspectorVisible(v.win), "code": look.BottomPanelVisible(v.win),
		"inline": v.inline != nil, "popover": v.popover != nil, "gotoAsked": v.search.asked,
	}
	out["banner"] = v.banner.Unwrap().GetText()
	if v.itemsUI.hover != nil && v.itemsUI.addCol != nil {
		b := v.itemsUI.hover.Unwrap().GetBounds()
		out["itemsAdd"] = []int{v.itemsUI.addX, int(b.Y + b.Height/2)}
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
	if sel := v.tree.Selected(); len(sel) == 1 {
		out["outlineSelected"] = sel[0].Text()
	}
	out["outlineCols"], out["outlineClient"] = treeCols(v.tree), int(v.tree.Unwrap().GetClientArea().Width)
	if v.state.Current != nil {
		out["element"] = PathOf(v.state.Current)
	}
	if v.rendered != nil {
		out["size"] = []float64{v.rendered.Width, v.rendered.Height}
	}
	out["kind"], out["version"], out["sceneMode"], out["nodes"] = v.kind, v.version, v.sceneMode(), v.selNodes
	out["indicators"] = v.ind.dots
	// bodyLinks: the links of the bodies on the canvas, in view points, for a test that clicks one
	out["bodyLinks"] = v.qaBodyLinks()
	// marks: the agent marks and the pulses on the canvas, the path of the node and the time left in ms
	out["marks"] = v.qaMarks()
	out["compare"] = v.qaCompare()
	// suggest: the findings the list shows, the last preview and the version the list was built from
	out["suggest"] = map[string]any{"findings": len(v.sug.findings), "preview": v.sug.preview, "version": v.sug.version}
	n := 0
	if v.animScreen != nil {
		n = len(v.animScreen.AnimatedNodes())
	}
	hasSV, hasScreen := v.sv != nil, v.sv != nil && v.sv.screen != nil
	if hasScreen {
		n = len(v.sv.screen.AnimatedNodes())
	}
	out["anim"] = map[string]any{"animated": n, "ticks": v.animTicks, "sv": hasSV, "screen": hasScreen,
		"ms": math.Round(v.animMS*100) / 100, "paused": v.animOff}
	// itemList: the rows of the Items list made so far, as the outline; itemRows: text -> path, kind, bounds
	items, itemRows := []string{}, map[string]map[string]any{}
	var walkItems func(ns []*g.Node, prefix string)
	walkItems = func(ns []*g.Node, prefix string) {
		for _, n := range ns {
			if n.Text() == "" {
				continue
			}
			items = append(items, prefix+n.Text())
			b := n.Unwrap().GetBounds()
			row := map[string]any{"box": []int{int(b.X), int(b.Y), int(b.Width), int(b.Height)}}
			if r, ok := n.Data().(*ItemRow); ok {
				row["path"], row["kind"], row["kids"] = r.Path, r.Kind, r.Kids
			}
			itemRows[n.Text()] = row
			walkItems(n.Children(), prefix+n.Text()+"/")
		}
	}
	walkItems(v.itemsUI.tree.Roots(), "")
	out["itemList"], out["itemRows"] = items, itemRows
	out["itemCols"], out["itemClient"] = treeCols(v.itemsUI.tree), int(v.itemsUI.tree.Unwrap().GetClientArea().Width)
	if sel := v.itemsUI.tree.Selected(); len(sel) == 1 {
		out["itemSelected"] = sel[0].Text()
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

// qaNode is a selectable scene node: its box in view points, its kind and how many nodes it has.
type qaNode struct {
	Box  []float64
	Kind string
	Kids int
}

// qaBodyLink is a link of a body in view points: where its label is and where it leads.
type qaBodyLink struct {
	Box    []float64
	File   string
	Anchor string
}

// qaBodyLinks are the links of the bodies of the scene element on the canvas, in view points.
func (v *Viewer) qaBodyLinks() []qaBodyLink {
	if !v.sceneMode() || v.sv.screen == nil {
		return nil
	}
	cam := v.view.Compositor().Camera()
	var out []qaBodyLink
	for _, l := range v.sv.screen.BodyLinks() {
		x, y := (l.X+v.sv.off.X-cam.Origin.X)*cam.Zoom, (l.Y+v.sv.off.Y-cam.Origin.Y)*cam.Zoom
		out = append(out, qaBodyLink{[]float64{x, y, l.W * cam.Zoom, l.H * cam.Zoom}, l.File, l.Anchor})
	}
	return out
}

func (v *Viewer) qaScene() map[string]qaNode {
	out := map[string]qaNode{}
	if !v.sceneMode() {
		return out
	}
	cam := v.view.Compositor().Camera()
	for p, pl := range v.sv.byPath {
		x, y := (pl.X+v.sv.off.X-cam.Origin.X)*cam.Zoom, (pl.Y+v.sv.off.Y-cam.Origin.Y)*cam.Zoom
		out[p] = qaNode{[]float64{x, y, pl.W * cam.Zoom, pl.H * cam.Zoom}, pl.Src.Kind.String(), len(sceneKids(pl.Src))}
	}
	return out
}
