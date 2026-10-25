package penimport

import (
	"strconv"
	"strings"
	"unicode"
)

func key(n *Node) string { return normalize(n.Name) }

func (n *Node) isNamed() bool { return n.Kind == KItem || n.Kind == KUse }

// pathSeg is a node as a segment of an at/replace path: slots are segments too.
func pathSeg(n *Node) (string, bool) {
	if n.Kind == KSlot {
		return n.Slot, true
	}
	return key(n), n.isNamed()
}

// eachNode visits every node of the file, the ones inside blocks included.
func eachNode(n *Node, fn func(*Node)) {
	fn(n)
	for _, k := range n.Kids {
		eachNode(k, fn)
	}
	for _, k := range n.Content {
		eachNode(k, fn)
	}
}

// markNeeded flags the nodes some path goes through: those must have names that tell siblings apart.
func markNeeded(f *File) {
	eachNode(f.Root, func(n *Node) {
		if n.Parent == f.Root {
			n.needed = true
		}
		if n.Kind != KUse || n.Target == nil {
			return
		}
		for p := n.Target; p != nil && p.Kind != KFile; p = p.Parent {
			p.needed = true
		}
		for _, ov := range n.Block {
			for _, c := range ov.Chain {
				c.needed = true
			}
		}
	})
}

func digitLead(s string) bool {
	for _, r := range s {
		return unicode.IsDigit(r)
	}
	return false
}

// dedupe renames the later of the siblings with one key where a path needs to tell them apart.
func dedupe(list []*Node) {
	need := map[string]bool{}
	all := map[string]bool{}
	for _, n := range list {
		if !n.isNamed() {
			continue
		}
		if n.needed && digitLead(key(n)) {
			n.Name = "_" + n.Name
		}
		need[key(n)] = need[key(n)] || n.needed
		all[key(n)] = true
	}
	seen := map[string]bool{}
	for _, n := range list {
		if !n.isNamed() {
			continue
		}
		k := key(n)
		if seen[k] && need[k] {
			for i := 2; ; i++ {
				cand := n.Name + " " + strconv.Itoa(i)
				if !all[normalize(cand)] {
					n.Name = cand
					all[normalize(cand)] = true
					break
				}
			}
		}
		seen[k] = true
	}
}

func dedupeAll(f *File) {
	dedupe(f.Root.Kids)
	eachNode(f.Root, func(n *Node) {
		dedupe(n.Kids)
		dedupe(n.Content)
	})
}

// pathTo is the path a use in file `from` writes to name t.
func pathTo(from *File, t *Node) string {
	var segs []string
	for p := t; p != nil && p.Kind != KFile; p = p.Parent {
		if p.isNamed() {
			segs = append(segs, key(p))
		}
	}
	for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
		segs[i], segs[j] = segs[j], segs[i]
	}
	if t.File != from {
		segs = append([]string{from.mountAlias(t.File)}, segs...)
	}
	return strings.Join(segs, ".")
}

func (f *File) mountAlias(lib *File) string { return normalize(lib.Alias) }

func named(a *Node, seg string) []*Node {
	var out []*Node
	pick := func(list []*Node) {
		for _, k := range list {
			if k.isNamed() && key(k) == seg {
				out = append(out, k)
			}
		}
	}
	switch a.Kind {
	case KFile:
		pick(a.Kids)
		for _, lib := range a.File.Imports {
			if a.File.mountAlias(lib) == seg {
				out = append(out, lib.Root)
			}
		}
	case KItem, KSlot:
		pick(a.Kids)
	case KUse:
		pick(a.Content)
	}
	return out
}

// resolveSim follows go/scene's rules: the first segment is looked up from the scope of the use
// upwards, the rest below what it found.
func resolveSim(u *Node, path string) []*Node {
	segs := strings.Split(path, ".")
	for a := u.Parent; a != nil; a = a.Parent {
		cur := named(a, segs[0])
		if len(cur) == 0 {
			continue
		}
		for _, s := range segs[1:] {
			var next []*Node
			for _, n := range cur {
				next = append(next, named(n, s)...)
			}
			cur = next
		}
		return cur
	}
	return nil
}

// shadowed lists the uses whose path does not reach their target because a node nearer to the use
// has the name of the first segment.
func shadowed(files []*File) []*Node {
	var bad []*Node
	for _, f := range files {
		eachNode(f.Root, func(n *Node) {
			if n.Kind != KUse || n.Target == nil {
				return
			}
			got := resolveSim(n, pathTo(f, n.Target))
			if len(got) != 1 || got[0] != n.Target {
				bad = append(bad, n)
			}
		})
	}
	return bad
}

// bump renames the first segment of the path of u's target.
func bump(u *Node) {
	t := u.Target
	if t.File != u.File {
		t.File.Alias += "_lib"
		return
	}
	for p := t; p != nil; p = p.Parent {
		if p.Parent != nil && p.Parent.Kind == KFile {
			p.Name += " top"
			dedupe(p.Parent.Kids)
			return
		}
	}
}

// finalizeNames makes the names unique where a path needs it and checks the paths.
func (c *conv) finalizeNames() {
	for _, f := range c.order {
		markNeeded(f)
	}
	for _, f := range c.order {
		dedupeAll(f)
	}
	for round := 0; round < 8; round++ {
		bad := shadowed(c.order)
		if len(bad) == 0 {
			return
		}
		for _, u := range bad {
			bump(u)
		}
	}
	for _, u := range shadowed(c.order) {
		u.File.warn("path %s does not reach its target", pathTo(u.File, u.Target))
	}
}
