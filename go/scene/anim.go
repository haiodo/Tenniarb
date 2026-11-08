package scene

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/tenn"
)

// animatedProps are the properties a keyframe may set (spec 13): appearance only, so the layout never
// has to run per frame.
var animatedProps = map[string]bool{
	"rotate": true, "scale": true, "opacity": true, "color": true, "border-color": true,
	"text-color": true, "pos": true,
}

// layoutProps are the properties a frame may not set: they need a layout of the whole scene per frame.
var layoutProps = map[string]bool{
	"width": true, "height": true, "min-width": true, "max-width": true, "min-height": true,
	"max-height": true, "gap": true, "line-gap": true, "padding": true, "auto-layout": true,
	"wrap": true, "align": true, "justify": true, "absolute": true,
}

// easings is the easing words of an animation or a transition, as CSS names them.
var easings = map[string]bool{"linear": true, "ease": true, "ease-in": true, "ease-out": true, "ease-in-out": true}

// declShape tells what a property takes, for the warning about a frame value that does not read.
var declShape = map[string]string{
	"rotate": "degrees", "scale": "1 or 2 numbers", "opacity": "0..1", "pos": "two numbers",
	"color": "a color", "border-color": "a color", "text-color": "a color",
}

// Frame is one keyframe of an animation: where it is on the timeline and what it sets there.
type Frame struct {
	Percent string  // from, to or a percentage, as written: CSS takes the same words
	Offset  float64 // 0..1; the frames of an animation are sorted by it
	Props   []*Prop // what the frame sets, used as the still picture when it is the first frame
}

// Animation is one animation of an item (spec 13): the frames of a keyframes block of the element and
// the duration, easing and repeat its animation command plays them with.
type Animation struct {
	Name     string
	Duration time.Duration
	Easing   string // one of easings; "ease" when not written, as in CSS
	Repeat   int    // 0 or 1: once; -1: forever; n: n times
	Frames   []Frame
}

// duration is the duration as CSS writes it: 1s, 300ms.
func (a *Animation) duration() string {
	if a.Duration%time.Second == 0 {
		return fmt.Sprintf("%ds", a.Duration/time.Second)
	}
	return fmt.Sprintf("%gms", float64(a.Duration)/float64(time.Millisecond))
}

// shorthand is the value of the CSS animation property that plays a.
func (a *Animation) shorthand() string {
	s := a.Name + " " + a.duration() + " " + a.Easing
	switch {
	case a.Repeat < 0:
		return s + " infinite"
	case a.Repeat > 1:
		return s + " " + strconv.Itoa(a.Repeat)
	}
	return s
}

// css is the body of the @keyframes step of the frame.
func (f *Frame) css() string {
	var out []string
	for _, pr := range f.Props {
		if d := decl(pr); d != "" {
			out = append(out, d)
		}
	}
	return strings.Join(out, ";")
}

// decl is one keyframe property as a CSS declaration, "" when the property is not animated or its value
// does not read as one. rotate, scale and translate are the individual CSS properties: they add to the
// transform of the group instead of replacing it, so the item keeps the place the layout gave it.
func decl(pr *Prop) string {
	switch pr.Name {
	case "rotate":
		if v, ok := pr.num(0); ok {
			return fmt.Sprintf("rotate:%gdeg", v)
		}
	case "scale":
		if v, ok := pr.num(0); ok {
			if w, ok := pr.num(1); ok {
				return fmt.Sprintf("scale:%g %g", v, w)
			}
			return fmt.Sprintf("scale:%g", v)
		}
	case "opacity":
		if v, ok := pr.num(0); ok {
			return fmt.Sprintf("opacity:%g", v)
		}
	case "pos":
		if x, okx := pr.num(0); okx {
			if y, oky := pr.num(1); oky {
				return fmt.Sprintf("translate:%gpx %gpx", x, -y) // Y is up in .tenn, down in CSS
			}
		}
	case "color", "text-color", "border-color":
		s, _ := pr.text(0)
		c, ok := parseColor(s)
		if !ok {
			return ""
		}
		name := "fill"
		if pr.Name == "border-color" {
			name = "stroke"
		}
		return name + ":" + cssColor(c)
	}
	return ""
}

