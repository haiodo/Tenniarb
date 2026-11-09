package app

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/jsoncanvas"
	"github.com/haiodo/tenniarb/go/scene"
)

// IsCanvas reports whether path is a JSON Canvas file.
func IsCanvas(path string) bool { return strings.EqualFold(filepath.Ext(path), ".canvas") }

// canvasText reads a .canvas file as the text of a .tenn document named after the file; notes are
// what the conversion left out.
func canvasText(path string, data []byte) (text string, notes []string, err error) {
	c, err := jsoncanvas.Parse(data)
	if err != nil {
		return "", nil, err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return jsoncanvas.ToTenn(c, name)
}

// canvasTree is the scene tree of the document, loaded again when the current one is behind the text.
func (v *Viewer) canvasTree() *scene.Tree {
	if v.stree == nil || !v.treeShows(v.rel(), v.text) {
		v.loadScene()
	}
	return v.stree
}

// canvasData writes the document as JSON Canvas: the scene tree is the source, not the text.
func (v *Viewer) canvasData() ([]byte, error) {
	t := v.canvasTree()
	if t == nil && v.session != nil {
		// The rollback flag keeps a classic diagram out of the tree; the text still loads as a file.
		t, _ = scene.LoadSource(v.root, v.rel(), v.text)
	}
	if t == nil {
		return nil, errors.New("the document has no scene to write as JSON Canvas")
	}
	c, _, err := jsoncanvas.FromTree(t)
	if err != nil {
		return nil, err
	}
	return jsoncanvas.Marshal(c)
}

// canvasNotes is the warning before an explicit save to .canvas: what FromTree had to group or drop,
// since the format has no field for it. "" when the document converts without loss.
func (v *Viewer) canvasNotes() (string, bool) {
	t := v.canvasTree()
	if t == nil && v.session != nil {
		t, _ = scene.LoadSource(v.root, v.rel(), v.text)
	}
	if t == nil {
		return "", false
	}
	_, notes, err := jsoncanvas.FromTree(t)
	if err != nil {
		return err.Error(), true
	}
	if len(notes) == 0 {
		return "", false
	}
	return strings.Join(notes, "\n"), true
}
