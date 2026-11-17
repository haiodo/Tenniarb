// tenncomponents prints what the libraries a .tenn document mounts offer as components, as JSON.
//
//	tenncomponents [-root DIR] [-q TEXT] [-limit N] FILE > out.json
//	tenncomponents [-root DIR] [-q TEXT] [-limit N] -dir DIR > out.json
//
// It is the Go side of the component stand: the Swift tests build it and compare the listing of the
// AppKit component browser against it. The listing comes from app.Components (go/app/nav.go), the
// function the browser itself calls, so the two sides read one rule. The output is stable: two space
// indent, fixed keys, files and mounts in walk order.
//
// One object per file: the name it was read from and, for every element that mounts a file with
// --from-file, the name that element gives the library (the start of a use path), the mounted file,
// and what app.Components lists for it - the path a use names, the name, and the group it is filed
// under. A document with no mounts has an empty list.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/haiodo/tenniarb/go/app"
	"github.com/haiodo/tenniarb/go/scene"
)

func main() {
	root := flag.String("root", ".", "workspace root: mounted files may not leave it")
	dir := flag.String("dir", "", "print every .tenn in DIR as one array; DIR reads like FILE")
	q := flag.String("q", "", "only components whose name or key holds TEXT")
	limit := flag.Int("limit", 200, "at most LIMIT components of a library")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tenncomponents [-root DIR] [-q TEXT] [-limit N] FILE > out.json\n       tenncomponents [-root DIR] [-q TEXT] [-limit N] -dir DIR > out.json")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *dir != "" {
		out, err := all(*root, *dir, *q, *limit)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Stdout.Write(out)
		return
	}
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	d, err := listing(*root, flag.Arg(0), *q, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}

// all is the listing of every .tenn in dir, sorted by name: one process for the whole stand.
func all(root, dir, q string, limit int) ([]byte, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	pattern := filepath.Join(dir, "*.tenn")
	if !filepath.IsAbs(dir) {
		pattern = filepath.Join(absRoot, dir, "*.tenn")
	}
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := make([]document, 0, len(files))
	for _, f := range files {
		d, err := listing(root, f, q, limit)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		out = append(out, d)
	}
	return json.MarshalIndent(out, "", "  ")
}

// listing is one document as the window reads it: scene.LoadAuto so a part file resolves in the
// context of its entry, and the libraries of its own file root.
func listing(root, file, q string, limit int) (document, error) {
	abs, rel, err := paths(root, file)
	if err != nil {
		return document{}, err
	}
	tree, err := scene.LoadAuto(root, abs, nil)
	if tree == nil {
		return document{}, err
	}
	// Scene errors are the window's banner: the browser still lists the tree it loaded.
	n := tree.File(rel)
	if n == nil {
		return document{}, fmt.Errorf("%s is not in the loaded tree", rel)
	}
	out := document{File: filepath.Base(file), Mounts: make([]mount, 0, 4)}
	for _, l := range libraries(n) {
		out.Mounts = append(out.Mounts, mount{
			Alias:      l.alias,
			File:       l.file,
			Components: components(l.alias, tree.File(l.file), q, limit),
		})
	}
	return out, nil
}

// paths is the absolute path scene.Load reads and the slash path from root a tree keys files by. A
// relative file is relative to root, as scene.Load reads it.
func paths(root, file string) (abs, rel string, err error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	if r, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = r
	}
	abs = file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(absRoot, abs)
	}
	abs = filepath.Clean(abs)
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	rel, err = filepath.Rel(absRoot, abs)
	if err != nil {
		return "", "", err
	}
	return abs, filepath.ToSlash(rel), nil
}

// library is a file the document mounts with --from-file, under the name of the mounting element.
type library struct {
	alias, file string
}

// libraries are the files the document mounts, each once in walk order: the walk of the component
// browser (go/app/outline_scene.go, libraries).
func libraries(f *scene.Node) []library {
	var out []library
	seen := map[*scene.Node]bool{}
	var walk func(n *scene.Node)
	walk = func(n *scene.Node) {
		for _, c := range n.Elems {
			if c.Mount != nil && !seen[c.Mount] {
				seen[c.Mount] = true
				out = append(out, library{alias: c.Key, file: c.Mount.Pos.File})
			}
			walk(c)
		}
	}
	walk(f)
	return out
}

// components is app.Components without the Node, which JSON does not carry.
func components(alias string, root *scene.Node, q string, limit int) []component {
	out := make([]component, 0, 8)
	if root == nil {
		return out
	}
	for _, c := range app.Components(alias, root, q, limit) {
		out = append(out, component{Path: c.Path, Name: c.Name, Group: c.Group})
	}
	return out
}

// document is one file of the stand: its base name and what its libraries offer.
type document struct {
	File   string  `json:"file"`
	Mounts []mount `json:"mounts"`
}

// mount is one library the document mounts.
type mount struct {
	Alias      string      `json:"alias"`
	File       string      `json:"file"`
	Components []component `json:"components"`
}

// component is one row of the browser.
type component struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Group string `json:"group"`
}
