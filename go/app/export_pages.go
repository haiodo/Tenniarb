//go:build darwin || linux || windows

package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"
	"github.com/haiodo/gowt/rx"

	"github.com/haiodo/tenniarb/go/model"
)

// exportProgress is the running pages export the toolbar shows: the pages written of the pages to
// write, and the file they go to.
type exportProgress struct {
	active bool
	done   int
	total  int
	dest   string
	text   *rx.State[string] // the text of the toolbar button, "3/12"
}

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
	v.runPagesExport(pages, path)
}

// runPagesExport writes pages into path in the background, so the window keeps drawing: a toolbar item
// counts the pages written from the second page on, and the export says nothing once it is done.
func (v *Viewer) runPagesExport(pages []*model.Element, path string) {
	// shortcut: the export draws the tree of the window from its goroutine; an edit made while it runs
	// is a race, lock the tree if that ever shows.
	v.exporting = exportProgress{total: len(pages), dest: path}
	if len(pages) > 1 {
		v.exporting.active, v.exporting.text = true, rx.New("")
		v.setExportDone(0)
		v.toolbar()
	}
	var written int
	var skipped []string
	v.app.Job("Export pages as PDF", func(context.Context, *g.JobProgress) error {
		var buf bytes.Buffer
		var err error
		written, skipped, err = v.writePagesPDF(&buf, pages, func(n int) {
			v.app.Async(func() { v.setExportDone(n) }) // rx.Set belongs to the UI thread
		})
		if err == nil && written > 0 {
			err = WriteFileAtomic(path, buf.Bytes())
		}
		return err
	}).Then(func(err error) {
		v.endExport()
		if v.win.Unwrap().IsDisposed() {
			return
		}
		if err != nil {
			v.win.MessageBox(g.Message{Title: "Export failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
			return
		}
		v.reportPages("Export pages as PDF", path, written, len(pages), skipped)
	})
}

// setExportDone is the count on the toolbar button, on the UI thread.
func (v *Viewer) setExportDone(n int) {
	if v.exporting.text == nil {
		return
	}
	v.exporting.done = n
	v.exporting.text.Set(fmt.Sprintf("%d/%d", n, v.exporting.total))
}

// endExport takes the progress item off the toolbar.
func (v *Viewer) endExport() {
	if !v.exporting.active {
		return
	}
	v.exporting.active = false
	v.closePopover()
	if !v.win.Unwrap().IsDisposed() {
		v.toolbar()
	}
}

// showExportProgress is the popover of the toolbar progress button: how much of the export is left, and
// where it goes.
func (v *Viewer) showExportProgress() {
	if !v.exporting.active {
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4), g.Padding(6))
	p.Add(g.Label(fmt.Sprintf("Export pages: %d of %d written, %d left",
		v.exporting.done, v.exporting.total, v.exporting.total-v.exporting.done)))
	p.Add(g.Label(v.exporting.dest, faint()))
	v.openPopover(p)
	p.ShowAt(v.view.Canvas(), g.PopoverBelow)
}

// writePagesPDF writes one page per element into w. gfx.WriteSubtreePDF makes a document of one page,
// so the pages are written through the document of the renderer: NewPage starts each one. onPage counts
// the pages written so far.
func (v *Viewer) writePagesPDF(w io.Writer, pages []*model.Element, onPage func(written int)) (int, []string, error) {
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
		onPage(written)
	}
	return written, skipped, doc.Close()
}

// reportPages says a finished export in the banner: the one line a skip needs. A file with no page at
// all is a failure, and only that is a box.
func (v *Viewer) reportPages(title, dest string, written, total int, skipped []string) {
	if written == 0 {
		v.win.MessageBox(g.Message{Title: title, Text: "No pages written to " + dest, Icon: g.IconError, Buttons: g.ButtonsOK})
		return
	}
	if len(skipped) > 0 {
		v.setNote(fmt.Sprintf("%d of %d pages written to %s, skipped: %s", written, total, dest, strings.Join(skipped, ", ")))
	}
}
