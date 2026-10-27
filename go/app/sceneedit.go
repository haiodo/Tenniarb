package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// TextEdit replaces Text[Start:End) of a document by New. Offsets are bytes.
type TextEdit struct {
	Start, End int
	New        string
}

// CommandText prints one command with its arguments the way the compact printer writes them: a word
// or a number bare, anything else quoted.
func CommandText(name string, args ...string) string {
	nodes := make([]*tenn.Node, 0, len(args))
	for _, a := range args {
		nodes = append(nodes, argNode(a))
	}
	return strings.TrimSpace(tenn.Compact(tenn.NewCommand(name, nodes...)))
}

// argNode keeps a number, a hex colour, an expression and a plain word as written, and quotes the rest.
func argNode(a string) *tenn.Node {
	if a == "" {
		return tenn.NewStrNode(a)
	}
	ast, p := tenn.Parse("x " + a)
	if !p.Errors.HasErrors() && len(ast.Children) == 1 && len(ast.Children[0].Children) == 2 {
		n := ast.Children[0].Children[1]
		if n.Kind != tenn.KindStringLit && n.Kind != tenn.KindBlockExpr && n.Token != nil && n.Token.Literal == a {
			return n
		}
	}
	return tenn.NewStrNode(a)
}

// ownProp is the command of name written on n itself: in the block of an item, or among the overrides
// of a use. The last one wins, as in go/scene.
func ownProp(n *scene.Node, name string) *scene.Prop {
	var found *scene.Prop
	switch n.Kind {
	case scene.KItem, scene.KElement, scene.KLink:
		for _, p := range n.Props {
			if p.Name == name {
				found = p
			}
		}
	case scene.KUse:
		for _, ov := range n.Block {
			if ov.Kind == scene.OvProp && ov.Prop.Name == name {
				found = ov.Prop
			}
		}
	}
	return found
}

// SetPropEdit writes `name args` on n: in place of its own command, else as a new command in its block.
// No args removes the command.
func SetPropEdit(text string, n *scene.Node, name string, args ...string) (TextEdit, bool) {
	own := ownProp(n, name)
	if len(args) == 0 {
		if own == nil {
			return TextEdit{}, false
		}
		return removeEdit(text, own.Range), true
	}
	line := CommandText(name, args...)
	if own != nil {
		return TextEdit{own.Range.Start, own.Range.End, line}, true
	}
	return InsertEdit(text, n, line), true
}

// lineStart is the offset of the start of the line that holds off.
func lineStart(text string, off int) int { return strings.LastIndexByte(text[:off], '\n') + 1 }

// indentAt is the white space that starts the line of off.
func indentAt(text string, off int) string {
	ls := lineStart(text, off)
	i := ls
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return text[ls:i]
}

func blank(s string) bool { return strings.TrimLeft(s, " \t") == "" }

// removeEdit deletes a command: with its line when it is alone on it, else with the ; before or after it.
func removeEdit(text string, r scene.Range) TextEdit {
	ls := lineStart(text, r.Start)
	le := strings.IndexByte(text[r.End:], '\n')
	if le < 0 {
		le = len(text)
	} else {
		le += r.End
	}
	if blank(text[ls:r.Start]) && blank(text[r.End:le]) {
		if le < len(text) {
			le++
		}
		return TextEdit{ls, le, ""}
	}
	before := strings.TrimRight(text[:r.Start], " \t")
	if strings.HasSuffix(before, ";") {
		return TextEdit{len(before) - 1, r.End, ""}
	}
	after := r.End
	for after < len(text) && (text[after] == ' ' || text[after] == '\t') {
		after++
	}
	if after < len(text) && text[after] == ';' {
		after++
		for after < len(text) && text[after] == ' ' {
			after++
		}
		return TextEdit{r.Start, after, ""}
	}
	return TextEdit{r.Start, r.End, ""}
}

// DeleteEdit removes the node n.
func DeleteEdit(text string, n *scene.Node) TextEdit { return removeEdit(text, n.Range) }

// indentUnit guesses the indentation step of the file from its first indented line; two spaces when none.
func indentUnit(text string) string {
	for _, l := range strings.SplitN(text, "\n", 200) {
		if t := strings.TrimLeft(l, " \t"); t != "" && len(t) < len(l) {
			return l[:len(l)-len(t)]
		}
	}
	return "  "
}

