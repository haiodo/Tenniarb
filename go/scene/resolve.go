package scene

import (
	"errors"
	"fmt"
	"strings"
)

func pick(nodes []*Node, key string) []*Node {
	var out []*Node
	for _, n := range nodes {
		if n.Key == key {
			out = append(out, n)
		}
	}
	return out
}

func where(nodes []*Node) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = n.Pos.String()
	}
	return strings.Join(parts, ", ")
}

// mountCtx is the chain of mount points a file is reached through: at is the element with
// --from-file in the mounting file, up the context of that file. nil for the entry document. The
// nodes of a file are shared between its mounts, so the chain is kept beside them, not in them.
type mountCtx struct {
	at *Node
	up *mountCtx
}

func (c *mountCtx) push(at *Node) *mountCtx { return &mountCtx{at, c} }

// outward is the scope after a: its parent, or from the root of a mounted file the place it is
// mounted at (spec 5a.1). It returns nil past the entry document.
func outward(a *Node, c *mountCtx) (*Node, *mountCtx) {
	if a.Parent != nil {
		return a.Parent, c
	}
	if c == nil {
		return nil, nil
	}
	return c.at.Parent, c.up
}

// hit is a node a path names so far, with the context of the file it lives in.
type hit struct {
	n *Node
	c *mountCtx
}

func pickHits(n *Node, c *mountCtx, key string) []hit {
	var out []hit
	for _, m := range pick(n.named(), key) {
		out = append(out, hit{m, c})
	}
	return out
}

func hitNodes(hs []hit) []*Node {
	out := make([]*Node, len(hs))
	for i, h := range hs {
		out[i] = h.n
	}
	return out
}

// ctxOf is the mount context of the file n lives in; a file mounted in several places has the first.
func (t *Tree) ctxOf(n *Node) *mountCtx {
	for n.Parent != nil {
		n = n.Parent
	}
	return t.ctxs[n]
}

// Resolve finds the node a path names from scope, by the rules of spec 4.2/4.3: the first segment
// is looked up from scope up, the rest is followed from there. Names are normalized. The error
// says what is wrong with the path; nothing is added to the diagnostics.
func (t *Tree) Resolve(scope *Node, path string) (*Node, error) {
	n, _, msg := lookup(scope, t.ctxOf(scope), path)
	if msg != "" {
		return nil, errors.New(msg)
	}
	return n, nil
}

// resolve is Resolve that reports a failure at pos. The context of scope is given; the one of the
// found node comes back.
func (t *Tree) resolve(scope *Node, ctx *mountCtx, path string, pos Pos) (*Node, *mountCtx) {
	n, c, msg := lookup(scope, ctx, path)
	if msg != "" {
		t.fail(pos, "%s", msg)
	}
	return n, c
}

// lookup returns the node and the context of its file, or nil and the message of the problem. From
// the root of a mounted file the search goes on at the mount point, up to the entry document.
func lookup(scope *Node, ctx *mountCtx, path string) (*Node, *mountCtx, string) {
	segs, ok := splitPath(path)
	if !ok {
		return nil, nil, fmt.Sprintf("bad path %q", path)
	}
	for a, c := scope, ctx; a != nil; a, c = outward(a, c) {
		cur := pickHits(a, c, segs[0])
		if len(cur) == 0 {
			continue
		}
		for _, seg := range segs[1:] {
			var next []hit
			for _, h := range cur {
				cc := h.c
				if h.n.Mount != nil {
					cc = cc.push(h.n)
				}
				next = append(next, pickHits(h.n, cc, seg)...)
			}
			if len(next) == 0 {
				return nil, nil, fmt.Sprintf("path %s: nothing named %q under %s", path, seg, where(hitNodes(cur)))
			}
			cur = next
		}
		if len(cur) > 1 {
			return nil, nil, fmt.Sprintf("ambiguous path %s: candidates %s", path, where(hitNodes(cur)))
		}
		return cur[0].n, cur[0].c, ""
	}
	return nil, nil, fmt.Sprintf("path %s: %q is not defined here or above", path, segs[0])
}
