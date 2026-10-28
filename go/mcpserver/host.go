package mcpserver

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// Host is what every session of one process shares: loaded trees, the documents of connected apps,
// and the locks that order edits and renders across sessions.
type Host struct {
	cache treeCache
	live  *registry
	// presence tells the apps which agents work on their documents.
	presence *presenceHub
	// comments stores the comments of the user and wakes the agents that wait for them.
	comments *commentHub
	editMu   sync.Mutex // one edit at a time: a read-modify-write of one session must not lose another's
	// renderMu serializes Build on cached trees (scene.Tree is not safe for concurrent use) and the
	// shared CoreGraphics renderer.
	renderMu sync.Mutex
}

// NewHost makes a host that keeps the comments in the config folder of the user.
func NewHost() *Host { return NewHostIn(comments.ConfigDir()) }

// NewHostIn is NewHost with the comments in the folder config/comments.
func NewHostIn(config string) *Host {
	h := &Host{cache: treeCache{entries: map[cacheKey]*cacheEntry{}}, live: newRegistry()}
	h.presence = newPresenceHub(h.sendPresence)
	h.comments = newCommentHub(comments.NewStore(comments.DirIn(config)), h.live)
	return h
}

// sendPresence tells the app that holds abs; an app that moved to a newer service is not ours to tell.
func (h *Host) sendPresence(abs string, p servicewire.Presence) {
	d := h.live.lookup(abs)
	if d == nil || d.moved {
		return
	}
	p.Path = d.path
	d.prov.peer.Notify(servicewire.MethodPresence, p)
}

// Session opens the workspace at root for one client.
func (h *Host) Session(root string) (*Server, error) {
	s, err := open(root)
	if err != nil {
		return nil, err
	}
	s.host = h
	s.agent = h.presence.open()
	s.cursor.seq = h.comments.head()
	return s, nil
}

// maxTrees bounds the cache; the least recently used tree goes first. Ceiling: a count, not bytes,
// so a few huge diagrams can hold a lot of memory; switch to summed source sizes if that bites.
const maxTrees = 32

type cacheKey struct{ root, rel string }

type cacheEntry struct {
	tree  *scene.Tree
	diags scene.Errors      // copied at load: Build appends to the tree's own list
	deps  map[string]string // rel -> stamp of every file the tree read
	used  uint64
}

type treeCache struct {
	mu      sync.Mutex
	entries map[cacheKey]*cacheEntry
	clock   uint64
}

// stamp identifies the content of rel: the version of the open document, or size and mtime on disk.
func stamp(root, rel string, overlay map[string]liveDoc) string {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if d, ok := overlay[abs]; ok {
		return "v" + strconv.FormatInt(d.doc.Version, 10)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "-"
	}
	return strconv.FormatInt(st.Size(), 10) + "/" + strconv.FormatInt(st.ModTime().UnixNano(), 10)
}

// get returns the cached tree of rel when no file it read has changed since.
func (c *treeCache) get(root, rel string, overlay map[string]liveDoc) (*cacheEntry, bool) {
	c.mu.Lock()
	e := c.entries[cacheKey{root, rel}]
	c.mu.Unlock()
	if e == nil {
		return nil, false
	}
	for f, st := range e.deps {
		if stamp(root, f, overlay) != st {
			return nil, false
		}
	}
	c.mu.Lock()
	c.clock++
	e.used = c.clock
	c.mu.Unlock()
	return e, true
}

// put caches a tree without errors. A tree with errors is never cached: a missing mounted file that
// appears later would not change any stamp.
func (c *treeCache) put(root, rel string, tree *scene.Tree, deps map[string]string) *cacheEntry {
	e := &cacheEntry{tree: tree, diags: append(scene.Errors(nil), tree.Diagnostics()...), deps: deps}
	if e.diags.Fatal() {
		return e
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock++
	e.used = c.clock
	c.entries[cacheKey{root, rel}] = e
	for len(c.entries) > maxTrees {
		var oldK cacheKey
		var oldE *cacheEntry
		for k, v := range c.entries {
			if oldE == nil || v.used < oldE.used {
				oldK, oldE = k, v
			}
		}
		delete(c.entries, oldK)
	}
	return e
}
