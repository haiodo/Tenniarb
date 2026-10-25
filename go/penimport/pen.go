// Package penimport converts pen.dev design files (.pen, JSON) into the .tenn scene format of
// docs/spec/tenn-scene.md: item trees with auto-layout, use/at/replace/slot instead of ref/descendants.
//
// A .pen file is one canvas, so a .tenn file is one scene: every top level pen node becomes an item at
// the file root, at its canvas position (Y up, so pos y is -y). There is no element per screen: it
// would repeat the name of every screen as a second segment of the paths.
package penimport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options configure Convert.
type Options struct {
}

type Kind uint8

const (
	KFile Kind = iota
	KItem
	KUse
	KSlot
)

// Arg is a command argument: a quoted string or a bare word or number.
type Arg struct {
	S      string
	Quoted bool
}

type Prop struct {
	Name string
	Args []Arg
}

// Node is a node of the produced scene before it is printed.
type Node struct {
	Kind Kind
	File *File
	ID   string // pen id, empty for nodes made up here
	Typ  string // pen type of the node, of the root of the component for a use
	// res is the file the pen refs and ids of the node are written in; it differs from File for a copy
	// of a node of a library that a screen file holds in a block.
	res    *File
	Pen    map[string]any
	Name   string
	Parent *Node // scope: lookups go up this chain
	Props  []Prop
	Kids   []*Node // item and slot children, file root content
	// KUse
	Target  *Node
	Block   []*Ovr
	Content []*Node // nodes written inside the block: they are in scope of paths in it
	// KSlot
	Slot string
	// ref is the pen ref string of a use, kept for messages.
	ref     string
	state   uint8
	needed  bool
	missing bool
	b       bctx
}

type OvKind uint8

const (
	OvAt OvKind = iota
	OvReplace
	OvSlot
)

// Ovr is one command in the block of a use. Chain names the target by the nodes on the way down from
// the instance root; the path is written from their final names.
type Ovr struct {
	Kind  OvKind
	Chain []*Node
	Props []Prop
	Nodes []*Node
	Slot  *Node
}

// File is one .pen file with the libraries it imports.
type File struct {
	Path string
	// Name is the output file name without the extension: ui-kit.lib for ui-kit.lib.pen.
	Name    string
	Root    *Node
	Imports map[string]*File // pen alias -> library
	// Alias is the name the other files import this one by.
	Alias string

	raw     map[string]any
	rawIdx  map[string]map[string]any
	vars    map[string]map[string]any
	index   map[string]*Node
	dropped map[string]bool
	slots   map[string]bool
	// Tops lists the top level pen nodes that became items: the pen index and the node.
	Tops []Top
	// Unsupported counts what was left out or approximated, by feature.
	Unsupported map[string]int
	// Variables lists the variables whose values were resolved to concrete colors.
	Variables map[string]int
	Warnings  []string
	// Placeholders lists the refs that point nowhere.
	Placeholders []string
}

type Top struct {
	Index int
	Node  *Node
}

func (f *File) unsup(feature string) { f.Unsupported[feature]++ }

func (f *File) warn(format string, args ...any) {
	f.Warnings = append(f.Warnings, fmt.Sprintf(format, args...))
}

func kids(m map[string]any) []map[string]any {
	l, _ := m["children"].([]any)
	out := make([]map[string]any, 0, len(l))
	for _, c := range l {
		if cm, ok := c.(map[string]any); ok {
			out = append(out, cm)
		}
	}
	return out
}

func splitRef(s string) (alias, id string) {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

func (c *conv) load(path string) (*File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if f, ok := c.files[abs]; ok {
		return f, nil
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}
	base := strings.TrimSuffix(filepath.Base(abs), ".pen")
	f := &File{Path: abs, Name: base, Imports: map[string]*File{}, raw: root, rawIdx: map[string]map[string]any{},
		vars: map[string]map[string]any{}, index: map[string]*Node{}, dropped: map[string]bool{}, slots: map[string]bool{},
		Unsupported: map[string]int{}, Variables: map[string]int{}}
	f.Alias = normalize(strings.TrimSuffix(base, ".lib"))
	f.Root = &Node{Kind: KFile, File: f}
	c.files[abs] = f
	if v, ok := root["variables"].(map[string]any); ok {
		for k, x := range v {
			if xm, ok := x.(map[string]any); ok {
				f.vars[k] = xm
			}
		}
	}
	var walk func(m map[string]any)
	walk = func(m map[string]any) {
		if id, ok := m["id"].(string); ok {
			f.rawIdx[id] = m
		}
		for _, k := range kids(m) {
			walk(k)
		}
	}
	walk(root)
	imp, _ := root["imports"].(map[string]any)
	for alias, rel := range imp {
		lib, err := c.load(filepath.Join(filepath.Dir(abs), fmt.Sprint(rel)))
		if err != nil {
			return nil, fmt.Errorf("%s: import %s: %w", abs, alias, err)
		}
		f.Imports[alias] = lib
	}
	c.order = append(c.order, f)
	return f, nil
}

// lookupRaw finds the pen map and the file of a ref string as written in f.
func (f *File) lookupRaw(ref string) (*File, map[string]any) {
	alias, id := splitRef(ref)
	g := f
	if alias != "" {
		g = f.Imports[alias]
	}
	if g == nil {
		return nil, nil
	}
	m := g.rawIdx[id]
	if m == nil {
		return nil, nil
	}
	return g, m
}

// rootType follows refs down to the type of the node an instance is made of.
func (f *File) rootType(m map[string]any) string {
	for depth := 0; depth < 40; depth++ {
		t, _ := m["type"].(string)
		if t != "ref" {
			return t
		}
		s, _ := m["ref"].(string)
		g, c := f.lookupRaw(s)
		if c == nil {
			return "frame"
		}
		f, m = g, c
	}
	return "frame"
}

// resolve finds the node of a ref string as written in f.
func (f *File) resolve(ref string) *Node {
	alias, id := splitRef(ref)
	g := f
	if alias != "" {
		g = f.Imports[alias]
	}
	if g == nil {
		return nil
	}
	return g.index[id]
}
