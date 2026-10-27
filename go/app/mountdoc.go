package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Edits inside an element with --from-file go to the file it shows. The document keeps the text of
// such a file until it is saved, like its own text, and the poll of the window writes it atomically.

// libDoc is the text the document holds for a file that its elements mount, and whether it is saved.
type libDoc struct {
	text    string
	tracker *Tracker
	stamp   Stamp // of the file when the first unsaved edit was made: a later change of it is not ours
}

// mountDoc pairs a mounting element with its file: shape is the element tree as the text of the file
// holds it, so a classic edit can be spliced into that text.
type mountDoc struct {
	file    string // relative to the workspace root, with slashes
	shape   shape
	restore restore
}

// peerOf is the document another window holds for the mounted file, nil when none.
func (d *docCore) peerOf(file string) *docCore {
	if d.peer == nil {
		return nil
	}
	return d.peer(d, file)
}

func (d *docCore) libPath(file string) string {
	return filepath.Join(d.root, filepath.FromSlash(file))
}

// libText is the text of a mounted file now: what another window shows for it, else what this
// document holds, else what the tree read.
func (d *docCore) libText(file string) string {
	if p := d.peerOf(file); p != nil {
		return p.text
	}
	if ld := d.libDocs[file]; ld != nil {
		return ld.text
	}
	if d.stree != nil {
		s, _ := d.stree.Source(file)
		return s
	}
	return ""
}

// overlay is the text the scene tree reads instead of files: the other open documents and the
// unsaved text of the mounted files.
func (d *docCore) overlay() map[string]string {
	out := map[string]string{}
	if d.others != nil {
		out = d.others(d.root)
	}
	for f, ld := range d.libDocs {
		if ld.tracker.Dirty() {
			out[f] = ld.text
		}
	}
	return out
}

func (d *docCore) libDoc(file string) *libDoc {
	ld := d.libDocs[file]
	if ld == nil {
		ld = &libDoc{text: d.libText(file)}
		ld.tracker = NewTracker(ld.text)
		if d.libDocs == nil {
			d.libDocs = map[string]*libDoc{}
		}
		d.libDocs[file] = ld
	}
	return ld
}

// adoptMounts takes the elements MountFiles returns: one that was read anew starts from the text of
// its file, the others keep the shape they have.
func (d *docCore) adoptMounts(ms []model.Mounted) {
	next := map[*model.Element]*mountDoc{}
	for _, m := range ms {
		md := d.mounts[m.El]
		if md == nil || m.Fresh {
			md = &mountDoc{file: m.File, shape: mountShape(m.El)}
			if ld := d.libDocs[m.File]; ld == nil || !ld.tracker.Dirty() {
				delete(d.libDocs, m.File)
			}
		}
		next[m.El] = md
	}
	d.mounts = next
}

// mountWritable: the mounting element shows a file this document can write.
func (d *docCore) mountWritable(m *model.Element) bool { return d.mounts[m] != nil }

// mountEdited runs after a classic edit, undo or redo inside m: the text of its file follows the
// elements. The edited element is not read again from that text.
func (d *docCore) mountEdited(m *model.Element) {
	md := d.mounts[m]
	if md == nil {
		return
	}
	text, ok := spliceText(d.libText(md.file), md.shape, m, &md.restore)
	if !ok {
		text = printMount(m)
	}
	md.shape = mountShape(m)
	d.keep = append(d.keep, m)
	d.libErr = d.setLibText(md.file, text)
}

// printMount is the whole text of the content of m, when the old text no longer pairs with it.
func printMount(m *model.Element) string {
	root := model.NewElement("")
	root.Kind = model.KindRoot
	root.Properties, root.Elements = m.Properties, m.Elements
	return model.ToTennStr(root, true, true)
}

// setLibText makes text the text of the mounted file. A window that has the file takes the change as
// an edit of its own (its undo, its save); else the document holds it until saveLibs.
func (d *docCore) setLibText(file, text string) error {
	if p := d.peerOf(file); p != nil {
		d.libBusy = true
		defer func() { d.libBusy = false }()
		a, b := diffRange(p.text, text)
		return p.applyText(a, len(p.text)-b, text[a:len(text)-b])
	}
	ld := d.libDoc(file)
	if !ld.tracker.Dirty() {
		ld.stamp, _ = StampOf(d.libPath(file))
	}
	ld.text = text
	ld.tracker.Changed(text)
	return nil
}

