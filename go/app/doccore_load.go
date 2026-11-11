//go:build darwin || linux || windows

package app

import (
	"maps"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// loadScene reads the scene tree anew, with the unsaved text of the other open documents.
func (d *docCore) loadScene() {
	d.stree, d.sceneErr, d.libs, d.entry = nil, "", nil, ""
	// Taken here, not where it is used: a document that stops being a scene would otherwise keep the tree
	// of the last edit for good.
	pending := d.pending
	d.pending = nil
	keep := d.keep
	d.keep = nil
	// The tree belongs to the element the session edits: without a session there is none.
	if d.session == nil {
		d.mounts = nil
		return
	}
	rel := d.rel()
	overlay := d.overlay()
	overlay[rel] = d.text
	var tree *scene.Tree
	var err error
	if pending != nil && maps.Equal(pending.overlay, overlay) {
		tree = pending.tree
	} else {
		tree, err = loadAuto(d.root, rel, overlay)
	}
	if tree == nil {
		d.sceneErr = errString(err)
		d.mounts = nil
		return
	}
	d.stree, d.entry = tree, tree.Entry()
	d.adoptMounts(model.MountFiles(d.session.Root(), tree, rel, keep...))
	d.sceneErr = sceneErrors(tree.Diagnostics())
	if d.kind == model.DocDiagram {
		d.sceneErr = "" // the classic profile has no scene errors
	}
	d.libs = map[string]Stamp{}
	for _, f := range tree.Files()[1:] {
		abs := filepath.Join(tree.Root(), filepath.FromSlash(f))
		s, _ := StampOf(abs)
		d.libs[abs] = s
	}
}

// loadAuto is scene.LoadAuto; tests count the loads.
var loadAuto = scene.LoadAuto

// pendingScene is a tree loaded with overlay as the text of the files.
type pendingScene struct {
	overlay map[string]string
	tree    *scene.Tree
}

// sceneErrorCount is how many scene errors (not warnings) the tree would have with text as the text
// of file; -1 when the tree does not load at all.
func (d *docCore) sceneErrorCount(file, text string) int {
	_, n := d.loadWith(file, text)
	return n
}

func (d *docCore) loadWith(file, text string) (*pendingScene, int) {
	return d.loadFiles(map[string]string{file: text})
}

// loadFiles is loadWith for several files at once.
func (d *docCore) loadFiles(texts map[string]string) (*pendingScene, int) {
	overlay := d.overlay()
	rel := d.rel()
	overlay[rel] = d.text
	maps.Copy(overlay, texts)
	tree, _ := loadAuto(d.root, rel, overlay)
	return &pendingScene{overlay: overlay, tree: tree}, errorCount(tree)
}

// treeShows reports whether d.stree was loaded from text as the text of file and from the current text of
// every other file the document overlays, so its errors are the ones of a load now.
func (d *docCore) treeShows(file, text string) bool {
	if d.stree == nil {
		return false
	}
	if src, ok := d.stree.Source(file); !ok || src != text {
		return false
	}
	overlay := d.overlay()
	overlay[d.rel()] = d.text
	for f, t := range overlay {
		if src, ok := d.stree.Source(f); ok && src != t {
			return false
		}
	}
	return true
}

func errorCount(tree *scene.Tree) int {
	if tree == nil {
		return -1
	}
	n := 0
	for _, e := range tree.Diagnostics() {
		if !e.Warn {
			n++
		}
	}
	return n
}

// editAddsErrors reports whether replacing text[e.Start:e.End] of file by e.New adds scene errors: an
// edit from the window must not, autosave would write a broken file. Errors are counted, not compared,
// because their positions move with the edit. The tree of the new text is kept for the loadScene that
// follows the edit, so an edit loads the scene once.
func (d *docCore) editAddsErrors(file, text string, e TextEdit) bool {
	if e.Start < 0 || e.Start > e.End || e.End > len(text) {
		return false
	}
	var was int
	if d.treeShows(file, text) {
		was = errorCount(d.stree)
	} else {
		was = d.sceneErrorCount(file, text)
	}
	p, now := d.loadWith(file, text[:e.Start]+e.New+text[e.End:])
	if now > was || (now < 0 && was >= 0) {
		return true
	}
	if p.tree != nil {
		d.pending = p
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// sceneErrors is the banner text of the scene errors, without warnings, as errorText is for parse errors.
func sceneErrors(es scene.Errors) string {
	var lines []string
	for _, e := range es {
		if e.Warn {
			continue
		}
		if len(lines) == 5 {
			lines = append(lines, "...")
			break
		}
		lines = append(lines, e.Error())
	}
	return strings.Join(lines, "\n")
}

// bannerText is what the banner shows: parse errors first, then the scene errors.
func (d *docCore) bannerText() string {
	if d.state.Error != "" {
		return d.state.Error
	}
	if d.builtin {
		return errLibraryReadOnly(d.state.Doc.Path).Error()
	}
	return d.sceneErr
}
