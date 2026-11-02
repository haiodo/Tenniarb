package optimize

import (
	"fmt"
	"sort"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// dead reports the commands that change nothing (rule 4): a style nobody names, a group nobody chooses
// from, an override that sets the value the component already has, a pos a flow layout ignores.
func (a *analyzer) dead(root *scene.Node) {
	refs, dynamic := a.styleRefs(root)
	a.deadStyles(root, refs, dynamic)
	a.deadPos()
	a.deadOverrides(root)
}

// styleRefs lists, for every node whose styles its nodes may name, the style names they name with
// use-style; the names a use names belong to the component, not to the place it sits in. dynamic holds
// the nodes a use-style with an expression names: which style it picks is known only at build time, so
// no style of theirs is dead.
func (a *analyzer) styleRefs(root *scene.Node) (map[*scene.Node][]string, map[*scene.Node]bool) {
	out := map[*scene.Node][]string{}
	dynamic := map[*scene.Node]bool{}
	add := func(owner *scene.Node, props []*scene.Prop) {
		if sc := nearestStyles(owner); sc != nil {
			out[sc] = append(out[sc], scene.StyleRefs(props)...)
			for _, pr := range props {
				if isDynamic(pr) {
					dynamic[sc] = true
				}
			}
		}
	}
	a.eachNode(root, func(n *scene.Node) {
		if n.Kind != scene.KUse {
			add(n, n.Props)
			return
		}
		comp, err := a.tree.Resolve(n, n.Path)
		if err != nil || comp == nil {
			return // a broken path: validate reports it
		}
		for _, ov := range n.Block {
			switch {
			case ov.Prop != nil && ov.Prop.Name == "use-style":
				add(comp, []*scene.Prop{ov.Prop})
			case ov.Kind == scene.OvAt || ov.Kind == scene.OvReplace:
				add(a.instanceNode(comp, ov.Path), useStyleProps(ov.Block))
			}
		}
	})
	return out, dynamic
}

// isDynamic reports a use-style whose names are expressions (spec 6): a parameter or a value of the
// instance, not a literal.
func isDynamic(pr *scene.Prop) bool {
	for _, a := range pr.Args {
		if a.Kind == tenn.KindExpression || a.Kind == tenn.KindExpressionBlock {
			return true
		}
	}
	return false
}

// useStyleProps are the direct use-style commands of an override block.
func useStyleProps(ovs []*scene.Override) []*scene.Prop {
	var out []*scene.Prop
	for _, ov := range ovs {
		if ov.Prop != nil && ov.Prop.Name == "use-style" {
			out = append(out, ov.Prop)
		}
	}
	return out
}

// nearestStyles is the node whose styles a node names: itself when it holds styles, else the nearest
// ancestor that does (scene's styleScope, which starts at the parent).
func nearestStyles(n *scene.Node) *scene.Node {
	for p := n; p != nil; p = p.Parent {
		if len(p.Styles) > 0 || len(p.StyleGroups) > 0 {
			return p
		}
	}
	return nil
}

// instanceNode follows an at/replace path along the tree as written in the component. A path through a
// nested use is not followed: its children come from another component.
func (a *analyzer) instanceNode(comp *scene.Node, path string) *scene.Node {
	segs, ok := scene.SplitPath(path)
	if !ok {
		return nil
	}
	n := comp
	for _, seg := range segs {
		if n == nil || n.Kind == scene.KUse {
			return nil
		}
		n = instanceChild(n, seg)
	}
	return n
}

// instanceChild is the direct child of a component a segment of an at path names; nil when there is no
// such child or more than one (validate reports the ambiguity).
func instanceChild(comp *scene.Node, seg scene.Segment) *scene.Node {
	var found *scene.Node
	for _, c := range comp.Nodes() {
		if !seg.Matches(c) {
			continue
		}
		if found != nil {
			return nil
		}
		found = c
	}
	return found
}

// implicitStyle are the style names the classic layer applies without use-style: "item" is the default
// look of every item and "line" of every link (render/style.go, SceneStyle.parseLine).
var implicitStyle = map[string]bool{"item": true, "line": true}

// deadStyles reports the named styles and the groups of an element that no use-style names.
func (a *analyzer) deadStyles(root *scene.Node, refs map[*scene.Node][]string, dynamic map[*scene.Node]bool) {
	var each func(n *scene.Node)
	each = func(n *scene.Node) {
		for _, e := range n.Elems {
			each(e)
		}
		if a.other(n) || n.Mount != nil {
			return // the styles of a mounted file belong to that file
		}
		if dynamic[n] {
			return // a use-style with an expression may name any of them
		}
		used := map[string]bool{}
		for _, name := range refs[n] {
			used[name] = true
		}
		names := make([]string, 0, len(n.Styles))
		for name := range n.Styles {
			names = append(names, name)
		}
		sort.Strings(names)
		// a style a group offers is reachable from the inspector; a group nobody chooses from is
		// reported as a whole
		inGroup := map[string]bool{}
		for _, g := range n.StyleGroups {
			for _, name := range g.Styles {
				inGroup[name] = true
			}
		}
		for _, name := range names {
			if used[name] || implicitStyle[name] || inGroup[name] {
				continue
			}
			f := a.note("dead-style", n.Pos.File, n.Range.Start,
				[]string{nodePath(n) + " -> " + name},
				fmt.Sprintf("style %q is never named with use-style: delete it or use it", name),
				"a style nobody names", 0)
			f.elem, f.name = n, name
		}
		for _, g := range n.StyleGroups {
			chosen := false
			for _, name := range g.Styles {
				if used[name] {
					chosen = true
					break
				}
			}
			if chosen {
				continue
			}
			f := a.note("dead-group", n.Pos.File, n.Range.Start,
				[]string{nodePath(n) + " -> " + g.Name},
				fmt.Sprintf("no node chooses a member of group %q (%d members): delete the group or one of its styles", g.Name, len(g.Styles)),
				"a group nobody chooses from", 0)
			f.elem, f.name = n, g.Name
		}
	}
	each(root)
}

// deadPos reports a pos a flow layout ignores: with auto-layout row or column the parent places its
// children itself, and pos only counts for an absolute child or in auto-layout abs (spec 5).
func (a *analyzer) deadPos() {
	for _, c := range a.cands {
		if !hasProp(writtenProps(c.n), "pos") {
			continue
		}
		p := c.n.Parent
		if p == nil || p.Kind != scene.KItem {
			continue
		}
		parent := effectiveProps(p)
		if !flowDir(parent) || flagOn(effectiveProps(c.n), "absolute") {
			continue
		}
		f := a.note("dead-pos", c.n.Pos.File, c.n.Range.Start,
			[]string{a.withPos(c.n)},
			fmt.Sprintf("%s sits in a %s layout, so its pos is ignored: drop the pos, or make the item absolute", nodePath(c.n), flowDirName(parent)),
			"a command the layout ignores", 0)
		if own := findProp(c.n.Props, "pos"); own != nil {
			f.del = &own.Range
		}
	}
}

// flowDir reports whether props place the children in a row or a column.
func flowDir(props []propVal) bool {
	for _, p := range props {
		if p.name != "auto-layout" {
			continue
		}
		return len(p.args) == 0 || p.args[0] == "row" || p.args[0] == "column"
	}
	return false
}

func flowDirName(props []propVal) string {
	for _, p := range props {
		if p.name == "auto-layout" {
			if len(p.args) == 0 {
				return "row"
			}
			return p.args[0]
		}
	}
	return "row"
}

func flagOn(props []propVal, name string) bool {
	for _, p := range props {
		if p.name == name && len(p.args) > 0 {
			return p.args[0] == "yes"
		}
	}
	return false
}

func hasProp(props []propVal, name string) bool {
	for _, p := range props {
		if p.name == name {
			return true
		}
	}
	return false
}

// deadOverrides reports the overrides of a use that change nothing: an empty block, a value the
// component already has at that path.
func (a *analyzer) deadOverrides(root *scene.Node) {
	a.eachNode(root, func(n *scene.Node) {
		if n.Kind != scene.KUse {
			return
		}
		for _, ov := range n.Block {
			switch {
			case (ov.Kind == scene.OvAt || ov.Kind == scene.OvReplace) && len(ov.Block) == 0:
				word := "at"
				if ov.Kind == scene.OvReplace {
					word = "replace"
				}
				f := a.note("dead-override", ov.Pos.File, ov.Range.Start,
					[]string{nodePath(n) + " -> " + word + " " + ov.Path},
					fmt.Sprintf("%s %s of the instance has an empty block: delete it", word, ov.Path),
					"an override that does nothing", 0)
				f.del = &ov.Range
			case ov.Kind == scene.OvAt && a.sameAsDefinition(n, ov):
				f := a.note("dead-override", ov.Pos.File, ov.Range.Start,
					[]string{nodePath(n) + " -> at " + ov.Path},
					fmt.Sprintf("at %s sets the values the component already has: delete the override", ov.Path),
					"an override that changes nothing", 0)
				f.del = &ov.Range
			}
		}
	})
}

// sameAsDefinition reports whether every property of an at block repeats a value written on the node it
// addresses, so the block changes nothing.
func (a *analyzer) sameAsDefinition(use *scene.Node, ov *scene.Override) bool {
	comp, err := a.tree.Resolve(use, use.Path)
	if err != nil || comp == nil {
		return false
	}
	target := a.instanceNode(comp, ov.Path)
	if target == nil {
		return false
	}
	if len(scene.StyleRefs(target.Props)) > 0 {
		return false // a style may win over the written value, so the same value can still change the look
	}
	for _, o := range ov.Block {
		if o.Prop == nil {
			return false // a node or another at inside: not a plain value override
		}
		own := findProp(target.Props, o.Prop.Name)
		if own == nil || !sameArgs(propArgs(own), propArgs(o.Prop)) {
			return false
		}
	}
	return true
}

func findProp(props []*scene.Prop, name string) *scene.Prop {
	for _, p := range props {
		if p.Name == name {
			return p
		}
	}
	return nil
}
