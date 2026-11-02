package scene

import (
	"errors"
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
	root  string              // absolute, symlinks resolved
	main  *Node               // the file loaded for; with an entry, a file mounted below the entry
	top   *Node               // the entry document: where lookups end; main when loaded alone
	ctxs  map[*Node]*mountCtx // file root -> the first chain of mounts that reaches it from top
	files map[string]*Node
	order []*Node // file roots in load order, for validation
	stack []loadFrame
	diags Errors
	seen  map[Error]bool
	sink  *Errors // collects every report of one Build, duplicates included
	quiet bool    // reports are dropped: References expands files again

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
	return LoadIn(root, file, "", overlay)
}

// LoadIn is LoadOverlay of a part file in the context of an entry document (spec 5a.1): the entry
// is loaded, and a name that file does not define itself is looked up where the file is mounted,
// up to the entry. entry is absolute or relative to root; "" loads file alone. The entry must mount
// file, directly or through other files; only file and what it mounts are validated.
func LoadIn(root, file, entry string, overlay map[string]string) (*Tree, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if r, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = r
	}
	t := &Tree{root: absRoot, files: map[string]*Node{}, seen: map[Error]bool{}, overlay: overlay, src: map[string]string{}}
	abs, ok := t.inside(t.abs(file))
	if !ok {
		return nil, fmt.Errorf("%s is outside the workspace root %s", file, absRoot)
	}
	if entry == "" {
		entry = abs
	}
	eabs, ok := t.inside(t.abs(entry))
	if !ok {
		return nil, fmt.Errorf("%s is outside the workspace root %s", entry, absRoot)
	}
	t.top = t.loadFile(eabs, nil)
	if t.top == nil {
		return nil, t.diags
	}
	t.mapMounts()
	if t.main = t.files[abs]; t.main == nil {
		return nil, fmt.Errorf("%s does not mount %s", t.rel(eabs), t.rel(abs))
	}
	t.validate()
	t.sortDiags()
	return t, t.diags.asError()
}

func (t *Tree) abs(file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(t.root, file)
}

func (t *Tree) sortDiags() {
	sort.SliceStable(t.diags, func(i, j int) bool {
		a, b := t.diags[i].Pos, t.diags[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
	})
}

// mapMounts records, for each file below the entry, the chain of mounts that reaches it first.
func (t *Tree) mapMounts() {
	t.ctxs = map[*Node]*mountCtx{t.top: nil}
	var walk func(n *Node, c *mountCtx)
	walk = func(n *Node, c *mountCtx) {
		for _, e := range n.Elems {
			m := e.Mount
			if m == nil {
				walk(e, c)
			} else if _, seen := t.ctxs[m]; !seen {
				t.ctxs[m] = c.push(e)
				walk(m, t.ctxs[m])
			}
		}
	}
	walk(t.top, nil)
}

// Entry is the entry document the tree was loaded in the context of, a slash path relative to the
// root; "" when the file was loaded alone.
func (t *Tree) Entry() string {
	if t.top == t.main {
		return ""
	}
	return t.top.Pos.File
}

// Diagnose is LoadOverlay for validation: every problem comes back as Errors, with warnings. The
// error is set only when the file could not be loaded at all. MCP validate and tenniarb check share it.
func Diagnose(root, file string, overlay map[string]string) (Errors, *Tree, error) {
	return diagnosed(LoadOverlay(root, file, overlay))
}

// DiagnoseIn is Diagnose of file in the context of entry, see LoadIn.
func DiagnoseIn(root, file, entry string, overlay map[string]string) (Errors, *Tree, error) {
	return diagnosed(LoadIn(root, file, entry, overlay))
}

// DiagnoseAuto is Diagnose where the entry of file is found as LoadAuto does.
func DiagnoseAuto(root, file string, overlay map[string]string) (Errors, *Tree, error) {
	return diagnosed(LoadAuto(root, file, overlay))
}

func diagnosed(tree *Tree, err error) (Errors, *Tree, error) {
	if tree != nil {
		return tree.Diagnostics(), tree, nil
	}
	var es Errors
	if errors.As(err, &es) {
		return es, nil, nil
	}
	return nil, nil, err
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
	if t.quiet {
		return
	}
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
	l.joinedBlocks(ast.Children)
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

// joinedBlocks warns on a command with two blocks. The format joins them into one command, so
// "item a { ... } item c { ... }" or "at A { ... } at B { ... }" on one line silently lose the
// second name: each block belongs on its own line.
func (l *loader) joinedBlocks(ns []*tenn.Node) {
	for _, n := range ns {
		if n.Kind == tenn.KindCommand {
			blocks := 0
			for _, c := range n.Children {
				if c.Kind == tenn.KindBlockExpr {
					blocks++
				}
			}
			if blocks > 1 {
				l.t.warn(l.pos(n), "%d blocks in one command %q are joined: put each block on its own line", blocks, cmdName(n))
			}
		}
		l.joinedBlocks(n.Children)
	}
}

func (l *loader) pos(c *tenn.Node) Pos { return nodePos(l.file, c) }

// nodePos is the position of a command of the file named file, 1-based.
func nodePos(file string, c *tenn.Node) Pos {
	if c.Token == nil {
		return Pos{File: file, Line: 1, Col: 1}
	}
	return Pos{File: file, Line: c.Token.Line + 1, Col: c.Token.Col - c.Token.Size + 1}
}

func rangeOf(c *tenn.Node) Range {
	if c.Token == nil {
		return Range{}
	}
	return Range{c.Token.Pos, c.End}
}

// argRange is the span of an argument in the text, with the quotes of a string.
func argRange(a *tenn.Node) Range {
	if a == nil || a.Token == nil {
		return Range{}
	}
	t := a.Token
	if t.Type == tenn.TokStringLit {
		return Range{t.Pos - 1, t.EndPos()}
	}
	return Range{t.Pos, t.EndPos()}
}

func argAt(args []*tenn.Node, i int) Range {
	if i >= len(args) {
		return Range{}
	}
	return argRange(args[i])
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
		case l.addContent(owner, c, name) != nil:
		case name == "styles" && owner.isContainer():
			l.addStyles(owner, c)
		case name == "kind": // kind Library in a file, kind Component in an element
			args, _ := split(c)
			l.setKind(owner, argText(args, 0), l.pos(c))
		case name == "export" && (owner.Kind == KElement || owner.Kind == KItem):
			l.addParam(owner, c)
		case name == "template" && owner.Kind == KElement:
			l.addTemplate(owner, c)
		default:
			owner.Props = append(owner.Props, l.prop(c))
		}
	}
}

