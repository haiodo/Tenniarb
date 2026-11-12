package penimport

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"

	"github.com/haiodo/tenn/go"
	"github.com/haiodo/tenniarb/go/library"
)

// A pen icon node names an icon of a library and carries no geometry of its own: the drawing is the
// vector of that name. go/scene draws a leaf with a `path` and a `view-box` and knows no icon property,
// so the converter writes the path of the glyph into the item, and the .tenn file stays readable on a
// machine without the icons library.
//
// The glyphs are the items of libraries/icons.tenn, the library of the lucide set the application
// ships: the same file a document mounts with --from-file "tenniarb-library://icons".

// defaultViewBox and defaultStroke are the view box and stroke of the lucide set, used when the
// library cannot be read: the converter still writes the room of an icon.
var defaultViewBox = [4]float64{0, 0, 24, 24}

const defaultStroke = 2

// glyph is the drawing of one item of the icons library.
type glyph struct {
	// paths are the shapes of the glyph in the order the library writes them: the contour stroked, and a
	// filled detail - the dot of tag or key-round - as a path of its own with filled set.
	paths   []glyphPath
	viewBox [4]float64
	stroke  float64
}

// glyphPath is one shape of a glyph: filled means the library fills it instead of stroking it.
type glyphPath struct {
	d      string
	filled bool
}

// cmdHasWord reports whether a command names the word among its arguments: `path "M…" color #000` fills
// its shape instead of stroking it.
func cmdHasWord(args []*tenn.Node, word string) bool {
	for _, a := range args {
		if a == nil {
			continue
		}
		if w, ok := a.GetIdentText(); ok && w == word {
			return true
		}
	}
	return false
}

// iconTable is libraries/icons.tenn read once: its glyphs, the view box and stroke of the set, and the
// error of reading it.
type iconTable struct {
	glyphs  map[string]glyph
	viewBox [4]float64
	stroke  float64
	err     error
}

var (
	iconsOnce  sync.Once
	iconsTable iconTable
)

// iconSet reads the library once per process. The folders are the ones a document resolves a scheme in,
// so the converter and the scene take the icons from the same file.
func iconSet() iconTable {
	iconsOnce.Do(func() {
		iconsTable = readIcons()
	})
	return iconsTable
}

func readIcons() iconTable {
	set := iconTable{viewBox: defaultViewBox, stroke: defaultStroke}
	file, err := library.Default("").Resolve(library.Path{Name: "icons"})
	if err != nil {
		set.err = fmt.Errorf("the icons library is not readable: %w", err)
		return set
	}
	src, err := os.ReadFile(file)
	if err != nil {
		set.err = fmt.Errorf("the icons library is not readable: %w", err)
		return set
	}
	glyphs, err := parseIcons(string(src))
	if err != nil {
		set.err = fmt.Errorf("libraries/icons.tenn: %w", err)
		return set
	}
	set.glyphs = glyphs
	for _, g := range glyphs { // every item of the set carries the same box and stroke
		set.viewBox, set.stroke = g.viewBox, g.stroke
		break
	}
	return set
}

// parseIcons reads the items of the library: one item per icon, with its path, view box and stroke. The
// text is written by cmd/tennicons, and the parser of the format reads it like any other .tenn file.
func parseIcons(src string) (map[string]glyph, error) {
	ast, parser := tenn.Parse(src)
	if len(parser.Errors.Errors) > 0 {
		e := parser.Errors.Errors[0]
		return nil, fmt.Errorf("line %d: %s", e.Line+1, e.Message)
	}
	glyphs := map[string]glyph{}
	for _, c := range ast.Children {
		if c.Kind != tenn.KindCommand {
			continue
		}
		name, _ := c.GetIdent(0)
		if name != "item" {
			continue
		}
		args, block := splitCommand(c)
		if len(args) == 0 {
			continue
		}
		item, _ := args[0].GetIdentText()
		g := glyph{viewBox: defaultViewBox, stroke: defaultStroke}
		for _, p := range block {
			if p.Kind != tenn.KindCommand {
				continue
			}
			pname, _ := p.GetIdent(0)
			pargs, _ := splitCommand(p)
			switch pname {
			case "path":
				if d, ok := argIdentAt(pargs, 0); ok {
					g.paths = append(g.paths, glyphPath{d: d, filled: cmdHasWord(pargs, "color")})
				}
			case "view-box":
				for i := 0; i < 4; i++ {
					if v, ok := argNumAt(pargs, i); ok {
						g.viewBox[i] = v
					}
				}
			case "line-width":
				if v, ok := argNumAt(pargs, 0); ok {
					g.stroke = v
				}
			}
		}
		if item != "" && len(g.paths) > 0 {
			glyphs[item] = g
		}
	}
	if len(glyphs) == 0 {
		return nil, fmt.Errorf("no icons in the library")
	}
	return glyphs, nil
}

