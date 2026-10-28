package penimport

import (
	"strconv"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Properties that repeat what go/scene assumes are dropped. Inside a layout (an auto-layout on the item
// or on its parent) an item has no fill and no border; outside, a white fill and a black border of 1.

func hasCmd(cmds []*tenn.Node, name string) bool {
	for _, c := range cmds {
		if cmdName(c) == name {
			return true
		}
	}
	return false
}

func hasKids(cmds []*tenn.Node) bool {
	return hasCmd(cmds, "item") || hasCmd(cmds, "use") || hasCmd(cmds, "slot")
}

// containerProps are read from an item only when it has an auto-layout.
var containerProps = [...]string{"padding", "gap", "line-gap", "wrap", "align", "justify"}

// defaultValue tells whether the plain command p of an item holds the default of its context.
func defaultValue(p *tenn.Node, cmds []*tenn.Node, parentAuto bool) bool {
	auto := hasCmd(cmds, "auto-layout")
	switch cmdName(p) {
	case "color":
		return argText(p, 1) == "#00000000" && (auto || parentAuto)
	case "line-width":
		w, err := strconv.ParseFloat(argText(p, 1), 64)
		return err == nil && (w == 1 || (auto || parentAuto) && !hasCmd(cmds, "border-color"))
	case "text-color":
		return argText(p, 1) == "#000000" && !hasCmd(cmds, "color")
	case "auto-layout":
		if argText(p, 1) != "row" || !parentAuto || hasKids(cmds) {
			return false
		}
		for _, name := range containerProps {
			if hasCmd(cmds, name) {
				return false
			}
		}
		return true
	}
	return false
}

// dropDefaults walks cmds, the content of a parent with an auto-layout when parentAuto. The text does
// not tell where the content of a use lands in the component: it counts as outside a layout.
func dropDefaults(cmds []*tenn.Node, parentAuto bool) {
	for _, c := range cmds {
		blk := blockOf(c)
		if blk == nil {
			continue
		}
		switch cmdName(c) {
		case "item":
			kept := blk.Children[:0:0]
			auto := hasCmd(blk.Children, "auto-layout") || parentAuto
			for _, p := range blk.Children {
				if isStructure(cmdName(p)) || !defaultValue(p, blk.Children, parentAuto) {
					kept = append(kept, p)
				}
			}
			dropDefaults(kept, auto)
			blk.Children = kept
		case "slot":
			dropDefaults(blk.Children, parentAuto)
		default:
			dropDefaults(blk.Children, false)
		}
	}
}

func cmdName(c *tenn.Node) string {
	s, _ := c.GetIdent(0)
	return s
}

func blockOf(c *tenn.Node) *tenn.Node {
	if n := len(c.Children); n > 1 && c.Children[n-1].Kind == tenn.KindBlockExpr {
		return c.Children[n-1]
	}
	return nil
}

func isStructure(name string) bool {
	switch name {
	case "item", "use", "at", "replace", "slot", "element":
		return true
	}
	return false
}

func argText(c *tenn.Node, i int) string {
	if i >= len(c.Children) {
		return ""
	}
	s, _ := c.Children[i].GetIdentText()
	return s
}
