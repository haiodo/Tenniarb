package penimport

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/pen"
	"github.com/haiodo/tenniarb/go/scene"
)

// ScreenReport compares the boxes of one top level node as gfx/pen and go/scene lay them out.
type ScreenReport struct {
	Name     string
	Compared int
	Within   int // boxes where x, y, width and height all differ by at most 0.5
	MaxDelta float64
	MaxAt    string
	// NameDiffs counts paired boxes whose names do not agree, which would mean a wrong pairing.
	NameDiffs int
	// PenOnly and SceneOnly count boxes without a partner in the other tree.
	PenOnly, SceneOnly int
	// Notes describe the first nodes whose children differ in number.
	Notes []string
	// Diffs lists the first boxes that differ, deepest first within a branch.
	Diffs []string
	// Err is set when the screen could not be compared.
	Err string
}

// Report is the result of Compare for one file.
type Report struct {
	File     string
	Screens  []ScreenReport
	Problems []string // errors go/scene found in the converted file
	Warnings int
	// WarningKinds counts the go/scene warnings by the property they name.
	WarningKinds map[string]int
	// Icons is the number of pen icon nodes the reference had to fill in as rectangles: gfx/pen builds no
	// icon, so a design with icons is compared with a copy of itself where every icon takes its room.
	Icons int
}

func (r *Report) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "fidelity %s: %d go/scene warnings, %d errors\n", r.File, r.Warnings, len(r.Problems))
	if r.Icons > 0 {
		fmt.Fprintf(&sb, "  reference: gfx/pen builds no icon, so the %d icons of the file are rectangles of their size in it\n", r.Icons)
	}
	for k, n := range r.WarningKinds {
		fmt.Fprintf(&sb, "  go/scene warning: %s (%d)\n", k, n)
	}
	for _, p := range r.Problems {
		fmt.Fprintf(&sb, "  error: %s\n", p)
	}
	for _, s := range r.Screens {
		if s.Err != "" {
			fmt.Fprintf(&sb, "  %-50q %s\n", s.Name, s.Err)
			continue
		}
		fmt.Fprintf(&sb, "  %-50q boxes %5d  within 0.5: %5d  max delta %8.2f  unpaired pen/scene %d/%d  name diffs %d  worst at %s\n",
			s.Name, s.Compared, s.Within, s.MaxDelta, s.PenOnly, s.SceneOnly, s.NameDiffs, s.MaxAt)
		for _, n := range s.Diffs {
			fmt.Fprintf(&sb, "      diff: %s\n", n)
		}
		for _, n := range s.Notes {
			fmt.Fprintf(&sb, "      note: %s\n", n)
		}
	}
	return sb.String()
}

type bnode struct {
	name, id   string
	x, y, w, h float64
	kids       []*bnode
}

func (b *bnode) size() int {
	n := 1
	for _, k := range b.kids {
		n += k.size()
	}
	return n
}

func penTree(boxes []pen.Box) *bnode {
	var root *bnode
	var stack []*bnode
	for _, b := range boxes {
		n := &bnode{name: b.Name, id: b.ID, x: b.X, y: b.Y, w: b.W, h: b.H}
		if b.Depth == 0 {
			root = n
		} else {
			stack[b.Depth-1].kids = append(stack[b.Depth-1].kids, n)
		}
		stack = append(stack[:b.Depth], n)
	}
	return root
}

// sceneTree takes the boxes of the item at path (a top level key) out of the boxes of the file.
func sceneTree(boxes []scene.Box, path string) *bnode {
	var root *bnode
	var stack []*bnode
	base := 0
	for _, b := range boxes {
		if root == nil {
			if b.Path != path || b.Depth != 1 {
				continue
			}
			base = b.Depth
		} else if b.Depth <= base {
			break
		}
		n := &bnode{name: b.Name, id: b.Path, x: b.X, y: b.Y, w: b.W, h: b.H}
		d := b.Depth - base
		if d == 0 {
			root = n
		} else {
			stack[d-1].kids = append(stack[d-1].kids, n)
		}
		stack = append(stack[:d], n)
	}
	return root
}

func names(l []*bnode) []string {
	out := make([]string, len(l))
	for i, k := range l {
		out[i] = k.name
	}
	return out
}

