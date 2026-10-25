package penimport

import (
	"fmt"
	"math"
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
}

func (r *Report) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "fidelity %s: %d go/scene warnings, %d errors\n", r.File, r.Warnings, len(r.Problems))
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
	d, err := pen.Open(f.Path, pen.Options{Measurer: m})
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
