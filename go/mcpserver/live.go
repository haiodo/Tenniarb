package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
)

// callTimeout bounds one call to the app: a hung UI must not hang the agent.
const callTimeout = 10 * time.Second

// provider is a connected app.
type provider struct {
	peer *servicewire.Peer
}

func (p *provider) call(method string, params, result any) error {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	err := p.peer.Call(ctx, method, params, result)
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("Tenniarb did not answer %s in %v", method, callTimeout)
	}
	return err
}

// liveDoc is a document open in an app, keyed by its real path.
type liveDoc struct {
	prov   *provider
	path   string // as the app sent it
	doc    servicewire.Doc
	moved  bool // its app reconnected to a newer service: this one must not touch the file
	text   string
	textOf int64 // version text belongs to; 0 when not fetched
}

// registry holds the documents of every connected app.
type registry struct {
	mu        sync.Mutex
	docs      map[string]*liveDoc // real path
	providers map[*provider]bool
}

func newRegistry() *registry {
	return &registry{docs: map[string]*liveDoc{}, providers: map[*provider]bool{}}
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// entries lists the open documents by real path.
func (r *registry) entries() map[string]liveDoc {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]liveDoc, len(r.docs))
	for k, d := range r.docs {
		out[k] = *d
	}
	return out
}

// set replaces the documents of p and returns the real paths it did not have before.
func (r *registry) set(p *provider, docs []servicewire.Doc) (added []string) {
	keys := make([]string, len(docs))
	for i, d := range docs {
		keys[i] = realPath(d.Path)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p] = true
	had := map[string]bool{}
	for k, d := range r.docs {
		if d.prov == p {
			had[k] = true
			delete(r.docs, k)
		}
	}
	for i, d := range docs {
		if !filepath.IsAbs(d.Path) {
			continue
		}
		if !had[keys[i]] {
			added = append(added, keys[i])
		}
		ld := &liveDoc{prov: p, path: d.Path, doc: d}
		if old := r.docs[keys[i]]; old != nil && old.prov == p && old.textOf == d.Version {
			ld.text, ld.textOf = old.text, old.textOf
		}
		r.docs[keys[i]] = ld
	}
	return added
}

// drop forgets p; when moved, its documents stay as tombstones that refuse every tool.
func (r *registry) drop(p *provider, moved bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, p)
	for k, d := range r.docs {
		if d.prov != p {
			continue
		}
		if moved {
			d.moved = true
		} else {
			delete(r.docs, k)
		}
	}
}

func (r *registry) all() []*provider {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*provider
	for p := range r.providers {
		out = append(out, p)
	}
	return out
}

// lookup returns a copy of the entry for the real path abs, or nil.
func (r *registry) lookup(abs string) *liveDoc {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d := r.docs[abs]; d != nil {
		c := *d
		return &c
	}
	return nil
}

// within lists the open documents under root, by real path.
func (r *registry) within(root string) map[string]liveDoc {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]liveDoc{}
	for k, d := range r.docs {
		if within(root, k) {
			out[k] = *d
		}
	}
	return out
}

var errMoved = errors.New("the file is open in Tenniarb, which has moved to a newer tenniarb-service: restart this MCP session (for example /mcp reconnect)")

// text returns the live text of abs; fresh asks the app even when the cached version looks current.
func (r *registry) text(abs string, fresh bool) (servicewire.TextResult, error) {
	d := r.lookup(abs)
	if d == nil {
		return servicewire.TextResult{}, errors.New("not open")
	}
	if d.moved {
		return servicewire.TextResult{}, errMoved
	}
	if !fresh && d.textOf != 0 && d.textOf == d.doc.Version {
		return servicewire.TextResult{Text: d.text, Version: d.textOf, Dirty: d.doc.Dirty}, nil
	}
	var res servicewire.TextResult
	if err := d.prov.call(servicewire.MethodText, servicewire.TextArgs{Path: d.path}, &res); err != nil {
		return res, fmt.Errorf("cannot read the open document from Tenniarb: %w", err)
	}
	r.mu.Lock()
	if cur := r.docs[abs]; cur != nil && cur.prov == d.prov {
		cur.text, cur.textOf = res.Text, res.Version
		if res.Version > cur.doc.Version {
			cur.doc.Version, cur.doc.Dirty = res.Version, res.Dirty
		}
	}
	r.mu.Unlock()
	return res, nil
}

// askOpen asks a connected app to open abs; true when one did.
func (r *registry) askOpen(abs string) bool {
	for _, p := range r.all() {
		var res servicewire.OpenFileResult
		if err := p.call(servicewire.MethodOpenFile, servicewire.OpenFileArgs{Path: abs}, &res); err == nil && res.Opened {
			return true
		}
	}
	return false
}

// openForEdit is the document that owns abs, opening a window when an open document mounts the file: a
// change to a library belongs to a document, so its undo and its save cover it and every board that
// mounts the file shows the same text. nil when no app holds or mounts abs: the file is nobody's draft.
func (s *Server) openForEdit(abs string) *liveDoc {
	if d := s.host.live.lookup(abs); d != nil {
		return d
	}
	if !s.mountedByOpen(abs) || !s.host.live.askOpen(abs) {
		return nil
	}
	for i := 0; i < 300; i++ { // the app opens the window on its UI thread and sends the documents after
		if d := s.host.live.lookup(abs); d != nil {
			return d
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// mountedByOpen reports whether an open document reads abs: a mounted library, a part or an entry.
func (s *Server) mountedByOpen(abs string) bool {
	want := realPath(abs)
	for docAbs := range s.host.live.within(s.root) {
		if realPath(docAbs) == want {
			continue
		}
		rel, err := filepath.Rel(s.root, docAbs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		_, tree, err := s.diagnose(filepath.ToSlash(rel))
		if err != nil || tree == nil {
			continue
		}
		for _, f := range tree.Files() {
			if realPath(filepath.Join(s.root, filepath.FromSlash(f))) == want {
				return true
			}
		}
	}
	return false
}
