//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"

	g "github.com/haiodo/gowt"
)

type exportKind struct {
	label, title, ext, filter string
}

var (
	exportPNG  = exportKind{"Export as &PNG...", "Export diagram as PNG", ".png", "*.png"}
	exportSVG  = exportKind{"Export as &SVG...", "Export diagram as SVG", ".svg", "*.svg"}
	exportHTML = exportKind{"Export as &HTML...", "Export diagram as HTML with embedded image", ".html", "*.html"}
	exportLive = exportKind{"Export as &interactive HTML...", "Export diagram as interactive HTML", ".html", "*.html"}
)

// exportMenu fills File > Export. Swift also copies to the clipboard and writes JSON, tenn and PDF;
// those are not ported, see the report of stage 8.
func (v *Viewer) exportMenu(m *g.Menu) {
	for _, k := range []exportKind{exportPNG, exportSVG, exportHTML, exportLive} {
		m.Item(k.label, func() { v.export(k) })
	}
}

// exportScale is 1 unless the settings ask for the screen resolution, as Swift's renderUseNativeResolution.
func (v *Viewer) exportScale() float64 {
	if !v.sh.prefs.ExportNativeScale {
		return 1
	}
	if m := v.win.Unwrap().GetMonitor(); m != nil && m.GetZoom() > 100 {
		return float64(m.GetZoom()) / 100
	}
	return 1
}

// export asks for a file and writes the element on the canvas into it.
func (v *Viewer) export(k exportKind) {
	if v.session == nil || v.state.Current == nil || v.state.Current.Parent == nil {
		v.setBanner("Select an element to export")
		return
	}
	dir := ""
	if p := v.state.Doc.Path; p != "" {
		dir = filepath.Dir(p)
	}
	paths := v.win.FileDialog(g.FileDialog{
		Title: k.title, Dir: dir, Name: ExportName(v.state.Current, k.ext), Save: true,
		Filters: []g.FileFilter{{Name: strings.ToUpper(k.ext[1:]), Pattern: k.filter}},
	})
	if len(paths) != 1 {
		return
	}
	path := paths[0]
	if filepath.Ext(path) == "" {
		path += k.ext
	}
	if err := v.exportTo(k, path); err != nil {
		v.win.MessageBox(g.Message{Title: "Export failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
	}
}

func (v *Viewer) exportTo(k exportKind, path string) error {
	e, p := v.state.Current, v.sh.prefs
	var data []byte
	var err error
	switch k {
	case exportPNG:
		data, err = ExportPNG(e, p, v.exportScale())
	case exportSVG:
		data, err = ExportSVG(e, p)
	case exportHTML:
		data, err = ExportHTML(e, p, v.exportScale())
	case exportLive:
		var bundle string
		if bundle, err = v.embedBundle(); err == nil {
			data = InteractiveHTML(bundle, v.text, e)
		}
	}
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data)
}

func (v *Viewer) embedBundle() (string, error) {
	path, err := FindEmbedBundle(v.sh.exe, v.sh.dir)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}
