package penimport

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// pctx tells props how to read a pen map. An override map (explicit) writes only the keys it has,
// including values equal to the default, because they may restore what the component changed.
type pctx struct {
	mode     string
	explicit bool
	// eff is the pen map with the overrides applied, for values derived from several keys.
	eff map[string]any
}

func fnum(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func numArg(v float64) Arg { return Arg{S: fnum(v)} }
func wordArg(s string) Arg { return Arg{S: s} }

// strArg quotes a string. go/tenn's printer does not escape a backslash and the lexer reads one before
// a quote as an escape, so such a backslash is dropped.
func strArg(s string) Arg {
	s = strings.ReplaceAll(s, `\"`, `"`)
	return Arg{S: strings.TrimSuffix(s, `\`), Quoted: true}
}
func prop(name string, a ...Arg) Prop { return Prop{Name: name, Args: a} }

// normalize is convertNameJS of web/packages/core/src/execution.ts, as in go/scene.
func normalize(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func parseHex(s string) (string, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 || len(s) == 4 {
		var t strings.Builder
		for _, r := range s {
			t.WriteRune(r)
			t.WriteRune(r)
		}
		s = t.String()
	}
	if len(s) != 6 && len(s) != 8 {
		return "", false
	}
	if _, err := strconv.ParseUint(s, 16, 32); err != nil {
		return "", false
	}
	return "#" + strings.ToLower(s), true
}

// color resolves a hex literal or a $variable to #rrggbb[aa] for the mode.
func (c *conv) color(f *File, s, mode string) (string, bool) {
	if !strings.HasPrefix(s, "$") {
		return parseHex(s)
	}
	name := s[1:]
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}
	v := c.variable(f, name)
	if v == nil {
		f.unsup("unknown variable")
		return "", false
	}
	f.Variables[name]++
	switch val := v["value"].(type) {
	case string:
		return c.color(f, val, mode)
	case []any:
		pick := ""
		for _, it := range val {
			im, _ := it.(map[string]any)
			th, _ := im["theme"].(map[string]any)
			cs, _ := im["value"].(string)
			if pick == "" {
				pick = cs
			}
			for _, tv := range th {
				if tv == mode {
					pick = cs
				}
			}
		}
		return c.color(f, pick, mode)
	}
	return "", false
}

// variable looks the name up in the file and in its libraries.
func (c *conv) variable(f *File, name string) map[string]any {
	if v := f.vars[name]; v != nil {
		return v
	}
	for _, g := range f.Imports {
		if v := g.vars[name]; v != nil {
			return v
		}
	}
	return nil
}

// paint returns the color of a fill or stroke value; gradients give their first stop.
func (c *conv) paint(f *File, v any, mode string) (string, bool) {
	switch x := v.(type) {
	case string:
		return c.color(f, x, mode)
	case []any:
		if len(x) > 1 {
			f.unsup("several fills (first used)")
		}
		if len(x) > 0 {
			return c.paint(f, x[0], mode)
		}
	case map[string]any:
		if en, ok := x["enabled"].(bool); ok && !en {
			return "", false
		}
		if x["type"] != "gradient" {
			f.unsup("fill " + toStr(x["type"]))
			return "", false
		}
		f.unsup("gradient fill (first stop)")
		cols, _ := x["colors"].([]any)
		for _, ci := range cols {
			cm, _ := ci.(map[string]any)
			cs, _ := cm["color"].(string)
			if col, ok := c.color(f, cs, mode); ok {
				return col, true
			}
		}
	}
	return "", false
}

func toStr(v any) string {
	s, _ := v.(string)
	return s
}

func sizeProp(f *File, name string, v any, explicit bool) []Prop {
	switch x := v.(type) {
	case float64:
		return []Prop{prop(name, numArg(x))}
	case string:
		word := "hug"
		if strings.HasPrefix(x, "fill_container") {
			word = "fill"
		}
		if i := strings.IndexByte(x, '('); i >= 0 && x[i:] != "(0)" {
			f.unsup("size fallback in " + strings.TrimSuffix(x[:i], "("))
		}
		if word == "hug" && !explicit {
			return nil
		}
		return []Prop{prop(name, wordArg(word))}
	}
	return nil
}

func padArgs(v any) []Arg {
	switch p := v.(type) {
	case float64:
		return []Arg{numArg(p)}
	case []any:
		out := make([]Arg, len(p))
		for i, e := range p {
			n, _ := e.(float64)
			out[i] = numArg(n)
		}
		return out
	}
	return nil
}

func alignWord(f *File, key string, v any) (string, bool) {
	switch v {
	case "center", "end":
		return v.(string), true
	case "space_between":
		return "space-between", true
	case "space_around":
		f.unsup(key + " space_around")
	}
	return "", false
}

func (c *conv) layoutProps(f *File, m map[string]any, x pctx) []Prop {
	var ps []Prop
	lay, has := m["layout"]
	if has || !x.explicit {
		dir := "row"
		switch lay {
		case "vertical":
			dir = "column"
		case "none":
			dir = "abs"
		}
		ps = append(ps, prop("auto-layout", wordArg(dir)))
	}
	if g, ok := m["gap"].(float64); ok && (g != 0 || x.explicit) {
		ps = append(ps, prop("gap", numArg(g)))
	}
	if a := padArgs(m["padding"]); a != nil {
		ps = append(ps, prop("padding", a...))
	}
	for _, k := range [...][2]string{{"alignItems", "align"}, {"justifyContent", "justify"}} {
		v, ok := m[k[0]]
		if !ok {
			continue
		}
		if w, ok := alignWord(f, k[0], v); ok {
			ps = append(ps, prop(k[1], wordArg(w)))
		} else if x.explicit && v == "start" {
			ps = append(ps, prop(k[1], wordArg("start")))
		}
	}
	return ps
}

func radiusProp(f *File, v any) []Prop {
	switch r := v.(type) {
	case float64:
		if r > 0 {
			return []Prop{prop("corner-radius", numArg(r))}
		}
	case []any:
		args := make([]Arg, len(r))
		same := true
		for i, e := range r {
			n, _ := e.(float64)
			args[i] = numArg(n)
			same = same && n == args[0].num()
		}
		if same {
			args = args[:1]
		}
		if args[0].num() > 0 || len(args) > 1 {
			return []Prop{prop("corner-radius", args...)}
		}
	}
	return nil
}

func (a Arg) num() float64 {
	v, _ := strconv.ParseFloat(a.S, 64)
	return v
}

// opacityProp writes a pen opacity; an override writes 1 too, it may restore what the component dimmed.
func opacityProp(f *File, v any, explicit bool) []Prop {
	n, ok := v.(float64)
	switch {
	case !ok || n < 0 || n > 1:
		f.unsup("opacity")
	case n < 1 || explicit:
		return []Prop{prop("opacity", numArg(n))}
	}
	return nil
}

func (c *conv) shadowProp(f *File, v any, mode string) []Prop {
	var m map[string]any
	switch x := v.(type) {
	case map[string]any:
		m = x
	case []any:
		for _, it := range x {
			if im, ok := it.(map[string]any); ok {
				if m == nil && im["type"] == "shadow" && im["shadowType"] != "inner" {
					m = im
				} else {
					f.unsup("effect " + toStr(im["type"]) + " " + toStr(im["shadowType"]))
				}
			}
		}
	}
	if m == nil {
		return nil
	}
	if m["type"] != "shadow" || m["shadowType"] == "inner" {
		f.unsup("effect " + toStr(m["type"]) + " " + toStr(m["shadowType"]))
		return nil
	}
	var dx, dy float64
	if off, ok := m["offset"].(map[string]any); ok {
		dx, _ = off["x"].(float64)
		dy, _ = off["y"].(float64)
	}
	blur, _ := m["blur"].(float64)
	col := "#00000040"
	if cs, ok := m["color"].(string); ok {
		if cc, ok := c.color(f, cs, mode); ok {
			col = cc
		}
	}
	return []Prop{prop("shadow", numArg(dx), numArg(dy), numArg(blur), wordArg(col))}
}

// lookProps are the fill and the border of an item. go/scene reads display only on a link, so none is written.
func (c *conv) lookProps(f *File, m map[string]any, typ string, x pctx) []Prop {
	fill, hasFill := c.paint(f, m["fill"], x.mode)
	stroke, hasStroke := c.paint(f, m["stroke"], x.mode)
	sw, _ := m["strokeWidth"].(float64)
	if _, perSide := m["strokeWidth"].(map[string]any); perSide {
		f.unsup("per-side stroke width")
	}
	if hasStroke && sw == 0 {
		sw = 1
	}
	if hasStroke && m["strokeAlignment"] != "inner" {
		f.unsup("stroke alignment other than inner")
	}
	if x.explicit {
		return explicitLook(typ, m, x.eff, fill, stroke, sw, hasFill, hasStroke)
	}
	switch typ {
	case "text":
		if hasFill {
			return []Prop{prop("text-color", wordArg(fill))}
		}
		return nil
	case "path":
		return pathLook(f, m, fill, stroke, sw, hasFill, hasStroke)
	}
	var ps []Prop
	if typ == "ellipse" {
		// go/scene has no ellipse; a radius of half the width is what gfx/pen does too
		if w, ok := m["width"].(float64); ok {
			ps = append(ps, prop("corner-radius", numArg(w/2)))
		} else {
			f.unsup("ellipse of flexible width")
		}
	}
	switch {
	case hasFill:
		ps = append(ps, prop("color", wordArg(fill)))
	case typ == "ellipse":
		ps = append(ps, prop("color", wordArg("#00000000")))
	}
	if hasStroke {
		ps = append(ps, prop("border-color", wordArg(stroke)), prop("line-width", numArg(sw)))
	} else if hasFill || typ == "ellipse" {
		ps = append(ps, prop("line-width", numArg(0)))
	}
	return ps
}

// viewBoxProp maps the pen viewBox onto the box. Without one pen draws the path unscaled, which is a
// view box of the size of the node.
func viewBoxProp(m map[string]any) []Prop {
	if vb, ok := m["viewBox"].([]any); ok && len(vb) == 4 {
		args := make([]Arg, 4)
		for i, v := range vb {
			n, _ := v.(float64)
			args[i] = numArg(n)
		}
		return []Prop{prop("view-box", args...)}
	}
	w, wok := m["width"].(float64)
	h, hok := m["height"].(float64)
	if wok && hok {
		return []Prop{prop("view-box", numArg(0), numArg(0), numArg(w), numArg(h))}
	}
	return nil
}

// pathLook is the look of a pen path as a vector icon; nil when pen draws nothing.
func pathLook(f *File, m map[string]any, fill, stroke string, sw float64, hasFill, hasStroke bool) []Prop {
	g, _ := m["geometry"].(string)
	if g == "" || !hasFill && !hasStroke {
		return nil
	}
	ps := append([]Prop{prop("path", strArg(g))}, viewBoxProp(m)...)
	if m["fillRule"] == "evenodd" {
		ps = append(ps, prop("fill-rule", wordArg("evenodd")))
	}
	if hasFill {
		ps = append(ps, prop("color", wordArg(fill)))
	}
	if hasStroke {
		ps = append(ps, prop("border-color", wordArg(stroke)), prop("line-width", numArg(sw)))
	}
	return ps
}

func explicitLook(typ string, m, eff map[string]any, fill, stroke string, sw float64, hasFill, hasStroke bool) []Prop {
	var ps []Prop
	if g, ok := m["geometry"].(string); ok && typ == "path" {
		ps = append(ps, prop("path", strArg(g)))
		if _, has := m["viewBox"]; has || eff["viewBox"] == nil {
			ps = append(ps, viewBoxProp(mergeMaps(eff, m))...)
		}
	} else if _, has := m["viewBox"]; has && typ == "path" {
		ps = append(ps, viewBoxProp(m)...)
	}
	if typ == "path" && m["fillRule"] == "evenodd" {
		ps = append(ps, prop("fill-rule", wordArg("evenodd")))
	}
	if _, ok := m["fill"]; ok && hasFill {
		name := "color"
		if typ == "text" {
			name = "text-color"
		}
		ps = append(ps, prop(name, wordArg(fill)))
	}
	if _, ok := m["stroke"]; ok && hasStroke {
		ps = append(ps, prop("border-color", wordArg(stroke)))
	}
	if _, ok := m["strokeWidth"]; ok || hasStroke {
		ps = append(ps, prop("line-width", numArg(sw)))
	}
	return ps
}

var ignoredKeys = [...]string{"rotation", "flipX", "flipY", "strokeLinecap", "strokeLinejoin", "metadata",
	"layoutIncludeStroke"}

func countIgnored(f *File, m map[string]any) {
	for _, k := range ignoredKeys {
		v, ok := m[k]
		if !ok {
			continue
		}
		if n, isNum := v.(float64); isNum && n == 0 {
			continue
		}
		if b, isBool := v.(bool); isBool && !b {
			continue
		}
		f.unsup(k)
	}
}

// fontProps maps the font keys present in m; overrides carry only the keys they change, so
// defaults are written only there, to reset what the component sets.
func fontProps(m map[string]any, explicit bool) []Prop {
	var ps []Prop
	if s, ok := m["fontFamily"].(string); ok && s != "" {
		ps = append(ps, prop("font-family", strArg(s)))
	}
	switch w := m["fontWeight"].(type) {
	case string:
		if explicit || w != "normal" && w != "400" {
			ps = append(ps, prop("font-weight", wordArg(w)))
		}
	case float64:
		if explicit || w != 400 {
			ps = append(ps, prop("font-weight", numArg(w)))
		}
	}
	if s, ok := m["fontStyle"].(string); ok && s != "" && (explicit || s != "normal") {
		ps = append(ps, prop("font-style", wordArg(s)))
	}
	if n, ok := m["letterSpacing"].(float64); ok && (explicit || n != 0) {
		ps = append(ps, prop("letter-spacing", numArg(n)))
	}
	return ps
}

func (c *conv) textProps(f *File, m map[string]any, x pctx) []Prop {
	var ps []Prop
	if s, ok := m["content"].(string); ok {
		ps = append(ps, prop("title", strArg(s)))
	} else if m["content"] != nil {
		f.unsup("text content other than a string")
	}
	_, hasSize := m["fontSize"]
	_, hasLH := m["lineHeight"]
	if hasSize || hasLH || !x.explicit {
		eff := m
		if x.eff != nil {
			eff = x.eff
		}
		size, _ := eff["fontSize"].(float64)
		if size == 0 {
			size = 14
		}
		if size != 18 || x.explicit {
			ps = append(ps, prop("font-size", numArg(size)))
		}
		// pen.dev rounds the line box to whole units.
		if lh, _ := eff["lineHeight"].(float64); lh > 0 {
			ps = append(ps, prop("line-height", numArg(math.Round(lh*size))))
		}
	}
	ps = append(ps, fontProps(m, x.explicit)...)
	// A leaf is centered in go/scene, left in pen.dev.
	switch m["textAlign"] {
	case "center":
		ps = append(ps, prop("layout", wordArg("middle")))
	case "right":
		ps = append(ps, prop("layout", wordArg("right")))
	default:
		if m["textAlign"] != nil || !x.explicit {
			ps = append(ps, prop("layout", wordArg("left")))
		}
	}
	return ps
}

// props converts the keys of a pen map of the given type; position is up to the caller.
func (c *conv) props(f *File, m map[string]any, typ string, x pctx) []Prop {
	countIgnored(f, m)
	var ps []Prop
	if typ == "frame" {
		ps = append(ps, c.layoutProps(f, m, x)...)
	}
	if typ == "text" {
		ps = append(ps, c.textProps(f, m, x)...)
		if !x.explicit {
			ps = append(ps, prop("auto-layout", wordArg("row")))
		}
	}
	for _, k := range [...]string{"width", "height"} {
		if v, ok := m[k]; ok {
			ps = append(ps, sizeProp(f, k, v, x.explicit)...)
		}
	}
	if b, ok := m["clip"].(bool); ok && (b || x.explicit) {
		ps = append(ps, prop("clip", wordArg(map[bool]string{true: "yes", false: "no"}[b])))
	}
	ps = append(ps, c.lookProps(f, m, typ, x)...)
	if o, ok := m["opacity"]; ok {
		ps = append(ps, opacityProp(f, o, x.explicit)...)
	}
	if r, ok := m["cornerRadius"]; ok {
		ps = append(ps, radiusProp(f, r)...)
	}
	if e, ok := m["effect"]; ok {
		ps = append(ps, c.shadowProp(f, e, x.mode)...)
	}
	return ps
}
