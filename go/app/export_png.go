//go:build darwin || linux || windows

package app

import (
	"bytes"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/editor"
)

// unionRect is the smallest rectangle that holds every rect; false when there are none.
func unionRect(rs []gfx.Rect) (gfx.Rect, bool) {
	if len(rs) == 0 {
		return gfx.Rect{}, false
	}
	u := rs[0]
	for _, r := range rs[1:] {
		u = u.Union(r)
	}
	return u, true
}

// selectionFileName is the default file name of an export of the selected items named names.
func selectionFileName(names []string) string {
	name := ""
	if len(names) > 0 {
		name = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
				return '_'
			}
			return r
		}, strings.TrimSpace(names[0]))
		name = strings.Trim(name, ". ")
	}
	if name == "" {
		name = "Untitled"
	}
	if len(names) > 1 {
		name += "+" + strconv.Itoa(len(names)-1)
	}
	return name + ".png"
}

// renderNodesPNG draws the union of the bounds of nodes, as they are in s, into a PNG. root is the node
// drawn: what is outside its subtree (the selection overlay) stays out of the image. bg is transparent
// for the zero Color.
func renderNodesPNG(s *gfx.Scene, root *gfx.Node, nodes []*gfx.Node, scale float64, bg gfx.Color) ([]byte, error) {
	s.Commit()
	var rects []gfx.Rect
	s.Read(func() {
		for _, n := range nodes {
			if n != nil {
				rects = append(rects, n.SubtreeBounds())
			}
		}
	})
	area, ok := unionRect(rects)
	if !ok {
		return nil, errors.New("nothing to export")
	}
	r, err := offscreen.Default()
	if err != nil {
		return nil, err
	}
	raster, err := gfx.RenderSubtree(r, s, root, area, scale, bg)
	if err != nil {
		return nil, err
	}
	return raster.PNG()
}

// selectionExport is what the canvas holds selected: the nodes to draw, the root to draw them from and the
// names for the file.
type selectionExport struct {
	root  *gfx.Node
	nodes []*gfx.Node
	names []string
}

// selectionToExport collects the selected items of the canvas, the whole drawing when none is selected in a
// scene.
func (v *Viewer) selectionToExport() (selectionExport, bool) {
	var x selectionExport
	switch {
	case v.sceneMode():
		if v.sv.placed == nil || v.sv.placed.Gfx == nil {
			return x, false
		}
		x.root = v.sv.placed.Gfx
		for _, p := range v.selNodes {
			if pl := v.sv.byPath[p]; pl != nil && pl.Gfx != nil {
				x.nodes = append(x.nodes, pl.Gfx)
				name := v.state.Current.Name
				if pl.Src != nil && pl.Src.Name != "" {
					name = pl.Src.Name
				}
				x.names = append(x.names, name)
			}
		}
		if len(x.nodes) == 0 {
			x.nodes, x.names = []*gfx.Node{x.root}, []string{v.state.Current.Name}
		}
	case v.rendered != nil && v.session != nil:
		x.root = v.rendered.Root
		for _, it := range v.session.Selection() {
			if d := v.session.Scene().Drawables[it]; d != nil {
				if n := v.rendered.Nodes[d]; n != nil {
					x.nodes = append(x.nodes, n)
					x.names = append(x.names, it.Name)
				}
			}
		}
	}
	return x, len(x.nodes) > 0
}

// selectionPNG renders the selection with a transparent background; the selection frame is hidden for the
// time of the render.
func (v *Viewer) selectionPNG(x selectionExport, scale int) ([]byte, error) {
	if v.rendered != nil && v.overlay != nil {
		ov := v.overlay
		v.scene.Update(func(tx *gfx.Tx) { tx.SetPicture(ov, nil) })
		defer v.drawOverlay()
	}
	return renderNodesPNG(v.scene, x.root, x.nodes, float64(clampExportScale(scale)), gfx.Color{})
}

// selectionExportMenu adds the selection items to File > Export.
func (v *Viewer) selectionExportMenu(m *g.Menu) {
	m.Separator()
	m.Item("Selection as PNG...", func() { v.exportSelectionPNG(0) }, g.Shortcut("Cmd+Shift+E"))
	sub := m.Submenu("Selection as PNG at Scale")
	for s := minExportScale; s <= maxExportScale; s++ {
		sub.Item(strconv.Itoa(s)+"x...", func() { v.exportSelectionPNG(s) })
	}
}

// exportSelectionPNG asks for a file and writes the selection into it, one PNG for the union of the bounds.
// scale 0 takes the last used one; any other scale is remembered in the settings.
func (v *Viewer) exportSelectionPNG(scale int) {
	x, ok := v.selectionToExport()
	if !ok {
		v.setBanner("Select items to export")
		return
	}
	if scale == 0 {
		scale = v.sh.prefs.ExportSelScale
	} else if scale != v.sh.prefs.ExportSelScale {
		p := v.sh.prefs
		p.ExportSelScale = scale
		v.sh.setPrefs(p)
	}
	dir := ""
	if p := v.state.Doc.Path; p != "" {
		dir = filepath.Dir(p)
	}
	paths := v.win.FileDialog(g.FileDialog{
		Title: "Export selection as PNG", Dir: dir, Name: selectionFileName(x.names), Save: true,
		Filters: []g.FileFilter{{Name: "PNG", Pattern: "*.png"}},
	})
	if len(paths) != 1 {
		return
	}
	path := paths[0]
	if filepath.Ext(path) == "" {
		path += ".png"
	}
	png, err := v.selectionPNG(x, scale)
	if err == nil {
		err = WriteFileAtomic(path, png)
	}
	if err != nil {
		v.win.MessageBox(g.Message{Title: "Export failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
	}
}

// copySelectionPNG puts the selection on the clipboard as an image.
func (v *Viewer) copySelectionPNG() {
	x, ok := v.selectionToExport()
	if !ok {
		v.setBanner("Select items to copy")
		return
	}
	png, err := v.selectionPNG(x, v.sh.prefs.ExportSelScale)
	var img *g.Image
	if err == nil {
		img, err = v.app.ImageFrom(bytes.NewReader(png))
	}
	if err == nil {
		defer img.Dispose()
		err = v.app.Clipboard().SetImage(img)
	}
	if err != nil {
		v.win.MessageBox(g.Message{Title: "Copy failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
	}
}

// exportEntries are the context menu entries of the selection export.
func (v *Viewer) exportEntries() []editor.Entry {
	if _, ok := v.selectionToExport(); !ok {
		return nil
	}
	return []editor.Entry{
		{Sep: true},
		{Label: "Export as PNG...", Run: func() { v.exportSelectionPNG(0) }},
		{Label: "Copy as PNG", Run: v.copySelectionPNG},
	}
}
