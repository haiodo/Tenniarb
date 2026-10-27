package scene

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// FromFileFlag is the argument of an element command that takes the content of the element from a file.
const FromFileFlag = "--from-file"

// Tree is a loaded workspace file with everything it imports.
type Tree struct {
	root  string // absolute, symlinks resolved
	main  *Node
	files map[string]*Node
	order []*Node // file roots in load order, for validation
	stack []loadFrame
	diags Errors
	seen  map[Error]bool
	sink  *Errors // collects every report of one Build, duplicates included

	overlay map[string]string // text of files given in memory, by relative slash path
	src     map[string]string // text of every loaded file, by relative slash path
}

type loadFrame struct {
	abs, rel string
	at       *Pos // the import that loaded this file; nil for the main file
}

// Load reads file (absolute, or relative to root) and its imports. root is the workspace root:
// no import may leave it. The tree is returned even with errors, unless the main file is unreadable;
// the error holds every problem, with warnings, and is nil when there are only warnings.
func Load(root, file string) (*Tree, error) { return LoadOverlay(root, file, nil) }

// LoadSource is Load where the text of file (relative to root) is given in memory: an edit that is
// not saved yet. file need not exist; the files it imports are read from disk.
func LoadSource(root, file, text string) (*Tree, error) {
	return LoadOverlay(root, file, map[string]string{path.Clean(filepath.ToSlash(file)): text})
}

// LoadOverlay is Load where overlay replaces the text of files; keys are slash paths relative to
// root. An overlaid file need not exist on disk.
func LoadOverlay(root, file string, overlay map[string]string) (*Tree, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if r, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = r
	}
	t := &Tree{root: absRoot, files: map[string]*Node{}, seen: map[Error]bool{}, overlay: overlay, src: map[string]string{}}
	if !filepath.IsAbs(file) {
		file = filepath.Join(absRoot, file)
	}
	abs, ok := t.inside(file)
	if !ok {
		return nil, fmt.Errorf("%s is outside the workspace root %s", file, absRoot)
	}
	t.main = t.loadFile(abs, nil)
	if t.main == nil {
		return nil, t.diags
	}
	t.validate()
	sort.SliceStable(t.diags, func(i, j int) bool {
		a, b := t.diags[i].Pos, t.diags[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
	})
	return t, t.diags.asError()
}

func (t *Tree) read(abs, rel string) (string, error) {
	if text, ok := t.overlay[rel]; ok {
		return text, nil
	}
	b, err := os.ReadFile(abs)
	return string(b), err
}

// File returns the root node of a loaded file (rel is a slash path relative to the workspace
// root), or nil when the file is not part of the tree.
func (t *Tree) File(rel string) *Node {
	for _, f := range t.order {
		if f.Pos.File == rel {
			return f
		}
	}
	return nil
}

// Source returns the text of a loaded file, the one Node.Range points into.
func (t *Tree) Source(rel string) (string, bool) {
	s, ok := t.src[rel]
	return s, ok
}

// Diagnostics lists the errors and warnings found so far.
func (t *Tree) Diagnostics() Errors { return t.diags }

func (t *Tree) report(pos Pos, warn bool, format string, args ...any) {
	e := Error{Pos: pos, Msg: fmt.Sprintf(format, args...), Warn: warn}
	if t.sink != nil {
		*t.sink = append(*t.sink, e)
	}
	if t.seen[e] {
		return
	}
	t.seen[e] = true
	t.diags = append(t.diags, e)
}

func (t *Tree) fail(pos Pos, format string, args ...any) { t.report(pos, false, format, args...) }
func (t *Tree) warn(pos Pos, format string, args ...any) { t.report(pos, true, format, args...) }

// inside cleans p and reports whether it lies in the workspace root, symlinks resolved when it exists.
func (t *Tree) inside(p string) (string, bool) {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	rel, err := filepath.Rel(t.root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p, false
	}
	return p, true
}

