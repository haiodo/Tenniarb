// tennicons is the tool that keeps the icons of the lucide set in the repository: a library of .tenn
// items, one per icon, that a document mounts and takes glyphs from by name.
//
//	tennicons [-icons DIR] [-format tenn|go] [-o FILE] [-pkg NAME]
//
// The input is the icons folder of the npm package lucide-static: one SVG per icon name. Get it with
//
//	export npm_config_cache=/tmp/npmcache && cd /tmp && npm pack lucide-static --pack-destination /tmp
//	tar xzf /tmp/lucide-static-<version>.tgz -C /tmp
//
// The default folder is $LUCIDE_ICONS, else /tmp/lucide-static/package/icons. The version written into
// the header is read from the package.json beside the icons folder.
//
// The formats are:
//
//	tenn  libraries/icons.tenn: an item per icon with its path, view box and stroke, the form of
//	      web/fixtures/scene/lib/icon.tenn. This is what the repository keeps and what a design reads.
//	go    a Go table of the same paths, for a caller that wants the glyphs as data in the binary
//	      (penimport did, before the library existed).
//
// Updating the icons is `make icons` from the root of the repository: it fetches the package, writes the
// library and names the version it wrote. The output of the tenn format is committed; the Go table is not,
// a table of thousands of strings the compiler has to chew on belongs in a data file.
//
// Every shape of an icon becomes part of one path: path keeps its d, and circle, ellipse, rect, line,
// polyline and polygon are turned into the same commands. Every icon is parsed with gfx.ParseSVGPath
// before it is written, so a glyph that go/scene cannot draw fails the generation instead of the import
// of a design. The icons are licensed under ISC, and the ones derived from Feather under MIT: the notice
// is in libraries/LICENSES/lucide.txt, and the header of the library points at it.
package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/haiodo/gowt/gfx"
)

func main() {
	iconsDir := flag.String("icons", defaultIcons(), "folder of the SVG files of lucide-static")
	out := flag.String("o", "icons_gen.go", "file to write")
	pkg := flag.String("pkg", "lucide", "package of the generated Go file")
	format := flag.String("format", "go", "what to write: go (a table of the app) or tenn (a library of icons)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tennicons [-icons DIR] [-o FILE] [-pkg NAME]")
		flag.PrintDefaults()
	}
	flag.Parse()
	table, err := readIcons(*iconsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tennicons: %v\n", err)
		os.Exit(1)
	}
	version := readVersion(*iconsDir)
	src, err := render(*format, *pkg, version, table)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tennicons: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "tennicons: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "tennicons: %d icons of lucide-static %s, %d bytes\n", len(table.names), version, len(src))
}

func defaultIcons() string {
	if dir := os.Getenv("LUCIDE_ICONS"); dir != "" {
		return dir
	}
	return "/tmp/lucide-static/package/icons"
}

// table is what the generated file holds.
type table struct {
	names   []string // icon names, sorted
	paths   map[string]string
	solid   map[string]bool // icons that fill a shape with the color instead of stroking it
	viewBox [4]float64      // the view box every icon is drawn in
	stroke  float64         // the stroke width of every icon, in the units of the view box
}

// readVersion reads the version of the npm package the icons belong to; the folder is <pkg>/icons.
func readVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(dir), "package.json"))
	if err != nil {
		return "unknown"
	}
	var p struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &p); err != nil || p.Version == "" {
		return "unknown"
	}
	return p.Version
}

// svg is an SVG element: the root, a group or a shape. lucide writes flat files, but a group is read too.
type svg struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Kids    []svg      `xml:",any"`
}

func (e *svg) attr(name string) string {
	for _, a := range e.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// num reads a numeric attribute; missing means 0.
func (e *svg) num(name string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(e.attr(name)), 64)
	if err != nil {
		return 0
	}
	return v
}

// has reports whether the attribute is written and is not the SVG default.
func (e *svg) has(name string) bool {
	return strings.TrimSpace(e.attr(name)) != ""
}

