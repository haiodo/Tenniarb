//go:build darwin || linux || windows

package app

import (
	"path/filepath"

	g "github.com/haiodo/gowt"
)

var tennFilters = []g.FileFilter{{Name: "Tenniarb", Pattern: "*.tenn"}, {Name: "All files", Pattern: "*"}}

// diskChanged reports whether the file is not the version this window last read or wrote.
func (v *Viewer) diskChanged() bool {
	if v.conflict {
		return true
	}
	s, err := StampOf(v.watch.Path)
	return err == nil && s != v.watch.Last
}

// save writes the document to its file, or asks for one when it has none. False: nothing was written.
func (v *Viewer) save() bool {
	if v.session == nil {
		return false
	}
	v.endInline(true)
	path := v.state.Doc.Path
	if path == "" {
		return v.saveAs()
	}
	if v.diskChanged() && !v.win.Confirm("File changed on disk",
		"\""+filepath.Base(path)+"\" was changed by another program. Overwrite it with your version?") {
		return false
	}
	return v.writeTo(path)
}

func (v *Viewer) saveAs() bool {
	if v.session == nil {
		return false
	}
	v.endInline(true)
	dir, name := "", "Untitled.tenn"
	if p := v.state.Doc.Path; p != "" {
		dir, name = filepath.Dir(p), filepath.Base(p)
	}
	paths := v.win.FileDialog(g.FileDialog{Title: "Save As", Dir: dir, Name: name, Save: true, Filters: tennFilters})
	if len(paths) != 1 {
		return false
	}
	return v.writeTo(WithExt(paths[0]))
}

// writeTo stores the text of the model at path atomically and makes path the file of the document.
func (v *Viewer) writeTo(path string) bool {
	if err := v.writeFile(path); err != nil {
		v.win.MessageBox(g.Message{Title: "Save failed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
		return false
	}
	return true
}

func (v *Viewer) writeFile(path string) error {
	text := v.text
	if err := WriteFileAtomic(path, []byte(text)); err != nil {
		return err
	}
	stamp, _ := StampOf(path)
	v.watch = Watcher{Path: path, Last: stamp}
	v.state.Doc.Path, v.state.Doc.Stamp = path, stamp
	if r := WorkspaceRoot(path); r != v.root {
		v.root = r
		v.loadScene()
	}
	v.tracker.Saved(text)
	v.conflict = false
	v.autosave.Cancel()
	if v.id != "" {
		DropUntitled(v.sh.dir, v.id)
		v.id = ""
	}
	v.sh.addRecent(path)
	v.updateTitle()
	v.sh.publish(DocEvent{Path: path, Kind: EventSave, Version: v.version})
	return nil
}

// scheduleAutosave runs after a change: the edits are written 2 s after the last one.
func (v *Viewer) scheduleAutosave() {
	if v.isDirty() {
		v.autosave.Touch()
		return
	}
	v.autosave.Cancel()
	if v.id != "" {
		DropUntitled(v.sh.dir, v.id) // undone back to the new document
	}
}

// autosaveNow writes the edits without asking, as NSDocument autosavesInPlace does: a file in place, an
// untitled document to its stash. A file another program changed is left alone: the user decides on Save.
func (v *Viewer) autosaveNow() {
	if v.session == nil || !v.isDirty() {
		return
	}
	if v.state.Doc.Path == "" {
		if err := StashUntitled(v.sh.dir, v.id, v.text); err != nil {
			v.setBanner("Could not keep the untitled document: " + err.Error())
		}
		return
	}
	if v.diskChanged() {
		return
	}
	if err := v.writeFile(v.state.Doc.Path); err != nil {
		v.setBanner("Autosave failed: " + err.Error())
	}
}

// onClose is the answer of the window to a close request: the document is kept or discarded, and the
// frame remembered.
func (v *Viewer) onClose() bool {
	if !v.confirmDiscard() {
		return false
	}
	v.saveFrame()
	return true
}

// confirmDiscard is the answer to "may the window drop the document": true when it is saved or kept
// for the next launch, or the user does not want it saved.
func (v *Viewer) confirmDiscard() bool {
	if v.force {
		return true
	}
	v.endInline(true)
	v.asking = true
	defer func() { v.asking = false }()
	switch DecideClose(v.state.Doc.Path, v.isDirty(), v.sh.quitting) {
	case CloseNow:
		v.autosave.Cancel()
		if v.id != "" {
			DropUntitled(v.sh.dir, v.id)
		}
		return true
	case CloseSave:
		return v.save()
	case CloseStash:
		v.autosave.Cancel()
		return StashUntitled(v.sh.dir, v.id, v.text) == nil
	}
	switch v.win.MessageBox(g.Message{
		Title:   "Save changes?",
		Text:    "Do you want to save the changes made to \"Untitled\"? Your changes will be lost if you do not save them.",
		Icon:    g.IconQuestion,
		Buttons: g.ButtonsYesNoCancel,
	}) {
	case g.AnswerYes:
		return v.save()
	case g.AnswerNo:
		v.autosave.Cancel()
		DropUntitled(v.sh.dir, v.id)
		return true
	}
	return false
}
