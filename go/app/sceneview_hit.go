//go:build darwin || linux || windows

package app

import (
	"slices"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// hitScene is the deepest item one can select under the content point p, nil on empty space.
func (sv *sceneView) hitScene(p gfx.Point) *scene.Placed {
	// A transparent layer yields to what it covers: the click goes to the node that draws something.
	if h := sv.paintedAt(p); h != nil {
		return h
	}
	// Nothing draws there: the deepest node whose box holds the point, as a layer with no fill of its own
	// still selects.
	var found *scene.Placed
	var walk func(pl *scene.Placed)
	walk = func(pl *scene.Placed) {
		for i := len(pl.Kids) - 1; i >= 0; i-- {
			k := pl.Kids[i]
			if p.X < k.X || p.Y < k.Y || p.X > k.X+k.W || p.Y > k.Y+k.H {
				continue
			}
			if _, ok := sv.pathOf[k]; ok {
				found = k
			}
			walk(k)
			return
		}
	}
	if sv.placed != nil {
		walk(sv.placed)
	}
	return found
}

// paintedAt is the deepest node at p that draws something of its own or holds one that does: what a press
// finds before a transparent layer over it.
func (sv *sceneView) paintedAt(p gfx.Point) *scene.Placed {
	var walk func(pl *scene.Placed) *scene.Placed
	walk = func(pl *scene.Placed) *scene.Placed {
		for i := len(pl.Kids) - 1; i >= 0; i-- {
			k := pl.Kids[i]
			if p.X < k.X || p.Y < k.Y || p.X > k.X+k.W || p.Y > k.Y+k.H {
				continue
			}
			if h := walk(k); h != nil {
				return h
			}
			if _, ok := sv.pathOf[k]; ok && painted(k) {
				return k
			}
		}
		return nil
	}
	if sv.placed == nil {
		return nil
	}
	return walk(sv.placed)
}

// painted reports whether the node draws something of its own: a fill, a border, an icon path, an image,
// a title or a shadow. Its children are the caller's business.
func painted(pl *scene.Placed) bool {
	for _, name := range [...]string{"color", "border-color", "path", "image", "title", "shadow"} {
		if pl.Prop(name) != nil {
			return true
		}
	}
	return false
}

// pick is what a click on hit selects. An instance is selected whole first: the outermost one above hit.
// A double click goes one level deeper toward hit; a single click never enters an instance, so a component
// is not taken for the text or the icon it is made of.
func (sv *sceneView) pick(hit, cur *scene.Placed, clicks int) *scene.Placed {
	var chain []*scene.Placed // from the top down to hit
	for c := hit; c != nil; c = c.Parent {
		if _, ok := sv.pathOf[c]; ok {
			chain = append(chain, c)
		}
	}
	slices.Reverse(chain)
	top := slices.IndexFunc(chain, func(c *scene.Placed) bool { return c.Inst })
	if top < 0 {
		return hit
	}
	at := slices.Index(chain, cur)
	if clicks >= 2 {
		if at >= 0 && at+1 < len(chain) {
			return chain[at+1]
		}
		if _, ok := sv.pathOf[hit]; ok {
			return hit
		}
	}
	return chain[top]
}

// movable is pl when its place comes from pos, else the nearest ancestor that has one: a click on a node
// inside an auto-layout moves the container it sits in, not the node.
func (sv *sceneView) movable(pl *scene.Placed) *scene.Placed {
	for n := pl; n != nil; n = n.Parent {
		if n.Free && n.Gfx != nil {
			if _, ok := sv.pathOf[n]; ok {
				return n
			}
		}
	}
	return nil
}
