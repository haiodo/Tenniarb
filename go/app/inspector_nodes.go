//go:build darwin || linux || windows

package app

import (
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/editor"
)

func bodyKind(v *Viewer) string {
	p := v.session.Prop("body")
	switch {
	case p.Mixed:
		return "Mixed"
	case !p.Set:
		return "None"
	}
	text := p.Arg(0).Text
	if strings.Contains(text, "image") {
		return "Image"
	}
	return "Markdown"
}

// nodeCfg is one kind of node in the sections both kinds show: where a row reads and writes, which
// rows apply, the id prefix its controls answer to, and whether its fields take words.
type nodeCfg struct {
	prefix  string
	src     func() propSrc
	words   bool // a size or a radius may be `hug`, `fill` or four corners, as a scene node's is
	layout  func(selKind) bool
	fill    func(selKind) bool
	text    func(selKind) bool
	border  func(selKind) bool
	shape   func(selKind) bool
	display func(selKind) bool
	layer   func(selKind) bool
}

func (c nodeCfg) id(row string) string { return c.prefix + "-" + row }

// entry is a field of the row; src nil follows the selection.
func (c nodeCfg) entry(row, name string) entryCfg {
	return entryCfg{id: c.id(row), name: name, src: c.src}
}

// num is a field that takes a number or an expression only.
func (c nodeCfg) num(row, name string, idx int) entryCfg {
	return entryCfg{id: c.id(row), name: name, idx: idx, numeric: true, src: c.src}
}

// size is a size or a corner radius: numeric for a classic item, words for a scene node.
func (c nodeCfg) size(row, name string) entryCfg {
	if c.words {
		return c.entry(row, name)
	}
	return c.num(row, name, 0)
}

// nodeRow is one row of a section both kinds share: the caption and the row itself.
type nodeRow struct {
	caption string
	build   func(in *inspector) func() g.Option
}

// layoutRows are the rows of Layout both a classic item and a scene node show.
func layoutRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Position", func(in *inspector) func() g.Option {
			x, y := c.num("pos-x", "pos", 0), c.num("pos-y", "pos", 1)
			x.keep, y.keep = true, true
			return in.pair("Position", "X", "Y", c.layout, x, y)
		}},
		{"Size", func(in *inspector) func() g.Option {
			return in.pair("Size", "W", "H", c.layout, c.size("width", "width"), c.size("height", "height"))
		}},
		{"W", func(in *inspector) func() g.Option {
			return in.sizeModes("W", c.id("width-mode"), "width", 0, c.layout, func() float64 { w, _ := in.v.itemSize(); return w }, c.src)
		}},
		{"H", func(in *inspector) func() g.Option {
			return in.sizeModes("H", c.id("height-mode"), "height", 0, c.layout, func() float64 { _, h := in.v.itemSize(); return h }, c.src)
		}},
		{"Padding", func(in *inspector) func() g.Option {
			cfg := c.entry("padding", "padding")
			cfg.list = true
			return in.text("Padding", c.layout, cfg)
		}},
	}
}

// fillRows are the rows of Fill both kinds show.
func fillRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Color", func(in *inspector) func() g.Option {
			return in.color("Color", c.fill, c.entry("color", "color"))
		}},
	}
}

// textRows are the rows of Text both kinds show; the alignment, the marker and the face are the
// kind's own.
func textRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Color", func(in *inspector) func() g.Option {
			return in.color("Color", c.text, c.entry("text-color", "text-color"))
		}},
		{"Size", func(in *inspector) func() g.Option {
			return in.text("Size", c.text, c.num("font-size", "font-size", 0))
		}},
	}
}

// borderRows are the rows of Border both kinds show; both engines read line-style, line-width and
// corner-radius of an item.
func borderRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Color", func(in *inspector) func() g.Option {
			return in.color("Color", c.border, c.entry("border-color", "border-color"))
		}},
		{"Width", func(in *inspector) func() g.Option {
			return in.text("Width", c.border, c.num("line-width", "line-width", 0))
		}},
		{"Style", func(in *inspector) func() g.Option {
			return in.combo("Style", c.border, c.id("line-style"), "line-style", []string{noneChoice, "solid", "dashed", "dotted"}, noneChoice, c.src)
		}},
		{"Radius", func(in *inspector) func() g.Option {
			return in.text("Radius", c.border, c.size("radius", "corner-radius"))
		}},
	}
}

// shapeRows are the rows of Shape both kinds show; `display` and `layer` are read by both engines.
func shapeRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Display", func(in *inspector) func() g.Option {
			return in.combo("Display", c.display, c.id("display"), "display", editor.ItemDisplay, "rect", c.src)
		}},
		{"Layer", func(in *inspector) func() g.Option {
			return in.combo("Layer", c.layer, c.id("layer"), "layer", []string{"default", "background", "hover"}, "default", c.src)
		}},
	}
}
