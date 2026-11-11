//go:build darwin || linux || windows

package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
)

// splitIndex reads "name#n" as PathOf writes it.
func splitIndex(seg string) (string, int) {
	if i := strings.LastIndex(seg, "#"); i >= 0 {
		if k, err := strconv.Atoi(seg[i+1:]); err == nil && k > 0 {
			return seg[:i], k
		}
	}
	return seg, 0
}

// nodeSeg is the segment of n among the kids of its parent: its key, with "#n" for the n-th repeat.
func nodeSeg(kids []*scene.Node, n *scene.Node) string {
	k := 0
	for _, c := range kids {
		if c == n {
			break
		}
		if c.Key == n.Key {
			k++
		}
	}
	if k > 0 {
		return n.Key + "#" + strconv.Itoa(k)
	}
	return n.Key
}

// NodePath names n by the segments from the element el down, joined by dots; "" when n is not written
// inside el.
func NodePath(el, n *scene.Node) string {
	var segs []string
	// The content of an element with --from-file is the file's: its top nodes have the file root as the parent.
	var mount *scene.Node
	if el != nil {
		mount = el.Mount
	}
	for c := n; c != el && c != mount; c = c.Parent {
		if c == nil || c.Parent == nil {
			return ""
		}
		segs = append(segs, nodeSeg(editor.SceneKids(c.Parent), c))
	}
	slices.Reverse(segs)
	return strings.Join(segs, ".")
}

// NodeAt finds the node of a path that NodePath wrote under el; nil when it is gone.
func NodeAt(el *scene.Node, path string) *scene.Node {
	if el == nil || path == "" {
		return nil
	}
	segs, ok := scene.SplitPath(path)
	if !ok {
		return nodeAtKeys(el, path) // a path of NodePath with an empty name, "Page."
	}
	cur := el
	for _, seg := range segs {
		var next *scene.Node
		if seg.Quoted {
			for _, c := range editor.SceneKids(cur) {
				if seg.Matches(c) {
					next = c
					break
				}
			}
		} else {
			key, k := splitIndex(seg.Name)
			for _, c := range editor.SceneKids(cur) {
				if c.Key == key {
					if k == 0 {
						next = c
						break
					}
					k--
				}
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// nodeAtKeys is NodeAt for a path of keys with empty segments, which NodePath writes for a node
// without a name.
func nodeAtKeys(el *scene.Node, path string) *scene.Node {
	cur := el
	for _, seg := range strings.Split(path, ".") {
		key, k := splitIndex(seg)
		var next *scene.Node
		for _, c := range editor.SceneKids(cur) {
			if c.Key == key {
				if k == 0 {
					next = c
					break
				}
				k--
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}