// libTextOp is a text edit of a mounted file as a store operation, so the undo of the document steps
// over it.
type libTextOp struct {
	model.OpBase
	d             *docCore
	file          string
	before, after string
}

func (o *libTextOp) Name() string { return "Edit library" }

func (o *libTextOp) Apply() error {
	o.d.textSet = true
	return o.d.setLibText(o.file, o.after)
}

func (o *libTextOp) Undo() error {
	o.d.textSet = true
	return o.d.setLibText(o.file, o.before)
}

// applyFileText replaces text[start:end) of the file (the document's own or a mounted one) as one undo
// step of the session.
func (d *docCore) applyFileText(file string, start, end int, repl string) error {
	if file == d.rel() {
		return d.applyText(start, end, repl)
	}
	if d.session == nil || d.stree == nil || d.stree.File(file) == nil {
		return fmt.Errorf("%s is not a file of this document", file)
	}
	cur := d.libText(file)
	if start < 0 || end < start || end > len(cur) {
		return fmt.Errorf("range %d..%d is outside the text of %d bytes", start, end, len(cur))
	}
	after := cur[:start] + repl + cur[end:]
	if _, p := tenn.Parse(after); p.Errors.HasErrors() {
		e := p.Errors.Errors[0]
		return fmt.Errorf("%s %d:%d: %s", file, e.Line+1, e.Col, e.Message)
	}
	op := &libTextOp{OpBase: model.OpBase{Store: d.session.Store(), UndoCalled: true}, d: d, file: file, before: cur, after: after}
	return d.session.Execute(op)
}

// fileOf is the file the offsets of the edits of n are in: the mounted file for an element with
// --from-file (its content is the file's), else the file the node is written in.
func (d *docCore) fileOf(n *scene.Node) string {
	switch {
	case n == nil:
		return d.rel()
	case n.Mount != nil:
		return n.Mount.Pos.File
	case n.Pos.File == "":
		return d.rel()
	}
	return n.Pos.File
}

// fileText is the text of a file of this document: the document's own, or a mounted one.
func (d *docCore) fileText(file string) string {
	if file == d.rel() {
		return d.text
	}
	return d.libText(file)
}

// libsDirty reports whether a mounted file has unsaved edits held by this document.
func (d *docCore) libsDirty() bool {
	for _, ld := range d.libDocs {
		if ld.tracker.Dirty() {
			return true
		}
	}
	return false
}

// mountDirty reports whether the file shown by m has unsaved edits, here or in the window that has it.
func (d *docCore) mountDirty(m *model.Element) bool {
	md := d.mounts[m]
	if md == nil {
		return false
	}
	if p := d.peerOf(md.file); p != nil {
		return p.tracker != nil && p.tracker.Dirty()
	}
	ld := d.libDocs[md.file]
	return ld != nil && ld.tracker.Dirty()
}

// saveLibs writes the mounted files with unsaved edits. A file that another program changed since the
// first edit is left alone.
func (d *docCore) saveLibs() error {
	var errs []error
	for file, ld := range d.libDocs {
		if !ld.tracker.Dirty() {
			continue
		}
		abs := d.libPath(file)
		if s, err := StampOf(abs); err == nil && s != ld.stamp {
			errs = append(errs, fmt.Errorf("%s was changed by another program: not saved", file))
			continue
		}
		if err := WriteFileAtomic(abs, []byte(ld.text)); err != nil {
			errs = append(errs, err)
			continue
		}
		ld.stamp, _ = StampOf(abs)
		if d.stree != nil && d.libs != nil { // our own write is not a change to follow
			d.libs[filepath.Join(d.stree.Root(), filepath.FromSlash(file))] = ld.stamp
		}
		ld.tracker.Saved(ld.text)
	}
	return errors.Join(errs...)
}