func indentLines(s, ind string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = ind + l
		}
	}
	return strings.Join(lines, "\n")
}

// InsertEdit adds cmd (one or more lines) at the end of the content of n: the block of an element, an
// item or a use, a new block when n has none, the end of the file for a file root or for an element
// with --from-file.
func InsertEdit(text string, n *scene.Node, cmd string) TextEdit {
	if n.Mount != nil { // the content of an element with --from-file is written in that file
		n = n.Mount
	}
	unit := indentUnit(text)
	if n.Kind == scene.KFile {
		at := len(text)
		pre := ""
		if at > 0 && text[at-1] != '\n' {
			pre = "\n"
		}
		return TextEdit{at, at, pre + cmd + "\n"}
	}
	if n.Body == (scene.Range{}) {
		if strings.Contains(cmd, "\n") {
			ind := indentAt(text, n.Range.Start)
			return TextEdit{n.Range.End, n.Range.End, " {\n" + indentLines(cmd, ind+unit) + "\n" + ind + "}"}
		}
		return TextEdit{n.Range.End, n.Range.End, " { " + cmd + " }"}
	}
	open, closing := n.Body.Start, n.Body.End-1
	inner := text[open+1 : closing]
	if blank(strings.ReplaceAll(inner, "\n", "")) && !strings.Contains(cmd, "\n") {
		return TextEdit{open + 1, closing, " " + cmd + " "}
	}
	if !strings.Contains(inner, "\n") && !strings.Contains(cmd, "\n") {
		end := open + 1 + len(strings.TrimRight(inner, " \t"))
		return TextEdit{end, end, "; " + cmd}
	}
	ind := indentAt(text, n.Range.Start) + unit
	ls := lineStart(text, closing)
	if blank(text[ls:closing]) {
		return TextEdit{ls, ls, indentLines(cmd, ind) + "\n"}
	}
	return TextEdit{closing, closing, "\n" + indentLines(cmd, ind) + "\n" + indentAt(text, n.Range.Start)}
}

// NodeText is the source of n as the Code section shows it: the lines after the first lose the
// indentation of the line n starts on.
func NodeText(text string, n *scene.Node) string {
	return dedent(text[n.Range.Start:n.Range.End], indentAt(text, n.Range.Start))
}

func dedent(src, ind string) string {
	if ind == "" {
		return src
	}
	lines := strings.Split(src, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.TrimPrefix(lines[i], ind)
	}
	return strings.Join(lines, "\n")
}