// readIcons reads every SVG of dir and turns it into one path.
func readIcons(dir string) (*table, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w (see the comment at the top for the npm command)", dir, err)
	}
	t := &table{paths: map[string]string{}, solid: map[string]bool{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".svg")
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		vb, sw, err := iconFrame(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if len(t.names) == 0 {
			t.viewBox, t.stroke = vb, sw
		} else if t.viewBox != vb || t.stroke != sw {
			// the table holds one view box and one stroke width for every icon; a set that varies them
			// needs a table that keeps them per icon.
			return nil, fmt.Errorf("%s: view box %v and stroke %g, the set has %v and %g", e.Name(), vb, sw, t.viewBox, t.stroke)
		}
		parts, solid, err := iconParts(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if len(parts) == 0 {
			return nil, fmt.Errorf("%s: no shapes", e.Name())
		}
		d, err := joinParts(parts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		t.names = append(t.names, name)
		t.paths[name] = d
		if solid {
			t.solid[name] = true
		}
	}
	if len(t.names) == 0 {
		return nil, fmt.Errorf("%s: no .svg files", dir)
	}
	sort.Strings(t.names)
	return t, nil
}

// iconFrame is the view box of the icon and the stroke width of its root, the two values every icon of
// the set shares.
func iconFrame(b []byte) (vb [4]float64, stroke float64, err error) {
	var root svg
	if err := xml.Unmarshal(b, &root); err != nil {
		return vb, 0, fmt.Errorf("not an SVG: %w", err)
	}
	f := floats(root.attr("viewBox"))
	if len(f) != 4 {
		return vb, 0, fmt.Errorf("view box %q, want four numbers", root.attr("viewBox"))
	}
	copy(vb[:], f)
	stroke, err = strconv.ParseFloat(strings.TrimSpace(root.attr("stroke-width")), 64)
	if err != nil || stroke <= 0 {
		return vb, 0, fmt.Errorf("stroke width %q", root.attr("stroke-width"))
	}
	return vb, stroke, nil
}

// iconParts is every shape of one icon as path commands, in the order the file writes them, and
// whether one of them is filled instead of stroked.
func iconParts(b []byte) (parts []string, solid bool, err error) {
	var root svg
	if err := xml.Unmarshal(b, &root); err != nil {
		return nil, false, fmt.Errorf("not an SVG: %w", err)
	}
	var walk func(e *svg)
	walk = func(e *svg) {
		switch e.XMLName.Local {
		case "g", "svg":
			for i := range e.Kids {
				walk(&e.Kids[i])
			}
			return
		case "title", "desc", "defs", "style", "metadata":
			return
		}
		if f := e.attr("fill"); f != "" && f != "none" && f != "transparent" {
			solid = true
		}
		if s := shapePath(e); s != "" {
			parts = append(parts, s)
		}
	}
	walk(&root)
	return parts, solid, nil
}

// joinParts puts the shapes of an icon into one path. A shape written in its own <path> starts at the
// origin, while the commands of one path continue from where the one before left off, so a shape that
// begins with a relative moveto has that moveto made absolute; the pairs after it stay relative. The
// result is checked against the shapes: every point of the joined path must be a point of a shape.
func joinParts(parts []string) (string, error) {
	abs := make([]string, len(parts))
	for i, p := range parts {
		a, err := absoluteStart(strings.TrimSpace(p))
		if err != nil {
			return "", err
		}
		abs[i] = a
	}
	d := strings.Join(abs, " ")
	g, err := gfx.ParseSVGPath(d)
	if err != nil {
		return "", fmt.Errorf("gfx cannot draw the path: %w", err)
	}
	want, err := shapePoints(parts)
	if err != nil {
		return "", err
	}
	if len(g.Pts) != len(want) {
		return "", fmt.Errorf("the joined path has %d points, the shapes %d", len(g.Pts), len(want))
	}
	for i := range want {
		if math.Abs(g.Pts[i].X-want[i].X) > 1e-6 || math.Abs(g.Pts[i].Y-want[i].Y) > 1e-6 {
			return "", fmt.Errorf("point %d of the joined path is %v, the shapes have %v", i, g.Pts[i], want[i])
		}
	}
	return d, nil
}

// absoluteStart makes the first command of a shape absolute, so that it does not depend on the shape
// before it. m x y moves to (x, y), and the pairs after it are relative linetos: an absolute M would
// read them as absolute points, so they are put behind an l.
func absoluteStart(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	c := p[0]
	switch {
	case c >= 'A' && c <= 'Z':
		return p, nil
	case c != 'm':
		return "", fmt.Errorf("a shape starts with %q, a relative command that is not a moveto", c)
	}
	i := 1
	skip := func() {
		for i < len(p) && (p[i] == ' ' || p[i] == ',') {
			i++
		}
	}
	readNum := func() (string, bool) {
		start := i
		if i < len(p) && (p[i] == '-' || p[i] == '+') {
			i++
		}
		dot := false
		for i < len(p) {
			switch ch := p[i]; {
			case ch >= '0' && ch <= '9':
				i++
			case ch == '.' && !dot:
				dot = true
				i++
			case (ch == 'e' || ch == 'E') && i > start:
				i++
				if i < len(p) && (p[i] == '-' || p[i] == '+') {
					i++
				}
			default:
				return p[start:i], i > start
			}
		}
		return p[start:i], i > start
	}
	skip()
	x, ok1 := readNum()
	skip()
	y, ok2 := readNum()
	if !ok1 || !ok2 {
		return "", fmt.Errorf("a moveto without two numbers: %q", p)
	}
	out := "M" + x + " " + y
	j := i
	for j < len(p) && (p[j] == ' ' || p[j] == ',') {
		j++
	}
	if j < len(p) && (p[j] == '-' || p[j] == '+' || p[j] == '.' || p[j] >= '0' && p[j] <= '9') {
		out += " l"
	}
	rest := p[i:]
	if rest != "" && rest[0] != ' ' && rest[0] != ',' && rest[0] != 'l' {
		out += " "
	}
	return out + rest, nil
}

// shapePoints parses every shape on its own, as the SVG draws it, and lists the points in order.
func shapePoints(parts []string) ([]gfx.Point, error) {
	var out []gfx.Point
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		g, err := gfx.ParseSVGPath(p)
		if err != nil {
			return nil, fmt.Errorf("a shape does not parse: %w", err)
		}
		out = append(out, g.Pts...)
	}
	return out, nil
}

// shapePath is one SVG shape as path commands; "" for an element that draws nothing.
func shapePath(e *svg) string {
	switch e.XMLName.Local {
	case "path":
		return strings.TrimSpace(e.attr("d"))
	case "line":
		return "M" + pair(e.num("x1"), e.num("y1")) + " L" + pair(e.num("x2"), e.num("y2"))
	case "circle":
		cx, cy, r := e.num("cx"), e.num("cy"), e.num("r")
		return ellipsePath(cx, cy, r, r)
	case "ellipse":
		return ellipsePath(e.num("cx"), e.num("cy"), e.num("rx"), e.num("ry"))
	case "rect":
		return rectPath(e.num("x"), e.num("y"), e.num("width"), e.num("height"), e.num("rx"), e.num("ry"))
	case "polyline", "polygon":
		pts := points(e.attr("points"))
		if len(pts) < 2 {
			return ""
		}
		var sb strings.Builder
		sb.WriteString("M" + pair(pts[0][0], pts[0][1]))
		for _, p := range pts[1:] {
			sb.WriteString(" L" + pair(p[0], p[1]))
		}
		if e.XMLName.Local == "polygon" {
			sb.WriteString(" Z")
		}
		return sb.String()
	}
	return ""
}

// ellipsePath is a full ellipse of two arcs, which is also how a circle is drawn.
func ellipsePath(cx, cy, rx, ry float64) string {
	if rx <= 0 || ry <= 0 {
		return ""
	}
	x0, x1 := cx-rx, cx+rx
	arc := "A" + pair(rx, ry) + " 0 1 0 "
	return "M" + pair(x0, cy) + " " + arc + pair(x1, cy) + " " + arc + pair(x0, cy) + " Z"
}

// rectPath is a rectangle, with rounded corners when rx or ry is written.
func rectPath(x, y, w, h, rx, ry float64) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	if ry == 0 {
		ry = rx
	}
	rx = math.Min(math.Min(rx, w/2), h/2)
	ry = math.Min(math.Min(ry, w/2), h/2)
	if rx <= 0 || ry <= 0 {
		return "M" + pair(x, y) + " L" + pair(x+w, y) + " L" + pair(x+w, y+h) + " L" + pair(x, y+h) + " Z"
	}
	arc := "A" + pair(rx, ry) + " 0 0 1 "
	var sb strings.Builder
	sb.WriteString("M" + pair(x+rx, y))
	sb.WriteString(" L" + pair(x+w-rx, y))
	sb.WriteString(" " + arc + pair(x+w, y+ry))
	sb.WriteString(" L" + pair(x+w, y+h-ry))
	sb.WriteString(" " + arc + pair(x+w-rx, y+h))
	sb.WriteString(" L" + pair(x+rx, y+h))
	sb.WriteString(" " + arc + pair(x, y+h-ry))
	sb.WriteString(" L" + pair(x, y+ry))
	sb.WriteString(" " + arc + pair(x+rx, y))
	sb.WriteString(" Z")
	return sb.String()
}

