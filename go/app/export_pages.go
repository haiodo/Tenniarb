//go:build darwin || linux || windows

package app

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/model"
)

// pagesOf is e and every element inside it, in tree order: one page of the pages export each.
func pagesOf(e *model.Element) []*model.Element {
	if e == nil {
		return nil
	}
	out := []*model.Element{e}
	for _, c := range e.Elements {
		out = append(out, pagesOf(c)...)
	}
	return out
}

// pageName is a name fit for a file: spaces, slashes, colons and the other separators become _.
func pageName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, strings.TrimSpace(name))
	name = strings.Trim(name, "._")
	if name == "" {
		return "Untitled"
	}
	return name
}

// pageFileName is the file of page i of n: 01-Name.png, the number as wide as n needs.
func pageFileName(i, n int, name, ext string) string {
	w := max(2, len(strconv.Itoa(n)))
	return fmt.Sprintf("%0*d-%s%s", w, i, pageName(name), ext)
}

// pagesToExport is the pages of the current element, or nil when there is nothing to export.
func (v *Viewer) pagesToExport() []*model.Element {
	cur := v.state.Current
	if v.session == nil || cur == nil {
		v.setBanner("Select an element to export")
		return nil
	}
	return pagesOf(cur)
}

// exportPagesPNG asks for a folder and writes the current element and the elements inside it into it,
// one PNG per page.
func (v *Viewer) exportPagesPNG() {
	pages := v.pagesToExport()
	if pages == nil {
		return
	}
	dir := ""
	if p := v.state.Doc.Path; p != "" {
		dir = filepath.Dir(p)
	}
	dir, ok := v.win.DirDialog("Export pages as PNG", dir)
	if !ok {
		return
	}
	p := v.sh.prefs
	scale := v.exportScale()
	written := 0
	var skipped []string
	for i, e := range pages {
		data, err := v.exportPNGOf(e, p, scale)
		if err == nil {
			err = WriteFileAtomic(filepath.Join(dir, pageFileName(i+1, len(pages), e.Name, ".png")), data)
		}
		if err != nil {
			skipped = append(skipped, e.Name)
			continue
		}
		written++
	}
	v.reportPages("Export pages as PNG", dir, written, len(pages), skipped)
}

// exportPagesPDF asks for a file and writes the current element and the elements inside it into it, one
// page per element.
func (v *Viewer) exportPagesPDF() {
	pages := v.pagesToExport()
	if pages == nil {
		return
	}
	dir := ""
	if p := v.state.Doc.Path; p != "" {
		dir = filepath.Dir(p)
	}
	paths := v.win.FileDialog(g.FileDialog{
		Title: "Export pages as PDF", Dir: dir, Name: ExportName(v.state.Current, ".pdf"), Save: true,
		Filters: []g.FileFilter{{Name: "PDF", Pattern: "*.pdf"}},
	})
	if len(paths) != 1 {
		return
	}
	path := paths[0]
	if filepath.Ext(path) == "" {
		path += ".pdf"
	}
	var buf bytes.Buffer
	written, skipped, err := v.writePagesPDF(&buf, pages)
	if err == nil && written > 0 {
		err = WriteFileAtomic(path, buf.Bytes())
	}
	if err != nil {
		v.win.MessageBox(g.Message{Title: "Export failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
		return
	}
	v.reportPages("Export pages as PDF", path, written, len(pages), skipped)
}

// writePagesPDF writes one page per element into w. gfx.WriteSubtreePDF makes a document of one page,
// so the pages are written through the document of the renderer: NewPage starts each one.
func (v *Viewer) writePagesPDF(w io.Writer, pages []*model.Element) (int, []string, error) {
	doc, err := offscreen.NewPDF(w)
	if err != nil {
		return 0, nil, err
	}
	bg := v.sh.prefs.exportBackground()
	written := 0
	var skipped []string
	for _, e := range pages {
		d, err := v.exportDrawing(e)
		if err != nil {
			skipped = append(skipped, e.Name)
			continue
		}
		l, err := gfx.PDFOptions{Background: bg}.Layout(d.area)
		if err != nil {
			skipped = append(skipped, e.Name)
			continue
		}
		var items gfx.Items
		d.scene.CollectSubtree(d.root, d.area, &items)
		doc.NewPage(l.W, l.H).Draw(l.Scale, l.Origin, gfx.ClipItems(items.List, d.area, bg))
		written++
	}
	return written, skipped, doc.Close()
}

// reportPages tells the user how many pages were written and which elements were skipped.
func (v *Viewer) reportPages(title, dest string, written, total int, skipped []string) {
	noun := "pages"
	if written == 1 {
		noun = "page"
	}
	done := fmt.Sprintf("%d %s written to %s", written, noun, dest)
	if len(skipped) > 0 {
		done += fmt.Sprintf("\n\nSkipped %d of %d: %s", len(skipped), total, strings.Join(skipped, ", "))
	}
	icon := g.IconInfo
	if written == 0 {
		icon, title = g.IconError, "Export failed"
	}
	v.win.MessageBox(g.Message{Title: title, Text: done, Icon: icon, Buttons: g.ButtonsOK})
}
