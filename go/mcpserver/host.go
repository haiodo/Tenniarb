package mcpserver

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/haiodo/tenniarb/go/scene"
)

// Host is what every session of one process shares: loaded trees, the documents of connected apps,
// and the locks that order edits and renders across sessions.
type Host struct {
	cache  treeCache
	live   *registry
	editMu sync.Mutex // one edit at a time: a read-modify-write of one session must not lose another's
	// renderMu serializes Build on cached trees (scene.Tree is not safe for concurrent use) and the
	// shared CoreGraphics renderer.
	renderMu sync.Mutex
}

func NewHost() *Host {
	return &Host{cache: treeCache{entries: map[cacheKey]*cacheEntry{}}, live: newRegistry()}
}

// Session opens the workspace at root for one client.
func (h *Host) Session(root string) (*Server, error) {
	s, err := open(root)
	if err != nil {
		return nil, err
	}
	s.host = h
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

// loadedFiles lists the files a tree read, by walking the imports from the main file.
func loadedFiles(tree *scene.Tree, rel string) []string {
	seen := map[string]bool{}
	var walk func(n *scene.Node)
	walk = func(n *scene.Node) {
		for _, m := range n.Mounts {
			if m.Target != nil && !seen[m.Target.Pos.File] {
				seen[m.Target.Pos.File] = true
				walk(m.Target)
			}
			walk(m)
		}
		if m := n.Mount; m != nil && !seen[m.Pos.File] {
			seen[m.Pos.File] = true
			walk(m)
		}
		for _, e := range n.Elems {
			walk(e)
		}
	}
	seen[rel] = true
	if f := tree.File(rel); f != nil {
		walk(f)
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	return out
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

// put caches a tree without errors. A tree with errors is never cached: a missing import that
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
