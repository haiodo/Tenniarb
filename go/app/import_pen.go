//go:build darwin || linux || windows

package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/penimport"
)

var penFilters = []g.FileFilter{{Name: "Pen", Pattern: "*.pen"}}

// importPenDialog is File > Import Pen File: asks for .pen files and imports them.
func (v *Viewer) importPenDialog() {
	dir := ""
	if p := v.state.Doc.Path; p != "" {
		dir = filepath.Dir(p)
	}
	paths := v.win.FileDialog(g.FileDialog{Title: "Import Pen File", Dir: dir, Multi: true, Filters: penFilters})
	if len(paths) > 0 {
		v.importPen(paths)
	}
}

// importPen converts the .pen files of paths next to the first of them, off the UI thread, and opens
// the .tenn written from that first file.
func (v *Viewer) importPen(paths []string) {
	paths = slices.Clone(paths)
	go func() {
		main, notes, n, err := penImport(paths)
		v.app.Async(func() {
			if err != nil {
				v.win.MessageBox(g.Message{Title: "Import failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
				return
			}
			if n > 0 {
				v.win.MessageBox(g.Message{Title: "Imported with notes", Icon: g.IconInfo, Buttons: g.ButtonsOK,
					Text: fmt.Sprintf("%d notes:\n\n%s", n, notes)})
			}
			v.sh.openPath(main)
		})
	}()
}

// penImport converts paths and writes the .tenn files next to them. It returns the file written from
// the first of paths, the notes of its import (n of them) and the error that stopped it.
func penImport(paths []string) (main, notes string, n int, err error) {
	abs, err := filepath.Abs(paths[0])
	if err != nil {
		return "", "", 0, err
	}
	res, err := penimport.Convert(paths, penimport.Options{})
	if err != nil {
		return "", "", 0, err
	}
	if err := res.Write(filepath.Dir(abs)); err != nil {
		return "", "", 0, err
	}
	for _, f := range res.Files {
		if f.Path != abs {
			continue
		}
		notes, n = penNotes(f, 8)
		return filepath.Join(filepath.Dir(abs), f.Name+".tenn"), notes, n, nil
	}
	return "", "", 0, fmt.Errorf("%s: no output written", paths[0])
}

// penNotes is what the conversion of f left out, one line per warning and per unsupported feature,
// the first limit of them; n is how many there are.
func penNotes(f *penimport.File, limit int) (text string, n int) {
	lines := slices.Clone(f.Warnings)
	keys := make([]string, 0, len(f.Unsupported))
	for k := range f.Unsupported {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s (%d)", k, f.Unsupported[k]))
	}
	n = len(lines)
	if n > limit {
		lines = append(lines[:limit], fmt.Sprintf("and %d more", n-limit))
	}
	return strings.Join(lines, "\n"), n
}
