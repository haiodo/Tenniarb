//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/model"
)

// noticeTime is how long a note about a link stays in the banner.
const noticeTime = 6 * time.Second

// notice shows text in the banner for a while; the banner is red, as it has no other colour.
func (v *Viewer) notice(text string) {
	if v.banner == nil || v.win.Unwrap().IsDisposed() {
		return
	}
	v.setBanner(text)
	v.app.After(noticeTime, func() {
		if !v.win.Unwrap().IsDisposed() {
			v.setBanner(v.bannerText())
		}
	})
}

// linkNode is the selected node of a scene or the selected item of a diagram; "" when none.
func (v *Viewer) linkNode() string {
	if v.sceneMode() {
		if len(v.selNodes) > 0 {
			return v.selNodes[0]
		}
		return ""
	}
	for _, it := range v.sessionItems() {
		if !it.IsLink() && it.Name != "" { // a link has no name to find it by
			return it.Name
		}
	}
	return ""
}

// copyLink puts the link to the element on the canvas and the selected node on the clipboard.
func (v *Viewer) copyLink() {
	if v.session == nil || v.state.Current == nil {
		return
	}
	if v.state.Doc.Path == "" {
		v.notice("Save the file to copy a link to it.")
		return
	}
	home, _ := os.UserHomeDir()
	l, note, err := v.sh.repos.Of(v.state.Doc.Path, home, PathOf(v.state.Current), v.linkNode())
	if err != nil {
		v.notice(err.Error())
		return
	}
	v.setClipboard(l.URL())
	if note != "" {
		v.notice(note)
	}
}

// withLink adds Copy Link to the end of a context menu.
func (v *Viewer) withLink(entries []editor.Entry) []editor.Entry {
	if v.session == nil {
		return entries
	}
	if len(entries) > 0 {
		entries = append(entries, editor.Entry{Sep: true})
	}
	return append(entries, editor.Entry{Label: "Copy Link", Run: v.copyLink})
}

// gotoLink shows the place l names in this document, selecting it and scrolling it into view. It returns
// what was not found, "" when the whole place was.
func (v *Viewer) gotoLink(l link.Link) (missing string) {
	if v.session == nil {
		return "The document has parse errors."
	}
	e := linkElement(&v.session.Root().Element, l.Element)
	if e == nil {
		return "No element " + strings.Join(l.Element, ".") + " in this file."
	}
	sel := Selection{Element: PathOf(e)}
	if l.Node != "" {
		if model.SceneElement(e) {
			sel.Nodes = []string{l.Node}
		} else if name, ok := linkItem(e, l.Node); ok {
			sel.Items = []string{name}
		}
	}
	el, items, err := v.resolveSelection(sel)
	if err != nil || (l.Node != "" && len(sel.Nodes)+len(sel.Items) == 0) {
		missing = fmt.Sprintf("No node %s in the element %s.", l.Node, strings.Join(l.Element, "."))
		sel.Nodes, sel.Items = nil, nil
		el, items, _ = v.resolveSelection(sel)
	}
	if v.win != nil {
		v.win.Unwrap().ForceActive()
		v.sh.active = v
	}
	v.showSelection(el, sel.Nodes, items, true)
	if v.win != nil && !v.sceneMode() && len(items) > 0 {
		v.revealItem(items[0])
	}
	return missing
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// say shows a message in the window that asked, else in the front one.
func (sh *Shell) say(from *Viewer, text string) {
	if from == nil {
		from = sh.active
	}
	if from != nil {
		from.notice(text)
	}
}

// openLink opens the document a link names and shows its place. from is the window that asked; nil for a
// request of the system.
func (sh *Shell) openLink(text string, from *Viewer) {
	l, err := link.Parse(text)
	if err != nil {
		sh.say(from, err.Error())
		return
	}
	path, ok := sh.linkFile(l, from)
	if !ok {
		return
	}
	sh.openPath(path)
	v, err := sh.viewer(path)
	if err != nil {
		sh.say(from, "Could not open "+path)
		return
	}
	if missing := v.gotoLink(l); missing != "" {
		v.notice(missing)
	}
}

// linkFile is the local file of l; when no known clone has it, the user is asked where the repository is.
func (sh *Shell) linkFile(l link.Link, from *Viewer) (string, bool) {
	home, _ := os.UserHomeDir()
	var open []string
	if sh.active != nil && sh.active.state.Doc.Path != "" {
		open = append(open, sh.active.state.Doc.Path)
	}
	for _, v := range sh.viewers {
		if v.state.Doc.Path != "" {
			open = append(open, v.state.Doc.Path)
		}
	}
	roots := linkRoots(l, &sh.repos, sh.dir, open, sh.recent)
	if p, ok := link.Resolve(l, roots, home, isFile); ok {
		return p, true
	}
	if l.Home || l.Repo == "" {
		sh.say(from, "Cannot find "+l.Path+" on this machine.")
		return "", false
	}
	return sh.askClone(l, from)
}

// askClone asks for the folder of the clone of l.Repo and remembers it.
func (sh *Shell) askClone(l link.Link, from *Viewer) (string, bool) {
	if from == nil {
		from = sh.active
	}
	if from == nil || from.win == nil {
		return "", false
	}
	if !from.win.Confirm("Open link", fmt.Sprintf("Tenniarb does not know where you keep %s. Choose the folder of your clone.", l.Repo)) {
		return "", false
	}
	dir, ok := from.win.DirDialog("Folder of "+l.Repo, "")
	if !ok {
		return "", false
	}
	path := filepath.Join(dir, filepath.FromSlash(l.Path))
	if !isFile(path) {
		sh.say(from, fmt.Sprintf("%s has no %s.", dir, l.Path))
		return "", false
	}
	if err := SaveRepoDir(sh.dir, l.Repo, dir); err != nil {
		sh.say(from, "Could not remember the folder: "+err.Error())
	}
	return path, true
}

// showOpenLink is File > Open Link: a field where a link is pasted; a link on the clipboard is filled in.
func (v *Viewer) showOpenLink() {
	v.closePopover()
	p := v.win.Popover(g.Gap(4), g.Padding(6))
	var field *g.TextView
	p.Add(g.Text(g.Focused(), g.Hint("tenniarb://open?... or File.tenn#Element/node"), g.Size(460, 0), g.ID("link-field"), g.Ref(&field), g.Init(func(t *g.TextView) {
		t.OnActivate(func() {
			text := t.Text()
			p.Close()
			v.app.Async(func() { v.sh.openLink(text, v) })
		})
	})))
	if text, ok := v.app.Clipboard().Text(); ok && link.IsLink(text) {
		field.SetText(strings.TrimSpace(text))
	}
	v.openPopover(p)
	v.activeText = field.Unwrap()
	w, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), w/2, 0, g.PopoverBelow)
}
