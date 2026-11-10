package app

import (
	"math"
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

// shadowsOf lists the shadow commands written on n itself, in file order.
func shadowsOf(n *scene.Node) []*scene.Prop {
	var out []*scene.Prop
	if n.Kind == scene.KUse {
		for _, ov := range n.Block {
			if ov.Kind == scene.OvProp && ov.Prop.Name == "shadow" {
				out = append(out, ov.Prop)
			}
		}
		return out
	}
	for _, p := range n.Props {
		if p.Name == "shadow" {
			out = append(out, p)
		}
	}
	return out
}

// SetShadowsEdit replaces every shadow of n with cmds: a soft shadow is more than one command, and the
// node may hold one already. The edits address one text, so apply them from the last to the first.
func SetShadowsEdit(text string, n *scene.Node, cmds ...string) []TextEdit {
	cur := shadowsOf(n)
	var out []TextEdit
	switch {
	case len(cmds) == 0:
		for _, p := range cur {
			out = append(out, removeEdit(text, p.Range))
		}
	case len(cur) == 0:
		out = append(out, InsertEdit(text, n, strings.Join(cmds, "\n")))
	default:
		// The first command takes the place of the first shadow, so the block never ends up without one.
		ind := indentAt(text, cur[0].Range.Start)
		repl := cmds[0]
		if len(cmds) > 1 {
			repl += "\n" + indentLines(strings.Join(cmds[1:], "\n"), ind)
		}
		out = append(out, TextEdit{cur[0].Range.Start, cur[0].Range.End, repl})
		for _, p := range cur[1:] {
			out = append(out, removeEdit(text, p.Range))
		}
	}
	slices.SortFunc(out, func(a, b TextEdit) int { return b.Start - a.Start })
	return out
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

// ParamEdit writes the values of parameter prm into the block of the use that sets it: in place of the
// command of that name, else as a new command. An empty list writes the bare name, which names no style.
func ParamEdit(text string, use *scene.Node, prm scene.Param, values ...string) (TextEdit, bool) {
	line := prm.Name
	for _, v := range values {
		line += " " + paramValue(prm, v)
	}
	if own := ownProp(use, prm.Name); own != nil {
		return TextEdit{own.Range.Start, own.Range.End, line}, true
	}
	return InsertEdit(text, use, line), true
}

// RemoveParamEdit deletes the value parameter prm of the use sets, so the default of the declaration comes
// back.
func RemoveParamEdit(text string, use *scene.Node, prm scene.Param) (TextEdit, bool) {
	own := ownProp(use, prm.Name)
	if own == nil {
		return TextEdit{}, false
	}
	return removeEdit(text, own.Range), true
}

// paramValue is one value of parameter prm as the file writes it (spec 5c): a string in quotes, a number
// or a word bare, as the default of the declaration is written.
func paramValue(prm scene.Param, v string) string {
	kind := tenn.KindIdent
	if prm.Default != nil && len(prm.Default.Args) > 0 {
		kind = prm.Default.Args[0].Kind
	}
	switch kind {
	case tenn.KindStringLit:
		return tenn.ToStr(tenn.NewStrNode(v), 0, false)
	case tenn.KindIntLit, tenn.KindFloatLit:
		if isNumber(v) {
			return v
		}
	case tenn.KindIdent, tenn.KindCharLit:
		if plainWord(v) {
			return v
		}
	}
	// no default of a shape, or a value that is not one of it: a number stays a number, else it is a string
	if isNumber(v) {
		return v
	}
	return tenn.ToStr(tenn.NewStrNode(v), 0, false)
}

// isNumber reports a value written as a number is read back the same, so "007" stays text.
func isNumber(v string) bool {
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && strconv.FormatFloat(f, 'f', -1, 64) == v
}

// plainWord reports a value that a command can hold as one bare word.
func plainWord(v string) bool {
	if v == "" || strings.ContainsAny(v, " \t\r\n\"';{}") {
		return false
	}
	return !isNumber(v)
}

// EffectiveText is the item drawn as pl with the properties in effect, the overrides applied; its kids are
// not listed.
func EffectiveText(pl *scene.Placed) string {
	head := CommandText("item", pl.Src.Name)
	if pl.Src.Kind == scene.KUse {
		head = CommandText("use", pl.Src.Path, "as", pl.Src.Name)
	}
	lines := []string{head + " {"}
	for _, p := range pl.Props() {
		lines = append(lines, "  "+CommandText(p.Name, p.Values()...))
	}
	return strings.Join(append(lines, "}"), "\n")
}

// atOf finds the at override of path directly in the block of use; paths are compared by their normalized
// segments, the last one written wins.
func atOf(use *scene.Node, path string) *scene.Override {
	var found *scene.Override
	for _, ov := range use.Block {
		if ov.Kind == scene.OvAt && samePath(ov.Path, path) {
			found = ov
		}
	}
	return found
}

func samePath(a, b string) bool {
	sa, sb := strings.Split(a, "."), strings.Split(b, ".")
	return slices.EqualFunc(sa, sb, func(x, y string) bool { return scene.NormalizeName(x) == scene.NormalizeName(y) })
}

// atProp is the command of name in an at override; the last one wins.
func atProp(at *scene.Override, name string) *scene.Prop {
	var found *scene.Prop
	for _, ov := range at.Block {
		if ov.Kind == scene.OvProp && ov.Prop.Name == name {
			found = ov.Prop
		}
	}
	return found
}

// AtPropEdit writes `name args` for the part of the instance use that path names (names from the root of
// the instance, spec 4.7): in place of its command in the at override of that path, else as a new command
// in that at, else as a new at in the block of use. No args removes the command.
func AtPropEdit(text string, use *scene.Node, path, name string, args ...string) (TextEdit, bool) {
	at := atOf(use, path)
	var own *scene.Prop
	if at != nil {
		own = atProp(at, name)
	}
	switch {
	case len(args) == 0 && own == nil:
		return TextEdit{}, false
	case len(args) == 0:
		return removeEdit(text, own.Range), true
	case own != nil:
		return TextEdit{own.Range.Start, own.Range.End, CommandText(name, args...)}, true
	case at != nil:
		return InsertEdit(text, &scene.Node{Kind: scene.KItem, Range: at.Range, Body: at.Body}, CommandText(name, args...)), true
	}
	return InsertEdit(text, use, CommandText("at", path)+" { "+CommandText(name, args...)+" }"), true
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

// PrependEdit adds cmd as the first command of the content of n: the block of an element, an item or
// a use, a new block when n has none, the start of the file for a file root.
func PrependEdit(text string, n *scene.Node, cmd string) TextEdit {
	if n.Mount != nil { // the content of an element with --from-file is written in that file
		n = n.Mount
	}
	unit := indentUnit(text)
	if n.Kind == scene.KFile {
		return TextEdit{0, 0, cmd + "\n"}
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
		return TextEdit{open + 1, open + 1, " " + cmd + ";"}
	}
	// the command goes before the first line of the content, with that line's indentation
	first := open + 1
	for first < closing && strings.ContainsRune(" \t\r\n", rune(text[first])) {
		first++
	}
	if first >= closing {
		return TextEdit{open + 1, open + 1, " " + cmd + " "}
	}
	ls := lineStart(text, first)
	if ls <= open { // the content starts on the line of the opening brace
		ind := indentAt(text, n.Range.Start) + unit
		return TextEdit{open + 1, open + 1, "\n" + indentLines(cmd, ind) + "\n" + indentAt(text, n.Range.Start) + " "}
	}
	return TextEdit{ls, ls, indentLines(cmd, indentAt(text, first)) + "\n"}
}

// flowLayout reports whether n lays its kids out in a flow: `auto-layout row` or `column`, also
// written bare. An item without auto-layout, or with `auto-layout abs`, places them by pos.
func flowLayout(n *scene.Node) bool {
	p := ownProp(n, "auto-layout")
	if p == nil {
		return false
	}
	vals := p.Values()
	return len(vals) == 0 || vals[0] != "abs"
}

// movedText is the source of n as a move writes it: into a flow layout its pos goes, so the node does
// not jump; an absolute parent keeps it.
func movedText(text string, n, into *scene.Node) string {
	raw := text[n.Range.Start:n.Range.End]
	if !flowLayout(into) {
		return raw
	}
	own := ownProp(n, "pos")
	if own == nil {
		return raw
	}
	e := removeEdit(text, own.Range)
	if e.Start < n.Range.Start || e.End > n.Range.End {
		return raw
	}
	return text[n.Range.Start:e.Start] + e.New + text[e.End:n.Range.End]
}

// MoveNodeEdit swaps n with the sibling delta places away among the kids of its parent, as one edit;
// at is where n starts in the new text. A node of a flow layout loses its pos.
func MoveNodeEdit(text string, n *scene.Node, delta int) (e TextEdit, at int, ok bool) {
	if n == nil || n.Parent == nil {
		return TextEdit{}, 0, false
	}
	sibs := sceneKids(n.Parent)
	i := slices.Index(sibs, n)
	j := i + delta
	if i < 0 || j < 0 || j >= len(sibs) {
		return TextEdit{}, 0, false
	}
	a, b := n, sibs[j]
	if a.Range.Start > b.Range.Start {
		a, b = b, a
	}
	if b.Range.Start < a.Range.End {
		return TextEdit{}, 0, false
	}
	left, gap, right := text[a.Range.Start:a.Range.End], text[a.Range.End:b.Range.Start], text[b.Range.Start:b.Range.End]
	moved := movedText(text, n, n.Parent)
	if n == a {
		return TextEdit{a.Range.Start, b.Range.End, right + gap + moved}, a.Range.Start + len(right) + len(gap), true
	}
	return TextEdit{a.Range.Start, b.Range.End, moved + gap + left}, a.Range.Start, true
}

// MoveNodeIntoEdit moves n into the sibling delta places away, as its last kid (delta < 0) or its
// first (delta > 0), as one edit; at is where n starts in the new text.
func MoveNodeIntoEdit(text string, n *scene.Node, delta int) (e TextEdit, at int, ok bool) {
	if n == nil || n.Parent == nil || delta == 0 {
		return TextEdit{}, 0, false
	}
	sibs := sceneKids(n.Parent)
	i := slices.Index(sibs, n)
	j := i + delta
	if i < 0 || j < 0 || j >= len(sibs) {
		return TextEdit{}, 0, false
	}
	into := sibs[j]
	moved := movedText(text, n, into)
	ins := InsertEdit(text, into, moved)
	if delta > 0 {
		ins = PrependEdit(text, into, moved)
	}
	rem := removeEdit(text, n.Range)
	if rem.Start < ins.Start && ins.Start < rem.End {
		return TextEdit{}, 0, false
	}
	idx := strings.Index(ins.New, moved)
	if idx < 0 {
		return TextEdit{}, 0, false
	}
	lo, hi := ins, rem
	if rem.Start < ins.Start {
		lo, hi = rem, ins
	}
	if hi.Start < lo.End {
		return TextEdit{}, 0, false
	}
	at = ins.Start + idx
	if delta > 0 { // the removal comes first: what it took shifts the inserted node
		at = lo.Start + len(lo.New) + (hi.Start - lo.End) + idx
	}
	return TextEdit{lo.Start, hi.End, lo.New + text[lo.End:hi.Start] + hi.New}, at, true
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
// line, so the Code section holds what n itself writes, not the thousands of lines below it. A kid
// without kids of its own is written in full: there is no subtree to hide behind a link.
func OwnText(text string, n *scene.Node) string {
	if n == nil || text == "" || n.Range.End > len(text) {
		return "" // a node of a file whose text is not here: better empty than a slice of another file
	}
	var b strings.Builder
	at := n.Range.Start
	for _, k := range sceneKids(n) {
		if k.Range.Start < at || k.Range.End > n.Range.End {
			continue
		}
		b.WriteString(text[at:k.Range.Start])
		if len(sceneKids(k)) == 0 {
			b.WriteString(text[k.Range.Start:k.Range.End])
		} else {
			b.WriteString(kidHead(text, k))
		}
		at = k.Range.End
	}
	b.WriteString(text[at:n.Range.End])
	return hideImageData(dedent(b.String(), indentAt(text, n.Range.Start)))
}

// Stub is a kid command of the Code section text that stands for a kid of the node: a command of the
// kind and name of that kid. Start and End are bytes of the section text. Full is set for a kid the
// text writes out itself: it hides nothing, so it is no link and the panel keeps its text as typed.
type Stub struct {
	Start, End int
	Kid        *scene.Node
	Full       bool
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
		kind, key, _ := kidCmd(c, n.Kind)
		q := byName[name{kind, key}]
		if len(q) == 0 {
			left = append(left, c)
			continue
		}
		k := kids[q[0]]
		used[q[0]] = true
		byName[name{kind, key}] = q[1:]
		stubs = append(stubs, Stub{c.Token.Pos, c.End, k, hasBlock(c) || k.Body == (scene.Range{})})
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
			stubs = append(stubs, Stub{c.Token.Pos, c.End, free[i], hasBlock(c)})
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
	if n.Range.Start >= 0 && n.Range.Start <= n.Range.End && n.Range.End <= len(text) {
		// The pane shows a stub in place of the data of a classic image: the document gets it whole.
		code = restoreImageData(code, text[n.Range.Start:n.Range.End])
	}
	kids := sceneKids(n)
	stubs, root := findStubs(n, code)
	if root == nil || len(kids) == 0 {
		return ReplaceNodeEdit(text, n, code)
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
		if !s.Full {
			edits = append(edits, kidEdit{s.Start, s.End, raw(s.Kid)})
		}
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

// numText prints a coordinate to 1/100 of a point without a trailing .0: a drag gives float noise
// that makes the file unreadable and says nothing a pixel can show.
func numText(f float64) string { return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64) }
