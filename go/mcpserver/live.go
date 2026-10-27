package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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

// set replaces the documents of p.
func (r *registry) set(p *provider, docs []servicewire.Doc) {
	keys := make([]string, len(docs))
	for i, d := range docs {
		keys[i] = realPath(d.Path)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p] = true
	for k, d := range r.docs {
		if d.prov == p {
			delete(r.docs, k)
		}
	}
	for i, d := range docs {
		if !filepath.IsAbs(d.Path) {
			continue
		}
		ld := &liveDoc{prov: p, path: d.Path, doc: d}
		if old := r.docs[keys[i]]; old != nil && old.prov == p && old.textOf == d.Version {
			ld.text, ld.textOf = old.text, old.textOf
		}
		r.docs[keys[i]] = ld
	}
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
