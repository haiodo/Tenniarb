//go:build darwin || linux || windows

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// docsDelay is the pause after the last change before the open documents are sent to the service.
const docsDelay = 200 * time.Millisecond

var (
	errQuit   = errors.New("Tenniarb is quitting")
	errTyping = errors.New("the user is typing in this document: try again when the edit is done")
)

// provider serves the open documents to tenniarb-service. The service calls it from its own
// goroutines; every call runs on the UI thread.
type provider struct{ sh *Shell }

var _ servicewire.Provider = provider{}

// startProvider connects to the service while ctx lives and keeps its list of open documents up to
// date. It runs on the UI thread.
func (sh *Shell) startProvider(ctx context.Context) {
	client, err := servicewire.RunProvider(ctx, "", provider{sh}, 0)
	if err != nil {
		return
	}
	// One producer, the UI thread: the channel holds the latest set, the goroutine sends it once the
	// changes rest for docsDelay.
	latest := make(chan []servicewire.Doc, 1)
	push := func() {
		select {
		case <-latest:
		default:
		}
		latest <- sh.serviceDocs()
	}
	if sh.subs == nil {
		sh.subs = map[int]func(DocEvent){}
	}
	sh.nextSub++
	sh.subs[sh.nextSub] = func(e DocEvent) {
		if e.Kind != EventSelect {
			push()
		}
	}
	push()
	go func() {
		var docs []servicewire.Doc
		// connected is closed when the client connects after a set was refused: a set given while the
		// client is connecting can miss that connection, so it is sent again.
		var connected chan struct{}
		send := func() {
			if !errors.Is(client.SetDocuments(docs), servicewire.ErrNotConnected) || connected != nil {
				return
			}
			ch := make(chan struct{})
			connected = ch
			go func() {
				if client.WaitConnected(ctx) == nil {
					close(ch)
				}
			}()
		}
		for {
			select {
			case docs = <-latest:
			case <-connected:
				connected = nil
				send()
				continue
			case <-ctx.Done():
				return
			}
			t := time.NewTimer(docsDelay)
		rest:
			for {
				select {
				case docs = <-latest:
					t.Reset(docsDelay)
				case <-t.C:
					break rest
				case <-ctx.Done():
					t.Stop()
					return
				}
			}
			send()
		}
	}()
}

// serviceDocs are the open documents that have a file.
func (sh *Shell) serviceDocs() []servicewire.Doc {
	out := []servicewire.Doc{}
	for _, v := range sh.viewers {
		if v.state.Doc.Path == "" {
			continue
		}
		out = append(out, servicewire.Doc{Path: realPath(v.state.Doc.Path), Version: int64(v.version), Dirty: v.isDirty()})
	}
	return out
}

// on runs f on the UI thread with the viewer of path; after the event loop ended it gives errQuit.
func (p provider) on(path string, f func(v *Viewer) error) error {
	err := errQuit
	p.sh.ui(func() {
		var v *Viewer
		if v, err = p.sh.viewer(path); err == nil {
			err = f(v)
		}
	})
	return err
}

func (p provider) Text(path string) (r servicewire.TextResult, err error) {
	err = p.on(path, func(v *Viewer) error {
		r = servicewire.TextResult{Text: v.text, Version: int64(v.version), Dirty: v.isDirty()}
		if v.session == nil { // parse errors: the text as read
			r.Text = v.state.Doc.Text
		}
		return nil
	})
	return
}

// Apply replaces only the bytes that differ, so the text edit swaps the content of the innermost
// element around them and the undo steps before it keep their objects.
func (p provider) Apply(path string, base int64, text string) (ver int64, err error) {
	err = p.on(path, func(v *Viewer) error {
		switch {
		case base != int64(v.version):
			return ErrStale
		case v.inline != nil || v.props.changes != v.props.applied:
			return errTyping
		}
		if text != v.text {
			a, b := diffRange(v.text, text)
			if err := v.applyText(a, len(v.text)-b, text[a:len(text)-b]); err != nil {
				return err
			}
		}
		ver = int64(v.version)
		return nil
	})
	return
}