// sameName allows for the suffixes and the prefix the importer adds to keep paths unambiguous.
func sameName(pen, scene string) bool {
	pen = strings.TrimSpace(strings.ReplaceAll(pen, "\n", " "))
	return pen == "" || strings.HasPrefix(strings.TrimPrefix(scene, "_"), pen)
}

func isPlaceholder(n *bnode) bool { return strings.HasPrefix(n.name, "missing: ") }

func (s *ScreenReport) pair(p, q *bnode, root bool) {
	d := math.Max(math.Abs(p.w-q.w), math.Abs(p.h-q.h))
	if !root {
		d = math.Max(d, math.Max(math.Abs(p.x-q.x), math.Abs(p.y-q.y)))
	}
	s.Compared++
	if !sameName(p.name, q.name) {
		s.NameDiffs++
		if s.NameDiffs == 1 {
			s.Notes = append(s.Notes, fmt.Sprintf("names differ: pen %s %q, scene %s %q", p.id, p.name, q.id, q.name))
		}
	}
	if d <= 0.5 {
		s.Within++
	}
	if d > 0.5 && len(s.Diffs) < 6 {
		s.Diffs = append(s.Diffs, fmt.Sprintf("pen %s %q %gx%g at %g,%g; scene %s %gx%g at %g,%g", p.id, p.name, p.w, p.h, p.x, p.y, q.id, q.w, q.h, q.x, q.y))
	}
	if d > s.MaxDelta {
		s.MaxDelta, s.MaxAt = d, fmt.Sprintf("pen %s = scene %s (pen %gx%g at %g,%g; scene %gx%g at %g,%g)", p.id, q.id, p.w, p.h, p.x, p.y, q.w, q.h, q.x, q.y)
	}
	var qs []*bnode
	for _, k := range q.kids {
		if !isPlaceholder(k) {
			qs = append(qs, k)
		}
	}
	n := min(len(p.kids), len(qs))
	if len(p.kids) != len(qs) && len(s.Notes) < 6 {
		s.Notes = append(s.Notes, fmt.Sprintf("pen %s %q has %d children %v, scene %s has %d %v", p.id, p.name, len(p.kids), names(p.kids), q.id, len(qs), names(qs)))
	}
	for i := 0; i < n; i++ {
		s.pair(p.kids[i], qs[i], false)
	}
	for _, k := range p.kids[n:] {
		s.PenOnly += k.size()
	}
	for _, k := range qs[n:] {
		s.SceneOnly += k.size()
	}
}

// Compare loads the converted file f from dir with go/scene and compares the layout of its screens
// (the top level nodes of at least 400 x 300) with the layout gfx/pen makes of the .pen file. Both use
// the same text measurer, so the fonts do not differ.
func Compare(f *File, dir string, all bool) (*Report, error) {
	return CompareWith(f, dir, all, gfx.EstimateText{})
}

// CompareWith is Compare with the given text measurer, for the metrics of real fonts.
func CompareWith(f *File, dir string, all bool, m gfx.TextMeasurer) (*Report, error) {
	rep := &Report{File: f.Name}
	tree, err := scene.Load(dir, f.Name+".tenn")
	if tree == nil {
		return rep, err
	}
	if errs, ok := err.(scene.Errors); ok {
		for _, e := range errs {
			if !e.Warn {
				rep.Problems = append(rep.Problems, e.Error())
			}
		}
	}
	if len(rep.Problems) > 0 {
		return rep, nil
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "", scene.Options{Measurer: m}) })
	if err != nil {
		rep.Problems = append(rep.Problems, err.Error())
		return rep, nil
	}
	rep.Warnings = len(screen.Warnings)
	rep.WarningKinds = map[string]int{}
	for _, w := range screen.Warnings {
		rep.WarningKinds[strings.TrimSpace(strings.SplitN(w.Msg, ":", 2)[0])]++
	}
	refPath, refDir, icons, err := iconReference(f)
	if err != nil {
		return rep, err
	}
	defer os.RemoveAll(refDir)
	rep.Icons = icons
	d, err := pen.Open(refPath, pen.Options{Measurer: m})
	if err != nil {
		return rep, err
	}
	boxes := screen.Boxes()
	infos := d.Screens()
	if all {
		infos = nil
		for _, t := range f.Tops {
			infos = append(infos, pen.Info{Index: t.Index, Name: t.Node.Name})
		}
	}
	for _, info := range infos {
		rep.Screens = append(rep.Screens, compareScreen(f, d, boxes, info))
	}
	return rep, nil
}

