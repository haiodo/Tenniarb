package editor

import "github.com/haiodo/tenniarb/go/scene"

// SceneKids are the nodes a node path goes through under n: the items and uses of an element, the
// items, uses and slots of an item, everything a use passes in its block. Links have no name to go by.
func SceneKids(n *scene.Node) []*scene.Node {
	var out []*scene.Node
	for _, c := range n.Content {
		switch c.Kind {
		case scene.KItem, scene.KUse:
			out = append(out, c)
		case scene.KSlot:
			if n.Kind == scene.KItem {
				out = append(out, c)
			}
		}
	}
	return out
}