// splitCommand separates the arguments of a command from the commands of its block.
func splitCommand(c *tenn.Node) (args, block []*tenn.Node) {
	for _, a := range c.Children[1:] {
		if a.Kind == tenn.KindBlockExpr {
			block = append(block, a.Children...)
		} else {
			args = append(args, a)
		}
	}
	return args, block
}

func argIdentAt(args []*tenn.Node, i int) (string, bool) {
	if i >= len(args) {
		return "", false
	}
	return args[i].GetIdentText()
}

func argNumAt(args []*tenn.Node, i int) (float64, bool) {
	s, ok := argIdentAt(args, i)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// iconProps is the look of a pen icon: the path of the glyph, its view box, the stroke the pen fill
// colours and the width of that stroke in the units of the item.
//
// A ref to an icon component carries an icon name of its own - the instances of one size preset show
// different glyphs - so the same function writes the override of a use: path, view box and stroke.
func (c *conv) iconProps(f *File, m map[string]any, x pctx) []Prop {
	if lib, _ := m["library"].(string); lib != "" && lib != "lucide" {
		f.unsup("icon library " + lib)
		return nil
	}
	set := iconSet()
	if set.err != nil {
		f.iconLibraryUnreadable(set.err)
		return nil
	}
	name, _ := m["icon"].(string)
	var ps []Prop
	box, stroke := set.viewBox, set.stroke
	fill, hasFill := "", false
	if name != "" {
		g, ok := set.glyphs[name]
		if !ok {
			f.iconNotFound(name)
			return nil
		}
		box, stroke = g.viewBox, g.stroke
		fill, hasFill = c.paint(f, m["fill"], x.mode, nil, true)
		width, hasWidth := iconStroke(box, stroke, m, x)
		ps = append(ps, prop("view-box", numArg(box[0]), numArg(box[1]), numArg(box[2]), numArg(box[3])))
		// The contour is stroked in the colour of the pen fill - lucide draws every icon with
		// stroke=currentColor and no fill - and a filled detail takes that colour as a fill.
		for _, gp := range g.paths {
			if gp.filled {
				if hasFill {
					ps = append(ps, prop("path", strArg(gp.d)), prop("color", wordArg(fill)))
				}
				continue
			}
			ps = append(ps, prop("path", strArg(gp.d)))
			if hasFill {
				ps = append(ps, prop("border-color", wordArg(fill)))
			}
			if hasWidth {
				ps = append(ps, prop("line-width", numArg(width)))
			}
		}
		return ps
	}
	if fill, ok := c.paint(f, m["fill"], x.mode, nil, true); ok {
		ps = append(ps, prop("border-color", wordArg(fill)))
	}
	if w, ok := iconStroke(box, stroke, m, x); ok {
		ps = append(ps, prop("line-width", numArg(w)))
	}
	return ps
}

// iconStroke is the width of the stroke of the glyph in the units of the item. go/scene scales the view
// box to the box of the item but strokes the scaled path with a width of the item, so the width of the
// library has to be scaled by hand, or a small icon draws with a stroke too thick for it.
//
// An override writes the width only when it changes the size of the instance: otherwise the component
// already carries the width of that size.
func iconStroke(box [4]float64, stroke float64, m map[string]any, x pctx) (w float64, ok bool) {
	src := m
	vw, vh := box[2], box[3]
	if x.explicit {
		_, hasW := m["width"]
		_, hasH := m["height"]
		if !hasW && !hasH {
			return 0, false
		}
		if x.eff != nil {
			src = x.eff
		}
	}
	s := 1.0 // the icon takes the size of its view box when the box of the item is not written
	if iw, wok := src["width"].(float64); wok && iw > 0 {
		if ih, hok := src["height"].(float64); hok && ih > 0 {
			s = math.Min(iw/vw, ih/vh)
		}
	}
	// Two decimals are plenty for the width of a stroke and keep the numbers of a .tenn short.
	return math.Round(stroke*s*100) / 100, true
}

// iconLibraryUnreadable warns once per file when the library itself is missing: every icon of the file
// would otherwise be an empty item with no word about the cause.
func (f *File) iconLibraryUnreadable(err error) {
	const key = "icons library"
	if f.once[key] {
		return
	}
	if f.once == nil {
		f.once = map[string]bool{}
	}
	f.once[key] = true
	f.warn("%v: the icons keep their room and are left empty", err)
}

// iconNotFound warns once per name, not once per use: a screen may hold a hundred instances of a
// component whose glyph the library does not have, and one line about the name says it.
func (f *File) iconNotFound(name string) {
	if f.missingIcons[name] {
		return
	}
	if f.missingIcons == nil {
		f.missingIcons = map[string]bool{}
	}
	f.missingIcons[name] = true
	f.warn("icon %q is not in the icons library (libraries/icons.tenn): the item keeps its room and is left empty", name)
}
