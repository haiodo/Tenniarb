// tennranges prints the byte range of every scene node and property of a .tenn file as JSON.
//
//	tennranges [-root DIR] FILE > out.json
//
// It is the Go side of the scene range stand: what it prints is the golden of
// TenniarbTests/SceneRangeTests.swift. The scene editor replaces text[Start:End) with a new command
// (go/app/sceneedit.go, TextEdit), so Start and End are the bytes Node.Range and Prop.Range hold.
// The output is stable: keys sorted, two space indent.
//
// Every entry is one command of the loaded tree: its kind (file, element, item, link, use, slot, or
// prop), its dotted path from the file root, the file the command is written in, and its byte range
// with the 1-based line and column of its start. Files a fixture mounts with --from-file are listed
// too.
//
// A styles block is read into Node.Styles, not Node.Props: its properties are not listed here.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/haiodo/tenniarb/go/scene"
)

func main() {
	root := flag.String("root", ".", "workspace root: mounted files may not leave it")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tennranges [-root DIR] FILE > out.json")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	out, err := ranges(*root, flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}

func ranges(root, file string) ([]byte, error) {
	tree, err := scene.Load(root, file)
	if err != nil {
		return nil, err
	}
	d := &dump{file: file}
	for _, name := range tree.Files() {
		d.addFile(tree.File(name))
	}
	sort.SliceStable(d.items, func(i, j int) bool {
		a, b := d.items[i], d.items[j]
		if a["file"] != b["file"] {
			return a["file"].(string) < b["file"].(string)
		}
		return a["start"].(int) < b["start"].(int)
	})
	return json.MarshalIndent(map[string]any{"file": d.file, "items": d.items}, "", "  ")
}

type dump struct {
	file  string
	items []map[string]any
}

// addFile lists one file of the tree: its root and every node and property below it.
func (d *dump) addFile(root *scene.Node) {
	if root == nil {
		return
	}
	var file []map[string]any
	d.node(&file, root, "")
	sort.SliceStable(file, func(i, j int) bool { return file[i]["start"].(int) < file[j]["start"].(int) })
	d.items = append(d.items, file...)
}

func (d *dump) node(out *[]map[string]any, n *scene.Node, path string) {
	d.add(out, n.Kind.String(), path, n.Pos, n.Range)
	// The lists of a mounting element are the lists of the file it mounts, which is listed on its own.
	if n.Mount != nil {
		return
	}
	for _, p := range n.Props {
		d.add(out, "prop", propPath(path, p.Name), p.Pos, p.Range)
	}
	// The properties of a use are overrides; its nodes are in Content, like every other content node.
	d.overrides(out, n.Block, path)
	for _, c := range n.Elems {
		d.node(out, c, joinPath(path, c.Name))
	}
	for _, c := range n.Content {
		d.node(out, c, joinPath(path, c.Name))
	}
}

func (d *dump) overrides(out *[]map[string]any, ovs []*scene.Override, path string) {
	for _, ov := range ovs {
		if ov.Prop != nil {
			d.add(out, "prop", propPath(path, ov.Prop.Name), ov.Prop.Pos, ov.Prop.Range)
		}
		d.overrides(out, ov.Block, path)
	}
}

func (d *dump) add(out *[]map[string]any, kind, path string, pos scene.Pos, r scene.Range) {
	*out = append(*out, map[string]any{
		"kind":  kind,
		"path":  path,
		"file":  pos.File,
		"start": r.Start,
		"end":   r.End,
		"line":  pos.Line,
		"col":   pos.Col,
	})
}

// joinPath adds a segment the way a path writes it, and skips the empty name of an unnamed node.
func joinPath(parent, name string) string {
	seg := scene.SegText(name)
	switch {
	case parent == "":
		return seg
	case seg == "":
		return parent
	}
	return parent + "." + seg
}

// propPath is a property below a node, without the dot a file root has no use for.
func propPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}