func (t *Tree) rel(abs string) string {
	r, err := filepath.Rel(t.root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(r)
}

type loader struct {
	t    *Tree
	file string // relative name, for positions
}

func (t *Tree) loadFile(abs string, at *Pos) *Node {
	for i, fr := range t.stack {
		if fr.abs == abs {
			t.importCycle(i, abs, at)
			return nil
		}
	}
	if f, ok := t.files[abs]; ok {
		return f
	}
	rel := t.rel(abs)
	src, err := t.read(abs, rel)
	if err != nil {
		pos := Pos{File: rel, Line: 1, Col: 1}
		if at != nil {
			pos = *at
		}
		t.fail(pos, "cannot read %s: %v", rel, err)
		return nil
	}
	t.stack = append(t.stack, loadFrame{abs: abs, rel: rel, at: at})
	root := &Node{Kind: KFile, Pos: Pos{File: rel, Line: 1, Col: 1}}
	t.src[rel] = src
	ast, parser := tenn.Parse(src)
	for _, e := range parser.Errors.Errors {
		t.fail(Pos{File: rel, Line: e.Line + 1, Col: e.Col}, "%s", e.Message)
	}
	l := &loader{t: t, file: rel}
	l.fill(root, ast.Children)
	t.stack = t.stack[:len(t.stack)-1]
	t.files[abs] = root
	t.order = append(t.order, root)
	return root
}

// importCycle reports a cycle that the import at closes onto stack[i]; the message holds the position
// of every import on the way round.
func (t *Tree) importCycle(i int, abs string, at *Pos) {
	var hops []string
	for k := i + 1; k < len(t.stack); k++ {
		hops = append(hops, fmt.Sprintf("%s imports %s", t.stack[k].at, t.stack[k].rel))
	}
	hops = append(hops, fmt.Sprintf("%s imports %s", at, t.rel(abs)))
	t.fail(*at, "import cycle: %s", strings.Join(hops, ", "))
}

func (l *loader) pos(c *tenn.Node) Pos {
	tok := c.Token
	if tok == nil {
		return Pos{File: l.file, Line: 1, Col: 1}
	}
	return Pos{File: l.file, Line: tok.Line + 1, Col: tok.Col - tok.Size + 1}
}

func rangeOf(c *tenn.Node) Range {
	if c.Token == nil {
		return Range{}
	}
	return Range{c.Token.Pos, c.End}
}

// bodyOf is the range of the last block of a command, which the parser ends the command with.
func bodyOf(c *tenn.Node) Range {
	if n := len(c.Children); n > 1 && c.Children[n-1].Kind == tenn.KindBlockExpr && c.Children[n-1].Token != nil {
		return Range{c.Children[n-1].Token.Pos, c.End}
	}
	return Range{}
}

func cmdName(c *tenn.Node) string {
	if c.Kind != tenn.KindCommand {
		return ""
	}
	s, _ := c.GetIdent(0)
	return s
}

// split separates the arguments of a command from its block.
func split(c *tenn.Node) (args []*tenn.Node, block []*tenn.Node) {
	for _, a := range c.Children[1:] {
		if a.Kind == tenn.KindBlockExpr {
			block = append(block, a.Children...)
		} else {
			args = append(args, a)
		}
	}
	return args, block
}

func (l *loader) prop(c *tenn.Node) *Prop {
	args, _ := split(c)
	p := &Prop{Name: cmdName(c), Args: args, Pos: l.pos(c), Range: rangeOf(c)}
	for _, a := range c.Children[1:] {
		if a.Kind == tenn.KindBlockExpr {
			p.Block = a
		}
	}
	return p
}

func argText(args []*tenn.Node, i int) string {
	if i >= len(args) {
		return ""
	}
	s, _ := args[i].GetIdentText()
	return s
}

// fill sorts the commands of an element, item or file into structure and properties.
func (l *loader) fill(owner *Node, cmds []*tenn.Node) {
	for _, c := range cmds {
		name := cmdName(c)
		if name == "" {
			continue
		}
		switch {
		case name == "element" && owner.isContainer():
			l.addElement(owner, c)
		case name == "import" && owner.isContainer():
			l.addImport(owner, c)
		case l.addContent(owner, c, name) != nil:
		default:
			owner.Props = append(owner.Props, l.prop(c))
		}
	}
}

func (l *loader) newNode(kind Kind, owner *Node, c *tenn.Node) *Node {
	n := &Node{Kind: kind, Parent: owner, Pos: l.pos(c), Range: rangeOf(c), Body: bodyOf(c)}
	if len(c.Children) > 1 && c.Children[1].Kind != tenn.KindBlockExpr {
		n.Name, _ = c.Children[1].GetIdentText()
	}
	n.Key = NormalizeName(n.Name)
	return n
}

func (l *loader) addElement(owner *Node, c *tenn.Node) {
	n := l.newNode(KElement, owner, c)
	owner.Elems = append(owner.Elems, n)
	args, block := split(c)
	if rel, ok := fromFile(args); ok {
		if len(block) > 0 {
			l.t.fail(n.Pos, "element %q takes its content from %s: remove the block", n.Name, rel)
		}
		l.mountFile(n, rel)
		return
	}
	l.fill(n, block)
}

// fromFile reads "--from-file <path>" among the arguments of an element command.
func fromFile(args []*tenn.Node) (string, bool) {
	for i, a := range args {
		if s, _ := a.GetIdentText(); s == FromFileFlag && i+1 < len(args) {
			return argText(args, i+1), true
		}
	}
	return "", false
}

// mountFile makes the content of the file the content of n: the lists are shared, so one file
// mounted twice is one subtree, and scopes inside it stay in that file.
func (l *loader) mountFile(n *Node, rel string) {
	abs, ok := l.resolveFile(rel, n.Pos, "element --from-file")
	if !ok {
		return
	}
	file := l.t.loadFile(abs, &n.Pos)
	if file == nil {
		return
	}
	n.Mount = file
	n.Props, n.Content, n.Elems, n.Mounts = file.Props, file.Content, file.Elems, file.Mounts
}

// resolveFile finds rel relative to the file being read; it fails when the path leaves the workspace root.
func (l *loader) resolveFile(rel string, pos Pos, what string) (string, bool) {
	target := filepath.Join(l.t.root, filepath.FromSlash(filepath.Dir(l.file)), filepath.FromSlash(rel))
	abs, in := l.t.inside(target)
	if !in {
		l.t.fail(pos, "%s %q leaves the workspace root", what, rel)
	}
	return abs, in
}

// addContent adds an item, link, use or slot command to owner; it returns nil when c is not one, so
// the caller keeps it as a property (a bare use, a slot outside a component).
func (l *loader) addContent(owner *Node, c *tenn.Node, name string) *Node {
	if !owner.holdsContent() && owner.Kind != KUse {
		return nil
	}
	args, block := split(c)
	var n *Node
	switch name {
	case "item":
		n = l.newNode(KItem, owner, c)
		l.fill(n, block)
	case "slot":
		if owner.Kind != KItem {
			return nil
		}
		n = l.newNode(KSlot, owner, c)
		l.fill(n, block)
	case "link":
		n = &Node{Kind: KLink, Parent: owner, Pos: l.pos(c), Range: rangeOf(c), Body: bodyOf(c), From: argText(args, 0), To: argText(args, 1)}
		for _, b := range block {
			if cmdName(b) != "" {
				n.Props = append(n.Props, l.prop(b))
			}
		}
	case "use":
		n = l.newUse(owner, c, args, block)
	}
	if n != nil {
		owner.Content = append(owner.Content, n)
	}
	return n
}

// newUse reads "use <path> as <name> { overrides }". A bare use stays a property.
func (l *loader) newUse(owner *Node, c *tenn.Node, args, block []*tenn.Node) *Node {
	if len(args) == 0 {
		return nil
	}
	pos := l.pos(c)
	if len(args) != 3 || argText(args, 1) != "as" {
		l.t.fail(pos, "use needs the form: use <path> as <name>")
		return nil
	}
	n := &Node{Kind: KUse, Parent: owner, Pos: pos, Range: rangeOf(c), Body: bodyOf(c), Path: argText(args, 0), Name: argText(args, 2)}
	n.Key = NormalizeName(n.Name)
	n.Block = l.overrides(n, block)
	return n
}

// overrides reads the block of a use. Nodes inside it, however deep, get the use as their scope.
func (l *loader) overrides(use *Node, cmds []*tenn.Node) []*Override {
	var out []*Override
	for _, c := range cmds {
		name := cmdName(c)
		if name == "" {
			continue
		}
		args, block := split(c)
		ov := &Override{Pos: l.pos(c)}
		switch name {
		case "at", "replace":
			ov.Kind, ov.Path = OvAt, argText(args, 0)
			if name == "replace" {
				ov.Kind = OvReplace
			}
			ov.Block = l.overrides(use, block)
		case "slot":
			ov.Kind, ov.Slot = OvSlot, argText(args, 0)
			ov.Block = l.overrides(use, block)
		default:
			if n := l.addContent(use, c, name); n != nil {
				ov.Kind, ov.Node = OvNode, n
			} else {
				ov.Kind, ov.Prop = OvProp, l.prop(c)
			}
		}
		out = append(out, ov)
	}
	return out
}

// addImport mounts the tree of another file under the alias.
func (l *loader) addImport(owner *Node, c *tenn.Node) {
	args, _ := split(c)
	pos := l.pos(c)
	rel := argText(args, 0)
	if rel == "" {
		l.t.fail(pos, "import needs a file path")
		return
	}
	alias := argText(args, 2)
	if len(args) < 3 || argText(args, 1) != "as" {
		alias = NormalizeName(strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)))
	}
	segs, ok := splitPath(alias)
	if !ok {
		l.t.fail(pos, "bad import alias %q", alias)
		return
	}
	abs, in := l.resolveFile(rel, pos, "import")
	if !in {
		return
	}
	file := l.t.loadFile(abs, &pos)
	if file == nil {
		return
	}
	l.mount(owner, segs, file, pos)
}

// mount puts file under owner as the path segs; the prefix segments are virtual nodes shared by
// the imports that start the same way.
func (l *loader) mount(owner *Node, segs []string, file *Node, pos Pos) {
	scope := owner
	for i, seg := range segs {
		last := i == len(segs)-1
		var found *Node
		for _, m := range scope.Mounts {
			if m.Key == seg {
				found = m
			}
		}
		switch {
		case last && found != nil:
			l.t.fail(pos, "import alias %s is already taken by %s", strings.Join(segs, "."), found.Pos)
			return
		case last:
			scope.Mounts = append(scope.Mounts, &Node{Kind: KMount, Name: seg, Key: seg, Pos: pos, Parent: owner, Target: file})
		case found == nil:
			found = &Node{Kind: KVirtual, Name: seg, Key: seg, Pos: pos, Parent: owner}
			scope.Mounts = append(scope.Mounts, found)
			scope = found
		case found.Kind == KVirtual:
			scope = found
		default:
			l.t.fail(pos, "import alias %s runs into the import at %s", strings.Join(segs, "."), found.Pos)
			return
		}
	}
}
