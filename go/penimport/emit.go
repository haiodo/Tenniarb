package penimport

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Result is a converted set of files, libraries first.
type Result struct {
	Files []*File
	c     *conv
}

// Convert reads the .pen files with the libraries they import and converts all of them.
func Convert(paths []string, o Options) (*Result, error) {
	c := newConv(o)
	for _, p := range paths {
		if _, err := c.load(p); err != nil {
			return nil, err
		}
	}
	for _, f := range c.order {
		c.buildFile(f)
	}
	for _, f := range c.order {
		c.finishAll(f)
	}
	c.finalizeNames()
	return &Result{Files: c.order, c: c}, nil
}

func simpleIdent(s string) bool {
	for i, r := range s {
		if !(unicode.IsLetter(r) || r == '_' || i > 0 && unicode.IsDigit(r)) {
			return false
		}
	}
	return s != ""
}

func argNode(a Arg) *tenn.Node {
	if a.Quoted {
		return tenn.NewStrNode(a.S)
	}
	return tenn.NewIdent(a.S)
}

func propNode(p Prop) *tenn.Node {
	args := make([]*tenn.Node, len(p.Args))
	for i, a := range p.Args {
		args[i] = argNode(a)
	}
	return tenn.NewCommand(p.Name, args...)
}

func withBlock(cmd *tenn.Node, body []*tenn.Node) *tenn.Node {
	if len(body) > 0 {
		cmd.Add(tenn.NewBlockExpr(body...))
	}
	return cmd
}

func (r *Result) emitNode(n *Node) *tenn.Node {
	var body []*tenn.Node
	for _, p := range n.Props {
		body = append(body, propNode(p))
	}
	switch n.Kind {
	case KSlot:
		for _, k := range n.Kids {
			body = append(body, r.emitNode(k))
		}
		return withBlock(tenn.NewCommand("slot", tenn.NewIdent(n.Slot)), body)
	case KUse:
		for _, ov := range n.Block {
			body = append(body, r.emitOvr(ov))
		}
		as := tenn.NewStrNode(n.Name)
		if simpleIdent(n.Name) {
			as = tenn.NewIdent(n.Name)
		}
		return withBlock(tenn.NewCommand("use", tenn.NewIdent(pathTo(n.File, n.Target)), tenn.NewIdent("as"), as), body)
	}
	leaf := n.Typ != "text"
	for _, k := range n.Kids {
		leaf = leaf && k.Kind == KSlot
		body = append(body, r.emitNode(k))
	}
	if leaf && !n.b.layout && n.Typ != "frame" && !hasProp(n.Props, "path") {
		// outside a layout the name of an item is its text, and a leaf is as big as its text
		body = append([]*tenn.Node{propNode(prop("title", strArg("")))}, body...)
	}
	return withBlock(tenn.NewCommand("item", tenn.NewStrNode(n.Name)), body)
}

func hasProp(ps []Prop, name string) bool {
	for _, p := range ps {
		if p.Name == name {
			return true
		}
	}
	return false
}

func chainPath(chain []*Node) string {
	var segs []string
	for _, n := range chain {
		if seg, ok := pathSeg(n); ok {
			segs = append(segs, seg)
		}
	}
	return strings.Join(segs, ".")
}

func (r *Result) emitOvr(ov *Ovr) *tenn.Node {
	var body []*tenn.Node
	switch ov.Kind {
	case OvAt:
		for _, p := range ov.Props {
			body = append(body, propNode(p))
		}
		return withBlock(tenn.NewCommand("at", tenn.NewIdent(chainPath(ov.Chain))), body)
	case OvReplace:
		for _, n := range ov.Nodes {
			body = append(body, r.emitNode(n))
		}
		return withBlock(tenn.NewCommand("replace", tenn.NewIdent(chainPath(ov.Chain))), body)
	}
	for _, n := range ov.Nodes {
		body = append(body, r.emitNode(n))
	}
	fill := withBlock(tenn.NewCommand("slot", tenn.NewIdent(ov.Slot.Slot)), body)
	if len(ov.Chain) > 0 {
		return withBlock(tenn.NewCommand("at", tenn.NewIdent(chainPath(ov.Chain))), []*tenn.Node{fill})
	}
	return fill
}

// Tenn prints the converted file.
func (r *Result) Tenn(f *File) string {
	stmts := tenn.NewNode(tenn.KindStatements, nil)
	var imports []*File
	for _, lib := range f.Imports {
		imports = append(imports, lib)
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Name < imports[j].Name })
	for _, lib := range imports {
		stmts.Add(tenn.NewCommand("element", tenn.NewStrNode(f.mountAlias(lib)), tenn.NewIdent(scene.FromFileFlag), tenn.NewStrNode(lib.Name+".tenn")))
	}
	for _, n := range f.Root.Kids {
		stmts.Add(r.emitNode(n))
	}
	return tenn.Compact(stmts) + "\n"
}

// Map is the side table: the pen id of every node of the file tree and the path that names it in
// the .tenn file, one pair per line.
func (r *Result) Map(f *File) string {
	var sb strings.Builder
	eachNode(f.Root, func(n *Node) {
		if n.ID == "" || n.Kind == KFile || n.Parent == nil || !inTree(n, f.Root) {
			return
		}
		fmt.Fprintf(&sb, "%s\t%s\n", n.ID, pathTo(f, n))
	})
	return sb.String()
}

func inTree(n, root *Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p.Kind == KUse && p != n {
			return false
		}
		if p == root {
			return true
		}
	}
	return false
}

// Vars lists the variables whose values were written as colors, with the number of uses. .tenn has
// no variables yet.
func (r *Result) Vars(f *File) string {
	names := make([]string, 0, len(f.Variables))
	for n := range f.Variables {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		fmt.Fprintf(&sb, "%s\t%d\n", n, f.Variables[n])
	}
	return sb.String()
}

// Write saves a .tenn, a .map and a .vars file per converted file into dir.
func (r *Result) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range r.Files {
		base := filepath.Join(dir, f.Name)
		if err := os.WriteFile(base+".tenn", []byte(r.Tenn(f)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(base+".map", []byte(r.Map(f)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(base+".vars", []byte(r.Vars(f)), 0o644); err != nil {
			return err
		}
	}
	return nil
}
