package app

import (
	"os"
	"path/filepath"
)

// Tracker follows the unsaved state of a document: it is dirty while the text of the model differs
// from the text last saved or loaded. Undoing back to the saved text makes it clean again.
type Tracker struct{ saved, cur string }

// NewTracker starts clean at text.
func NewTracker(text string) *Tracker { return &Tracker{saved: text, cur: text} }

// Changed records the text of the model after a change.
func (t *Tracker) Changed(text string) { t.cur = text }

// Saved records that text is on disk.
func (t *Tracker) Saved(text string) { t.saved, t.cur = text, text }

// Dirty reports whether the model has changes that are not on disk.
func (t *Tracker) Dirty() bool { return t.cur != t.saved }

// ReloadAction is what to do with a version of the open file that appeared on disk.
type ReloadAction int

const (
	// ReloadApply: read the file, nothing is lost.
	ReloadApply ReloadAction = iota
	// ReloadAsk: the unsaved edits would be lost, ask whether to discard them.
	ReloadAsk
)

// DecideReload is NSDocument: a clean document follows the file, a dirty one asks.
func DecideReload(dirty bool) ReloadAction {
	if dirty {
		return ReloadAsk
	}
	return ReloadApply
}

// Title is the window title: the file name, marked when it has unsaved edits.
func Title(path string, dirty bool) string {
	name := "Untitled"
	if path != "" {
		name = filepath.Base(path)
	}
	if dirty {
		name += " - Edited"
	}
	return name + " - Tenniarb"
}

// WithExt adds the .tenn extension to a path chosen in a save dialog that has none.
func WithExt(path string) string {
	if filepath.Ext(path) == "" {
		return path + ".tenn"
	}
	return path
}

// WriteFileAtomic writes data to path through a temporary file in the same directory and a rename,
// so a crash leaves either the old file or the new one. A symlink is written through. The mode of
// an existing file is kept.
func WriteFileAtomic(path string, data []byte) (err error) {
	if real, e := filepath.EvalSymlinks(path); e == nil {
		path = real
	}
	mode := os.FileMode(0o644)
	if fi, e := os.Stat(path); e == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
