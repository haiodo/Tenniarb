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
	h := &Host{cache: treeCache{entries: map[cacheKey]*cacheEntry{}, budget: cacheBudget()}, live: newRegistry()}
	h.presence = newPresenceHub(h.sendPresence)
	h.comments = newCommentHub(comments.NewStore(comments.DirIn(config)), h.live)
	return h
}

// sendPresence tells the app that holds abs; a file no app holds goes to every app, so the one that
// opens such a file (the user's setting) learns of the edit. An app that moved to a newer service is
// not ours to tell.
func (h *Host) sendPresence(abs string, p servicewire.Presence) {
	if d := h.live.lookup(abs); d != nil {
		if d.moved {
			return
		}
		p.Path = d.path
		d.prov.peer.Notify(servicewire.MethodPresence, p)
		return
	}
	p.Path = abs
	for _, prov := range h.live.all() {
		prov.peer.Notify(servicewire.MethodPresence, p)
	}
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

// cacheBudget is what the cache may hold, in bytes of loaded trees; TENNIARB_TREE_CACHE_MB sets it, in
// megabytes. A count of trees said nothing about memory: 32 trees of a 1.7 MB workspace are 800 MB.
func cacheBudget() int64 {
	mb := int64(defaultCacheMB)
	if s := os.Getenv("TENNIARB_TREE_CACHE_MB"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			mb = n
		}
	}
	return mb << 20
}

const defaultCacheMB = 192

// loadFactor is the memory a loaded tree takes per byte of its source: a workspace of 1.7 MB loads into
// 25 MB of nodes, measured with go/scene on PlatformDesign.tenn.
const loadFactor = 15

type cacheKey struct{ root, rel string }

type cacheEntry struct {
	tree  *scene.Tree
	diags scene.Errors      // copied at load: Build appends to the tree's own list
	deps  map[string]string // rel -> stamp of every file the tree read
	used  uint64
	size  int64 // source bytes of the tree, times loadFactor
}

type treeCache struct {
	mu      sync.Mutex
	entries map[cacheKey]*cacheEntry
	clock   uint64
	size    int64 // of the entries, by their size
	budget  int64
}

// treeSize estimates what a loaded tree holds: its source bytes times what a byte of source grows into.
func treeSize(tree *scene.Tree) int64 {
	var n int64
	for _, f := range tree.Files() {
		if src, ok := tree.Source(f); ok {
			n += int64(len(src))
		}
	}
	return n * loadFactor
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
// appears later would not change any stamp. The least recently used trees go first when the budget is
// spent; the one just cached stays even when it is over it alone.
func (c *treeCache) put(root, rel string, tree *scene.Tree, deps map[string]string) *cacheEntry {
	e := &cacheEntry{tree: tree, diags: append(scene.Errors(nil), tree.Diagnostics()...), deps: deps, size: treeSize(tree)}
	if e.diags.Fatal() {
		return e
	}
	k := cacheKey{root, rel}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock++
	e.used = c.clock
	if old, ok := c.entries[k]; ok {
		c.size -= old.size
	}
	c.entries[k] = e
	c.size += e.size
	for c.size > c.budget && len(c.entries) > 1 {
		var oldK cacheKey
		var oldE *cacheEntry
		for k, v := range c.entries {
			if v == e {
				continue
			}
			if oldE == nil || v.used < oldE.used {
				oldK, oldE = k, v
			}
		}
		delete(c.entries, oldK)
		c.size -= oldE.size
	}
	return e
}