// cssColor is c as CSS rgb(), the three channels gfx also writes into the SVG.
func cssColor(c gfx.Color) string {
	q := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return fmt.Sprintf("rgb(%d,%d,%d)", q(c.R), q(c.G), q(c.B))
}

// parseDuration reads a CSS time: 300ms, 1s, 1.5s.
func parseDuration(s string) (time.Duration, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	unit := time.Second
	switch {
	case strings.HasSuffix(s, "ms"):
		unit, s = time.Millisecond, s[:len(s)-2]
	case strings.HasSuffix(s, "s"):
		s = s[:len(s)-1]
	default:
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return time.Duration(v * float64(unit)), true
}

// frameOffset reads a frame selector: from, to or a percentage 0..100%.
func frameOffset(sel string) (float64, bool) {
	switch sel {
	case "from":
		return 0, true
	case "to":
		return 1, true
	}
	if !strings.HasSuffix(sel, "%") {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(sel, "%"), 64)
	if err != nil || v < 0 || v > 100 {
		return 0, false
	}
	return v / 100, true
}

// animations resolves the animations of rn and of its kids (spec 13). A keyframes block written on the
// node or above it gives the frames an animation command plays; the first frame is then written into the
// node as if the file said it, so the still picture of PNG and SVG is that frame. It runs before the
// layout, as a frame may set pos and opacity.
func (p *prep) animations(rn *RNode, defs map[string]*Animation) {
	local, own := defs, false
	declared := map[string]bool{}
	for _, pr := range p.allProps(rn, "animation") {
		if pr.Block == nil {
			continue
		}
		a := p.keyframes(pr, rn)
		if a == nil {
			continue
		}
		if !own {
			local, own = copyDefs(defs), true
		}
		if declared[a.Name] {
			p.t.warn(pr.Pos, "animation %s: the frames are declared twice, the last one is used", a.Name)
		}
		declared[a.Name] = true
		local[a.Name] = a
	}
	for _, pr := range p.allProps(rn, "animation") {
		if pr.Block != nil {
			continue
		}
		a := p.animationUse(pr, local)
		if a == nil {
			continue
		}
		p.anim[rn] = a
		rn.Props = append(rn.Props, a.Frames[0].Props...)
	}
	for _, pr := range p.allProps(rn, "transition") {
		p.transition(pr)
	}
	for _, k := range rn.Kids {
		p.animations(k, local)
	}
}

func copyDefs(defs map[string]*Animation) map[string]*Animation {
	out := make(map[string]*Animation, len(defs)+1)
	for k, v := range defs {
		out[k] = v
	}
	return out
}

// allProps are the properties of that name on rn, evaluated (spec 6): a node may name several
// animations, and the last one of a name is not enough.
func (p *prep) allProps(rn *RNode, name string) []*Prop {
	var out []*Prop
	for _, pr := range rn.Props {
		if pr.Name != name {
			continue
		}
		if hasExpr(pr) {
			if !p.canEval() { // a value check cannot know; the build with an evaluator reports it
				continue
			}
			if pr = p.evalProp(rn, pr); pr == nil {
				continue
			}
		}
		out = append(out, pr)
	}
	return out
}

// keyframes reads "animation <name> { <frame> { properties } ... }": the frames every node that names the
// animation plays, as a CSS @keyframes block. A frame property may be an expression (spec 6), evaluated in
// the scope of rn, the node the block is written on.
func (p *prep) keyframes(pr *Prop, rn *RNode) *Animation {
	name, _ := pr.text(0)
	if name == "" {
		p.t.warn(pr.Pos, "animation: the block needs a name, as in animation spin { from { ... } to { ... } }")
		return nil
	}
	a := &Animation{Name: name}
	for _, c := range pr.Block.Children {
		if c.Kind != tenn.KindCommand {
			continue
		}
		sel := cmdName(c)
		off, ok := frameOffset(sel)
		if !ok {
			p.t.warn(nodePos(pr.Pos.File, c), "animation %s: frame %q is not from, to or a percentage", name, sel)
			continue
		}
		f := Frame{Percent: sel, Offset: off}
		_, block := split(c)
		for _, pc := range block {
			pn := cmdName(pc)
			if pn == "" {
				continue
			}
			args, _ := split(pc)
			fp := &Prop{Name: pn, Args: args, Pos: nodePos(pr.Pos.File, pc)}
			if hasExpr(fp) {
				if p.eval == nil { // a value check cannot know; the build with an evaluator reports it
					continue
				}
				if fp = p.evalProp(rn, fp); fp == nil {
					continue
				}
			}
			switch {
			case decl(fp) != "":
				f.Props = append(f.Props, fp)
			case layoutProps[pn]:
				p.t.warn(fp.Pos, "animation %s: %s is a layout property and is not animated, the frame is ignored", name, pn)
			case animatedProps[pn]:
				p.t.warn(fp.Pos, "animation %s: %s takes %s, the frame is ignored", name, pn, declShape[pn])
			default:
				p.t.warn(fp.Pos, "animation %s: %s is not animated: rotate, scale, opacity, color, border-color, text-color or pos", name, pn)
			}
		}
		a.Frames = append(a.Frames, f)
	}
	if len(a.Frames) == 0 {
		p.t.warn(pr.Pos, "animation %s: the block has no frames (from { ... } to { ... })", name)
		return nil
	}
	sort.SliceStable(a.Frames, func(i, j int) bool { return a.Frames[i].Offset < a.Frames[j].Offset })
	return a
}

// animationUse reads "animation <name> <duration> [easing] [repeat]" and plays the frames of the
// keyframes block of that name.
func (p *prep) animationUse(pr *Prop, defs map[string]*Animation) *Animation {
	name, _ := pr.text(0)
	if name == "" {
		p.t.warn(pr.Pos, "animation takes a name and a duration: animation spin 1s linear infinite")
		return nil
	}
	def := defs[name]
	if def == nil {
		p.t.warn(pr.Pos, "animation %s: no keyframes block with that name in the element", name)
		return nil
	}
	if len(pr.Args) < 2 {
		p.t.warn(pr.Pos, "animation %s: the duration is missing, as in animation %s 1s linear infinite", name, name)
		return nil
	}
	word, _ := pr.text(1)
	dur, ok := parseDuration(word)
	switch {
	case !ok:
		p.t.warn(pr.Pos, "animation %s: %q is not a duration, as in 300ms or 1s", name, word)
		return nil
	case dur <= 0:
		p.t.warn(pr.Pos, "animation %s: the duration must be above zero", name)
		return nil
	}
	a := *def
	a.Duration, a.Easing = dur, "ease"
	for i := 2; i < len(pr.Args); i++ {
		w, _ := pr.text(i)
		switch {
		case easings[w]:
			a.Easing = w
		case w == "infinite":
			a.Repeat = -1
		default:
			n, err := strconv.Atoi(w)
			if err != nil || n < 0 {
				p.t.warn(pr.Pos, "animation %s: %q is not an easing (linear, ease, ease-in, ease-out, ease-in-out) or a repeat count", name, w)
				continue
			}
			a.Repeat = n
		}
	}
	return &a
}

// transition reads "transition <properties> <duration> [easing]" (spec 13). The format is checked only: a
// transition plays when a style changes, and a scene has no such state yet - a rebuild draws the new
// value at once.
func (p *prep) transition(pr *Prop) {
	words := make([]string, len(pr.Args))
	for i := range words {
		words[i], _ = pr.text(i)
	}
	if n := len(words); n > 0 && easings[words[n-1]] {
		words = words[:n-1]
	}
	if len(words) < 2 {
		p.t.warn(pr.Pos, "transition takes properties, a duration and an easing: transition color 200ms ease")
		return
	}
	word := words[len(words)-1]
	dur, ok := parseDuration(word)
	if !ok {
		p.t.warn(pr.Pos, "transition: %q is not a duration (200ms or 1s) or an easing (linear, ease, ease-in, ease-out, ease-in-out)", word)
		return
	}
	if dur <= 0 {
		p.t.warn(pr.Pos, "transition: the duration must be above zero")
	}
	for _, name := range strings.FieldsFunc(strings.Join(words[:len(words)-1], " "), func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	}) {
		switch {
		case animatedProps[name]:
		case layoutProps[name]:
			p.t.warn(pr.Pos, "transition: %s is a layout property and is not animated", name)
		default:
			p.t.warn(pr.Pos, "transition: %s is not animated: rotate, scale, opacity, color, border-color, text-color or pos", name)
		}
	}
}
