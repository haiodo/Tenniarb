//go:build darwin || linux || windows

package app

import (
	"path/filepath"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/model"
)

// copyItemHTML is "Export text as html": the text of the selected item on the clipboard as HTML.
func (v *Viewer) copyItemHTML() {
	sel := v.session.Selection()
	if len(sel) != 1 {
		return
	}
	html := v.itemHTML(sel[0])
	if html != "" {
		_ = v.app.Clipboard().SetHTML(html, html)
	}
}

// itemHTML is the text of an item as HTML for the clipboard: the title and the body as the canvas lays
// them out. The old renderer built it from its own text boxes; the scene draws the text and knows it.
func (v *Viewer) itemHTML(item *model.DiagramItem) string {
	if v.sv == nil || v.sv.screen == nil || item == nil {
		return ""
	}
	pl := v.classicPlaced(item)
	if pl == nil || pl.Src == nil {
		return ""
	}
	return v.sv.screen.ItemHTML(pl.Src)
}

// attachImage asks for a picture and adds it to the selected item.
func (v *Viewer) attachImage() {
	paths := v.win.FileDialog(g.FileDialog{
		Title:   "Attach image",
		Filters: []g.FileFilter{{Name: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.gif"}},
	})
	if len(paths) != 1 || v.session == nil {
		return
	}
	data, err := PNGBase64(paths[0])
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.session.AttachImage(filepath.Base(paths[0]), data)
}
