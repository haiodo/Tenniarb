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

// Resolve finds the node a path names from scope, by the rules of spec 4.2/4.3: the first segment
// is looked up from scope up, the rest is followed from there. Names are normalized. The error
// says what is wrong with the path; nothing is added to the diagnostics.
func (t *Tree) Resolve(scope *Node, path string) (*Node, error) {
	n, msg := lookup(scope, path)
	if msg != "" {
		return nil, errors.New(msg)
	}
	return n, nil
}

// resolve is Resolve that reports a failure at pos.
func (t *Tree) resolve(scope *Node, path string, pos Pos) *Node {
	n, msg := lookup(scope, path)
	if msg != "" {
		t.fail(pos, "%s", msg)
	}
	return n
}

// lookup returns the node, or nil and the message of the problem.
func lookup(scope *Node, path string) (*Node, string) {
	segs, ok := splitPath(path)
	if !ok {
		return nil, fmt.Sprintf("bad path %q", path)
	}
	for a := scope; a != nil; a = a.Parent {
		cur := pick(a.named(), segs[0])
		if len(cur) == 0 {
			continue
		}
		for _, seg := range segs[1:] {
			var next []*Node
			for _, c := range cur {
				next = append(next, pick(c.named(), seg)...)
			}
			if len(next) == 0 {
				return nil, fmt.Sprintf("path %s: nothing named %q under %s", path, seg, where(cur))
			}
			cur = next
		}
		if len(cur) > 1 {
			return nil, fmt.Sprintf("ambiguous path %s: candidates %s", path, where(cur))
		}
		return cur[0], ""
	}
	return nil, fmt.Sprintf("path %s: %q is not defined here or above", path, segs[0])
}
