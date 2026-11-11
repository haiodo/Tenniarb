//go:build darwin || linux || windows

package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
)

// instRows are the rows under an item drawn from an instance: what is drawn in it, written in the
// element or a part of the component.
func (sv *sceneView) instRows(pl *scene.Placed) []ItemRow {
	var out []ItemRow
	for _, k := range pl.Kids {
		p, ok := sv.pathOf[k]
		if !ok {
			continue
		}
		kids := len(k.Kids) > 0 || sv.written(k) && len(editor.SceneKids(k.Src)) > 0
		out = append(out, ItemRow{Label: nodeLabel(k.Src), Kind: k.Src.Kind.String(), Path: p, Kids: kids})
	}
	return out
}

// rowChain are the paths of the rows from the top down to the one of path p, as the tree nests them.
func (sv *sceneView) rowChain(p string) []string {
	var chain []string
	for ; p != ""; p = sv.rowParent(p) {
		chain = append(chain, p)
	}
	slices.Reverse(chain)
	return chain
}

// opensParts reports whether the row of p lists the drawn kids of an instance or of one of its parts; the
// other rows list what is written under them (see rowsAt).
func (sv *sceneView) opensParts(p string) bool {
	pl := sv.byPath[p]
	return pl != nil && (pl.Inst || strings.Contains(p, "/"))
}

// rowParent is the path of the row that holds the row of p, "" for a top row. Under an instance a row
// sits under the nearest drawn item above it, so what a use passes in sits under the part of the
// component it is drawn in, not under the use; elsewhere it nests as it is written.
func (sv *sceneView) rowParent(p string) string {
	if pl := sv.byPath[p]; pl != nil {
		for a := pl.Parent; a != nil; a = a.Parent {
			ap, ok := sv.pathOf[a]
			if !ok {
				continue
			}
			if strings.Contains(p, "/") || sv.opensParts(ap) {
				return ap
			}
			break
		}
	}
	if i := strings.LastIndex(p, "."); i >= 0 && !strings.Contains(p, "/") {
		return p[:i]
	}
	return ""
}

func slotSeg(sl *scene.Node) string {
	if sl.Key == "" {
		return "default"
	}
	return sl.Key
}

// nodeSegOf is the key of an item among the drawn siblings, with #n for the n-th repeat.
func nodeSegOf(p *scene.Placed) string {
	k := 0
	if p.Parent != nil {
		for _, c := range p.Parent.Kids {
			if c == p {
				break
			}
			if c.Src.Key == p.Src.Key {
				k++
			}
		}
	}
	if k > 0 {
		return p.Src.Key + "#" + strconv.Itoa(k)
	}
	return p.Src.Key
}

func nearestInst(p *scene.Placed) *scene.Placed {
	for ; p != nil; p = p.Parent {
		if p.Inst {
			return p
		}
	}
	return nil
}

// written reports whether p is an item written in the element (a path without "/").
func (sv *sceneView) written(p *scene.Placed) bool {
	path, ok := sv.pathOf[p]
	return ok && !strings.Contains(path, "/")
}

func under(n, owner *scene.Node) bool {
	for c := n.Parent; c != nil; c = c.Parent {
		if c == owner {
			return true
		}
	}
	return false
}