func (p provider) Selection() (sel servicewire.Selection, err error) {
	err = errQuit
	p.sh.ui(func() {
		err = nil
		v := p.sh.active
		if v == nil || v.state.Doc.Path == "" || v.state.Current == nil {
			return
		}
		sel.Path = realPath(v.state.Doc.Path)
		var segs []string
		for e := v.state.Current; e.Parent != nil; e = e.Parent {
			segs = append([]string{scene.NormalizeName(e.Name)}, segs...)
		}
		sel.Element = strings.Join(segs, ".")
		prefix := sel.Element
		if prefix != "" {
			prefix += "."
		}
		for _, n := range v.selNodes {
			sel.Nodes = append(sel.Nodes, prefix+n)
		}
		if v.session != nil && !v.sceneMode() {
			for _, it := range v.session.Selection() {
				if it.Kind == model.ItemKindItem {
					sel.Nodes = append(sel.Nodes, prefix+scene.NormalizeName(it.Name))
				}
			}
		}
	})
	return
}

func (p provider) Select(path string, nodes []string, reveal bool) error {
	return p.on(path, func(v *Viewer) error { return v.selectPaths(nodes, reveal) })
}

func (p provider) Render(path string, scale float64) ([]byte, error) {
	if path == "" {
		return nil, ErrNoDocument
	}
	return p.sh.RenderPNG(path, scale)
}

// selectPaths selects nodes named by dotted paths from the root of the file (spec 4.2), all in one
// element; that element goes on the canvas. No paths clear the selection.
func (v *Viewer) selectPaths(paths []string, reveal bool) error {
	if v.session == nil {
		return errors.New("the document has parse errors")
	}
	tree := v.stree
	if tree == nil {
		tree, _ = scene.LoadOverlay(v.root, v.rel(), map[string]string{v.rel(): v.text})
	}
	if tree == nil || tree.File(v.rel()) == nil {
		return errors.New("the document does not load")
	}
	e := v.state.Current
	var el *scene.Node
	var nodes []string
	var items []*model.DiagramItem
	for i, p := range paths {
		n, err := tree.Resolve(tree.File(v.rel()), p)
		if err != nil {
			return err
		}
		if n.Kind != scene.KItem && n.Kind != scene.KUse && n.Kind != scene.KElement {
			return fmt.Errorf("%s is not an item, a use or an element", p)
		}
		owner := n
		for owner.Kind != scene.KElement && owner.Kind != scene.KFile {
			owner = owner.Parent
		}
		if i == 0 {
			el = owner
			if e = Resolve(&v.session.Root().Element, elementSegs(owner)); e == nil {
				return fmt.Errorf("no element for %s", p)
			}
		} else if owner != el {
			return fmt.Errorf("%s is in another element than %s", p, paths[0])
		}
		if n == owner {
			continue
		}
		if !model.SceneElement(e) {
			it := classicItem(e, owner, n)
			if it == nil {
				return fmt.Errorf("%s is not an item of the diagram", p)
			}
			items = append(items, it)
			continue
		}
		rel := NodePath(owner, n)
		if rel == "" {
			return fmt.Errorf("%s is inside a component, not written in the element", p)
		}
		nodes = append(nodes, rel)
	}
	if reveal && v.win != nil {
		v.win.Unwrap().ForceActive()
		v.sh.active = v
	}
	v.showSelection(e, nodes, items, reveal)
	return nil
}

// elementSegs is the PathOf of the model element of the scene element n.
func elementSegs(n *scene.Node) []string {
	var segs []string
	for ; n.Kind == scene.KElement; n = n.Parent {
		k := 0
		for _, s := range n.Parent.Elems {
			if s == n {
				break
			}
			if s.Name == n.Name {
				k++
			}
		}
		seg := n.Name
		if k > 0 {
			seg += fmt.Sprintf("#%d", k)
		}
		segs = append([]string{seg}, segs...)
	}
	return segs
}

// classicItem is the model item of the scene node n of the classic element e: items keep file order
// in both.
func classicItem(e *model.Element, el, n *scene.Node) *model.DiagramItem {
	if n.Parent != el {
		return nil
	}
	k := 0
	for _, c := range el.Content {
		if c == n {
			break
		}
		if c.Kind == scene.KItem {
			k++
		}
	}
	for _, it := range e.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		if k == 0 {
			return it
		}
		k--
	}
	return nil
}