func compareScreen(f *File, d *pen.Doc, boxes []scene.Box, info pen.Info) ScreenReport {
	s := ScreenReport{Name: info.Name}
	var top *Node
	for _, t := range f.Tops {
		if t.Index == info.Index {
			top = t.Node
		}
	}
	if top == nil {
		s.Err = "not converted"
		return s
	}
	ps := gfx.NewScene()
	var psc *pen.Screen
	var err error
	ps.Update(func(tx *gfx.Tx) { psc, err = d.Build(tx, nil, info.Index) })
	if err != nil {
		s.Err = err.Error()
		return s
	}
	p := penTree(psc.Boxes())
	q := sceneTree(boxes, key(top))
	if p == nil || q == nil {
		s.Err = "no boxes"
		return s
	}
	s.pair(p, q, true)
	return s
}

// iconReference writes f and the libraries it imports into a temp dir with every icon node made a
// rectangle of the room the icon takes. gfx/pen builds no icon: its box tree has no box where the design
// has one, so the layout of a screen with icons cannot be compared with it - the first icon in a frame
// already shifts every box after it. A rectangle of the size of the icon takes that room and nothing
// else, so the copy lays out as the design does, and the icons themselves become comparable too.
//
// It returns the path of the copy, the dir to remove, and how many icons were made rectangles.
func iconReference(f *File) (path, dir string, icons int, err error) {
	dir, err = os.MkdirTemp("", "penimport-reference")
	if err != nil {
		return "", "", 0, err
	}
	files := importGraph(f)
	names := make(map[*File]string, len(files))
	for _, g := range files {
		names[g] = filepath.Base(g.Path)
	}
	for _, g := range files {
		root, err := copyJSON(g.raw)
		if err != nil {
			os.RemoveAll(dir)
			return "", "", 0, fmt.Errorf("%s: %w", g.Name, err)
		}
		// The libraries of the copy sit beside it, so their relative paths become plain names.
		if imp, ok := root["imports"].(map[string]any); ok {
			for alias := range imp {
				if lib := g.Imports[alias]; lib != nil {
					imp[alias] = names[lib]
				}
			}
		}
		icons += iconRects(root)
		b, err := json.Marshal(root)
		if err != nil {
			os.RemoveAll(dir)
			return "", "", 0, fmt.Errorf("%s: %w", g.Name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, names[g]), b, 0o644); err != nil {
			os.RemoveAll(dir)
			return "", "", 0, err
		}
	}
	return filepath.Join(dir, names[f]), dir, icons, nil
}

// importGraph is f and every library it imports, directly or through another library.
func importGraph(f *File) []*File {
	var out []*File
	seen := map[*File]bool{}
	var visit func(g *File)
	visit = func(g *File) {
		if seen[g] {
			return
		}
		seen[g] = true
		out = append(out, g)
		aliases := make([]string, 0, len(g.Imports))
		for a := range g.Imports {
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		for _, a := range aliases {
			visit(g.Imports[a])
		}
	}
	visit(f)
	return out
}

// copyJSON deep-copies the pen map, so that the copy of the file can be changed without touching the
// document the converter read.
func copyJSON(m map[string]any) (map[string]any, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// iconRects turns every icon node of the tree into a rectangle of the size of the icon, and returns how
// many it changed. An icon without a size takes the size of the view box, as go/scene draws it.
func iconRects(root map[string]any) int {
	set := iconSet()
	vw, vh := set.viewBox[2], set.viewBox[3]
	n := 0
	var walk func(m map[string]any)
	walk = func(m map[string]any) {
		if m["type"] == "icon" {
			m["type"] = "rectangle"
			if !positiveNumber(m["width"]) {
				m["width"] = vw
			}
			if !positiveNumber(m["height"]) {
				m["height"] = vh
			}
			n++
		}
		for _, k := range kids(m) {
			walk(k)
		}
	}
	walk(root)
	return n
}

func positiveNumber(v any) bool {
	n, ok := v.(float64)
	return ok && n > 0
}