// pair writes two numbers of a path; lucide has at most three decimals, and the sums of the rounded
// values are rounded too, so the text stays short.
func pair(x, y float64) string { return num(x) + " " + num(y) }

func num(v float64) string {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// floats splits a list of SVG numbers: "0 0 24 24", "1,2 3,4" or "1 2 3 4".
func floats(s string) []float64 {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// points splits the points attribute of a polyline or polygon into pairs.
func points(s string) [][2]float64 {
	f := floats(s)
	out := make([][2]float64, 0, len(f)/2)
	for i := 0; i+1 < len(f); i += 2 {
		out = append(out, [2]float64{f[i], f[i+1]})
	}
	return out
}

// render writes the Go file of the table, formatted by go/format.
// render is the file the generator writes: the table of the app as Go, or the library of the icons as
// .tenn, which a document mounts the way it mounts any other library.
func render(format, pkg, version string, t *table) ([]byte, error) {
	if format == "tenn" {
		return renderTenn(version, t)
	}
	return renderGo(pkg, version, t)
}

// renderTenn writes the icons as a library of items: one item per icon, with the path, its view box and
// the stroke of the set. The form is the one of web/fixtures/scene/lib/icon.tenn, so a document mounts it
// with --from-file and takes an icon by name.
func renderTenn(version string, t *table) ([]byte, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "// The icons of lucide-static %s, one item per icon.\n", version)
	sb.WriteString("//\n")
	sb.WriteString("// Generated by cmd/tennicons from the SVG files of the package: regenerate with\n")
	sb.WriteString("// go run ./cmd/tennicons -format tenn -icons <lucide-static>/icons -o ../../libraries/icons.tenn\n")
	sb.WriteString("//\n")
	sb.WriteString("// The icons are licensed under the ISC license, and the ones derived from Feather under the MIT\n")
	sb.WriteString("// license: see ../LICENSES/lucide.txt.\n\n")
	for _, name := range t.names {
		fmt.Fprintf(&sb, "item %q {\n", name)
		fmt.Fprintf(&sb, "\twidth %s; height %s; path %q\n", num(t.viewBox[2]), num(t.viewBox[3]), t.paths[name])
		fmt.Fprintf(&sb, "\tview-box %s %s %s %s; border-color #000000; line-width %s\n",
			num(t.viewBox[0]), num(t.viewBox[1]), num(t.viewBox[2]), num(t.viewBox[3]), num(t.stroke))
		sb.WriteString("}\n")
	}
	return []byte(sb.String()), nil
}

func renderGo(pkg, version string, t *table) ([]byte, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "// Code generated by cmd/tennicons from lucide-static %s; DO NOT EDIT.\n", version)
	sb.WriteString("//\n")
	sb.WriteString("// The paths are the shapes of the SVG files of the package, each icon in one path. Regenerate with\n")
	sb.WriteString("// go generate ./lucide, which needs the package unpacked where cmd/tennicons looks for it.\n")
	sb.WriteString("//\n")
	sb.WriteString("// The icons are licensed under the ISC license, and the ones derived from Feather under the MIT\n")
	sb.WriteString("// license: see LICENSES/lucide.txt beside this file, which the generated table repeats nowhere.\n\n")
	fmt.Fprintf(&sb, "package %s\n\n", pkg)
	sb.WriteString("// version is the version of lucide-static the table was made from.\n")
	fmt.Fprintf(&sb, "const version = %q\n\n", version)
	sb.WriteString("// The view box every icon of the set is drawn in, and the stroke width the library draws it with.\n")
	fmt.Fprintf(&sb, "const (\n\tviewBoxX, viewBoxY = %s, %s\n\tviewBoxW, viewBoxH = %s, %s\n\tstroke            = %s\n)\n\n",
		num(t.viewBox[0]), num(t.viewBox[1]), num(t.viewBox[2]), num(t.viewBox[3]), num(t.stroke))
	sb.WriteString("// paths maps the name of an icon to the d of its path: every shape of the icon in one path.\n")
	sb.WriteString("var paths = map[string]string{\n")
	for _, name := range t.names {
		fmt.Fprintf(&sb, "\t%q: `%s`,\n", name, t.paths[name])
	}
	sb.WriteString("}\n\n")
	sb.WriteString("// solid lists the icons with a shape filled with the color instead of stroked, a dot of\n")
	sb.WriteString("// `tag` or `vault`: go/scene draws one path either filled or stroked, so such a dot comes out as\n")
	sb.WriteString("// an outline (the converter counts them).\n")
	sb.WriteString("var solid = map[string]bool{\n")
	for _, name := range t.names {
		if t.solid[name] {
			fmt.Fprintf(&sb, "\t%q: true,\n", name)
		}
	}
	sb.WriteString("}\n")
	return format.Source([]byte(sb.String()))
}