func (l *loader) newNode(kind Kind, owner *Node, c *tenn.Node) *Node {
	n := &Node{Kind: kind, Parent: owner, Pos: l.pos(c), Range: rangeOf(c), Body: bodyOf(c)}
	if len(c.Children) > 1 && c.Children[1].Kind != tenn.KindBlockExpr {
		n.Name, _ = c.Children[1].GetIdentText()
		n.NameArg = argRange(c.Children[1])
	}
	n.Key = NormalizeName(n.Name)
	return n
}

func (l *loader) addElement(owner *Node, c *tenn.Node) {
	n := l.newNode(KElement, owner, c)
	owner.Elems = append(owner.Elems, n)
	args, block := split(c)
	if _, at, ok := kindOf(args); ok {
		l.t.warn(l.pos(at), "kind goes inside the element block: %q", `element "`+n.Name+`" { kind Component }`)
	}
	if rel, ok := fromFile(args); ok {
		if len(block) > 0 {
			l.t.fail(n.Pos, "element %q takes its content from %s: remove the block", n.Name, rel)
		}
		l.mountFile(n, rel)
		return
	}
	l.fill(n, block)
}

// kindOf reads "kind <name>" among the arguments of an element command. The kind goes inside the block:
// the classic model reads the content of an element at child 2, so an argument before the block hides
// everything the element holds from the app that draws classic diagrams.
func kindOf(args []*tenn.Node) (string, *tenn.Node, bool) {
	for i, a := range args {
		if s, _ := a.GetIdentText(); s == "kind" && i+1 < len(args) {
			return argText(args, i+1), args[i+1], true
		}
	}
	return "", nil, false
}

// setKind records the kind of n, from "kind Component" on an element or "kind Library" in a file. A kind
// the format does not know is a warning: the command stays harmless (spec 5c).
func (l *loader) setKind(n *Node, name string, pos Pos) {
	n.Type = strings.ToLower(name)
	switch n.Type {
	case TypeComponent, TypeScreen, TypeLibrary, TypeApplication:
	default:
		l.t.warn(pos, "kind %q is not one of %s, %s, %s, %s", name, TypeComponent, TypeScreen, TypeLibrary, TypeApplication)
	}
}

// addParam reads "export let name = value": a parameter of the element, with its default (spec 5c).
// Without "=" the parameter is required, and a use has to set it.
func (l *loader) addParam(owner *Node, c *tenn.Node) {
	args, _ := split(c)
	pos := l.pos(c)
	if len(args) != 2 && len(args) != 4 || argText(args, 0) != "let" || len(args) == 4 && argText(args, 2) != "=" {
		l.t.fail(pos, "a parameter is written: export let <name> or export let <name> = <value>")
		return
	}
	name := argText(args, 1)
	if owner.Param(name) != nil {
		l.t.warn(l.pos(args[1]), "parameter %q is declared twice", name)
		return
	}
	p := Param{Name: name, Pos: l.pos(args[1])}
	if len(args) == 4 {
		p.Default = &Prop{Name: name, Args: args[3:], Pos: l.pos(args[3]), Range: spanOf(args[3:])}
	}
	owner.Params = append(owner.Params, p)
}

// spanOf is the range from the first to the last of args; zero when there is none.
func spanOf(args []*tenn.Node) Range {
	if len(args) == 0 || args[0].Token == nil {
		return Range{}
	}
	return Range{args[0].Token.Pos, args[len(args)-1].Token.EndPos()}
}