// reindent adds ind to the lines of s after the first; an empty one gets it only when more text follows.
func reindent(s, ind string, more bool) string {
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" || (more && i == len(lines)-1) {
			lines[i] = ind + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// ReplaceNodeEdit puts code (as NodeText shows it) in place of n.
func ReplaceNodeEdit(text string, n *scene.Node, code string) TextEdit {
	ind := indentAt(text, n.Range.Start)
	return TextEdit{n.Range.Start, n.Range.End, reindent(strings.TrimRight(code, "\n"), ind, false)}
}

// kidHead is a kid written without its block: `item "Name"`, `use path as name`. It is the line the
// Code section shows for the kid, and parses on its own.
func kidHead(text string, k *scene.Node) string {
	end := k.Range.End
	if k.Body != (scene.Range{}) {
		end = k.Body.Start
	}
	return strings.TrimRight(text[k.Range.Start:end], " \t\n")
}

// OwnText is NodeText with every kid the item tree lists (items, uses, slots) cut down to its head
// line, so the Code section holds what n itself writes, not the thousands of lines below it.
func OwnText(text string, n *scene.Node) string {
	var b strings.Builder
	at := n.Range.Start
	for _, k := range sceneKids(n) {
		if k.Range.Start < at || k.Range.End > n.Range.End {
			continue
		}
		b.WriteString(text[at:k.Range.Start])
		b.WriteString(kidHead(text, k))
		at = k.Range.End
	}
	b.WriteString(text[at:n.Range.End])
	return dedent(b.String(), indentAt(text, n.Range.Start))
}

// Stub is a kid command of the Code section text that stands for a kid of the node: a command without
// a block, of the kind and name of that kid. Start and End are bytes of the section text.
type Stub struct {
	Start, End int
	Kid        *scene.Node
}

// kidCmd reads a command as a kid of an owner: its kind and key; not ok when it is no kid, as a use
// not written `use <path> as <name>`.
func kidCmd(c *tenn.Node, owner scene.Kind) (scene.Kind, string, bool) {
	if c.Kind != tenn.KindCommand || len(c.Children) == 0 {
		return 0, "", false
	}
	name, _ := c.GetIdent(0)
	var args []string
	for _, a := range c.Children[1:] {
		if a.Kind != tenn.KindBlockExpr {
			s, _ := a.GetIdentText()
			args = append(args, s)
		}
	}
	first := ""
	if len(args) > 0 {
		first = args[0]
	}
	switch {
	case name == "item":
		return scene.KItem, scene.NormalizeName(first), true
	case name == "slot" && owner == scene.KItem:
		return scene.KSlot, scene.NormalizeName(first), true
	case name == "use" && len(args) == 3 && args[1] == "as":
		return scene.KUse, scene.NormalizeName(args[2]), true
	}
	return 0, "", false
}

// kidCmds are the kid commands written in the block of c, as go/scene reads them: a use passes the
// kids of its at, replace and slot overrides up to itself.
func kidCmds(c *tenn.Node, owner scene.Kind, out []*tenn.Node) []*tenn.Node {
	for _, a := range c.Children[1:] {
		if a.Kind != tenn.KindBlockExpr {
			continue
		}
		for _, k := range a.Children {
			if k.Token == nil {
				continue
			}
			if _, _, ok := kidCmd(k, owner); ok {
				out = append(out, k)
			} else if name, _ := k.GetIdent(0); owner == scene.KUse && (name == "at" || name == "replace" || name == "slot") {
				out = kidCmds(k, owner, out)
			}
		}
	}
	return out
}

func hasBlock(c *tenn.Node) bool { return c.Children[len(c.Children)-1].Kind == tenn.KindBlockExpr }

// findStubs pairs the stubs of the Code section text of n with its kids, and returns the command that
// is n there; nil when the text does not parse. Stubs of one name go in order. A kid left without a
// stub pairs with a command left without a kid when there are as many of one as of the other: a stub
// that was renamed. A command with a block is a kid typed in full and stays out.
func findStubs(n *scene.Node, code string) ([]Stub, *tenn.Node) {
	ast, p := tenn.Parse(code)
	if p.Errors.HasErrors() || len(ast.Children) == 0 || ast.Children[0].Token == nil {
		return nil, nil
	}
	root, kids := ast.Children[0], sceneKids(n)
	type name struct {
		kind scene.Kind
		key  string
	}
	byName := map[name][]int{}
	for i, k := range kids {
		byName[name{k.Kind, k.Key}] = append(byName[name{k.Kind, k.Key}], i)
	}
	used := make([]bool, len(kids))
	var stubs []Stub
	var left []*tenn.Node
	for _, c := range kidCmds(root, n.Kind, nil) {
		if hasBlock(c) {
			continue
		}
		kind, key, _ := kidCmd(c, n.Kind)
		q := byName[name{kind, key}]
		if len(q) == 0 {
			left = append(left, c)
			continue
		}
		used[q[0]] = true
		byName[name{kind, key}] = q[1:]
		stubs = append(stubs, Stub{c.Token.Pos, c.End, kids[q[0]]})
	}
	for _, kind := range []scene.Kind{scene.KItem, scene.KUse, scene.KSlot} {
		var cmds []*tenn.Node
		for _, c := range left {
			if k, _, _ := kidCmd(c, n.Kind); k == kind {
				cmds = append(cmds, c)
			}
		}
		var free []*scene.Node
		for i, k := range kids {
			if !used[i] && k.Kind == kind {
				free = append(free, k)
			}
		}
		if len(cmds) != len(free) {
			continue
		}
		for i, c := range cmds {
			stubs = append(stubs, Stub{c.Token.Pos, c.End, free[i]})
		}
	}
	slices.SortFunc(stubs, func(a, b Stub) int { return a.Start - b.Start })
	return stubs, root
}

// cmdNode is the command c as a node InsertEdit can add to.
func cmdNode(c *tenn.Node) *scene.Node {
	n := &scene.Node{Kind: scene.KItem, Range: scene.Range{Start: c.Token.Pos, End: c.End}}
	if last := c.Children[len(c.Children)-1]; len(c.Children) > 1 && last.Kind == tenn.KindBlockExpr && last.Token != nil {
		n.Body = scene.Range{Start: last.Token.Pos, End: c.End}
	}
	return n
}

// kidEdit replaces code[at:to] of the Code section text by text, which is final: not indented again.
type kidEdit struct {
	at, to int
	text   string
}

// ReplaceOwnEdit puts the Code section text of n (as OwnText shows it) in place of n. Every stub is
// replaced by the bytes of its kid; a kid whose stub was deleted stays where it was, after the kid
// before it; a command typed in full is a new kid. Deleting a kid is done in the item tree.
func ReplaceOwnEdit(text string, n *scene.Node, panel string) TextEdit {
	code := strings.TrimRight(panel, "\n")
	kids := sceneKids(n)
	stubs, root := findStubs(n, code)
	if root == nil || len(kids) == 0 {
		return ReplaceNodeEdit(text, n, panel)
	}
	if len(stubs) == 0 { // no stub left to hold places by: bring the heads back in the block first
		heads := make([]string, len(kids))
		for i, k := range kids {
			heads[i] = kidHead(text, k)
		}
		e := InsertEdit(code, cmdNode(root), strings.Join(heads, "\n"))
		code = code[:e.Start] + e.New + code[e.End:]
		if stubs, _ = findStubs(n, code); len(stubs) == 0 {
			return TextEdit{n.Range.Start, n.Range.Start, ""}
		}
	}
	ind := indentAt(text, n.Range.Start)
	raw := func(k *scene.Node) string { return text[k.Range.Start:k.Range.End] }
	// after puts kid k behind a stub: on a line of its own when the stub ends its line, else after a ;
	after := func(s Stub, k *scene.Node) kidEdit {
		sep := "; "
		if end := strings.IndexByte(code[s.End:], '\n'); end < 0 || blank(code[s.End:s.End+end]) {
			sep = "\n" + ind + indentAt(code, s.Start)
		}
		return kidEdit{s.End, s.End, sep + raw(k)}
	}
	before := func(s Stub, k *scene.Node) kidEdit {
		sep := "; "
		if blank(code[lineStart(code, s.Start):s.Start]) {
			sep = "\n" + ind + indentAt(code, s.Start)
		}
		return kidEdit{s.Start, s.Start, raw(k) + sep}
	}
	byKid := map[*scene.Node]Stub{}
	var edits []kidEdit
	for _, s := range stubs {
		byKid[s.Kid] = s
		edits = append(edits, kidEdit{s.Start, s.End, raw(s.Kid)})
	}
	var prev *Stub
	var lost []*scene.Node // kids before the first stub that is left
	for _, k := range kids {
		switch s, ok := byKid[k]; {
		case ok:
			for _, l := range lost {
				edits = append(edits, before(s, l))
			}
			lost, prev = nil, &s
		case prev != nil:
			edits = append(edits, after(*prev, k))
		default:
			lost = append(lost, k)
		}
	}
	slices.SortStableFunc(edits, func(a, b kidEdit) int {
		if a.at != b.at {
			return a.at - b.at
		}
		return a.to - b.to
	})
	var b strings.Builder
	at := 0
	for _, e := range edits {
		b.WriteString(reindent(code[at:e.at], ind, true))
		b.WriteString(e.text)
		at = e.to
	}
	b.WriteString(reindent(code[at:], ind, false))
	return TextEdit{n.Range.Start, n.Range.End, b.String()}
}

// FreeName is base, or base_2, base_3... when a kid of n already has that key.
func FreeName(n *scene.Node, base string) string {
	taken := map[string]bool{}
	for _, c := range n.Nodes() {
		taken[c.Key] = true
	}
	for _, c := range n.Content {
		taken[c.Key] = true
	}
	name := base
	for i := 2; taken[scene.NormalizeName(name)]; i++ {
		name = base + "_" + strconv.Itoa(i)
	}
	return name
}

// numText prints a coordinate without a trailing .0.
func numText(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