// addTemplate reads "template { ... }", "template %{ ... }" or "template "..."": the code the element
// generates (spec 5c). A block is kept as written - the commands the parser reads inside it are not read
// as commands - so the text of the file is the text of the template.
func (l *loader) addTemplate(owner *Node, c *tenn.Node) {
	pos := l.pos(c)
	if owner.Template != nil {
		l.t.warn(pos, "the element has a second template: the first one is used")
		return
	}
	args, _ := split(c)
	text := ""
	switch {
	case hasBlock(c):
		src := l.t.src[l.file]
		if b := bodyOf(c); b.Start+1 <= b.End-1 && b.End <= len(src) {
			text = src[b.Start+1 : b.End-1]
		}
	case len(args) > 0 && (args[0].Kind == tenn.KindMarkdownLit || args[0].Kind == tenn.KindStringLit):
		text, _ = args[0].GetIdentText()
		// "%{ ... %}" is what a reader writes: the markdown literal closes at "}", so the percent of the
		// closing marker lands in the text and is cut here.
		if a := args[0]; a.Token != nil && strings.HasSuffix(text, "%") {
			at := a.Token.Pos + len(text) // the literal is a slice of the file text
			if src := l.t.src[l.file]; at < len(src) && src[at] == '}' {
				text = strings.TrimSuffix(text, "%")
			}
		}
	default:
		l.t.fail(pos, "template needs text: template { ... } or template %%{ ... }")
		return
	}
	owner.Template = &Template{Text: dedent(text), Pos: pos}
}

// dedent cuts the blank lines around text and the indentation its lines share, so the code of a template
// starts at column 0 and reads the way it is written in a component.
func dedent(text string) string {
	lines := strings.Split(strings.Trim(text, "\n"), "\n")
	indent := -1
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if n := len(ln) - len(strings.TrimLeft(ln, " \t")); indent < 0 || n < indent {
			indent = n
		}
	}
	out := make([]string, 0, len(lines))
	if indent < 0 {
		indent = 0
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			out = append(out, "")
		} else {
			out = append(out, ln[indent:])
		}
	}
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
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
	n.Props, n.Content, n.Elems = file.Props, file.Content, file.Elems
	if n.Type == "" { // the kind of the mounted file, unless the mounting element says its own
		n.Type = file.Type
	}
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

// addStyles reads "styles { name { properties } ... }": the named styles an element gives its nodes.
// A command without a block is a group declaration for the inspector, not a style.
func (l *loader) addStyles(owner *Node, c *tenn.Node) {
	_, block := split(c)
	if owner.Styles == nil {
		owner.Styles = map[string]*Style{}
	}
	for _, b := range block {
		name := cmdName(b)
		if name == "" {
			continue
		}
		if !hasBlock(b) {
			if g, ok := styleGroup(b); ok {
				owner.StyleGroups = append(owner.StyleGroups, g)
			}
			continue
		}
		owner.Styles[NormalizeName(name)] = l.style(b)
	}
}

// styleGroup reads a group declaration "name one-of a b" or "name any-of a b"; ok is false for any other
// command without a block, which is no group.
func styleGroup(c *tenn.Node) (StyleGroup, bool) {
	args, _ := split(c)
	mode := argText(args, 0)
	if mode != "one-of" && mode != "any-of" {
		return StyleGroup{}, false
	}
	g := StyleGroup{Name: NormalizeName(cmdName(c)), AnyOf: mode == "any-of"}
	for i := 1; i < len(args); i++ {
		g.Styles = append(g.Styles, NormalizeName(argText(args, i)))
	}
	return g, true
}

// style reads one style: its properties and the blocks of kid names in it.
func (l *loader) style(c *tenn.Node) *Style {
	st := &Style{}
	_, block := split(c)
	for _, cmd := range block {
		name := cmdName(cmd)
		if name == "" {
			continue
		}
		if hasBlock(cmd) {
			if name == "animation" {
				l.t.warn(l.pos(cmd), "the keyframes block goes in the element, not in a style: write it next to styles")
				continue
			}
			st.Kids = append(st.Kids, StyleKid{Key: NormalizeName(name), Style: l.style(cmd)})
			continue
		}
		st.Props = append(st.Props, l.prop(cmd))
	}
	return st
}

func hasBlock(c *tenn.Node) bool {
	return len(c.Children) > 0 && c.Children[len(c.Children)-1].Kind == tenn.KindBlockExpr
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
		n = &Node{Kind: KLink, Parent: owner, Pos: l.pos(c), Range: rangeOf(c), Body: bodyOf(c), From: argText(args, 0), To: argText(args, 1),
			FromArg: argAt(args, 0), ToArg: argAt(args, 1)}
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
	n := &Node{Kind: KUse, Parent: owner, Pos: pos, Range: rangeOf(c), Body: bodyOf(c), Path: argText(args, 0), Name: argText(args, 2),
		PathArg: argAt(args, 0), NameArg: argAt(args, 2)}
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
		ov := &Override{Pos: l.pos(c), Range: rangeOf(c), Body: bodyOf(c)}
		switch name {
		case "at", "replace":
			ov.Kind, ov.Path, ov.PathArg = OvAt, argText(args, 0), argAt(args, 0)
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
