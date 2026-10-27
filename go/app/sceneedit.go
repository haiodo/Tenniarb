package app

import (
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
// item or a use, a new block when n has none, the end of the file for a file root.
func InsertEdit(text string, n *scene.Node, cmd string) TextEdit {
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
	ind := indentAt(text, n.Range.Start)
	src := text[n.Range.Start:n.Range.End]
	if ind == "" {
		return src
	}
	lines := strings.Split(src, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.TrimPrefix(lines[i], ind)
	}
	return strings.Join(lines, "\n")
}

// ReplaceNodeEdit puts code (as NodeText shows it) in place of n.
func ReplaceNodeEdit(text string, n *scene.Node, code string) TextEdit {
	ind := indentAt(text, n.Range.Start)
	lines := strings.Split(strings.TrimRight(code, "\n"), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = ind + lines[i]
		}
	}
	return TextEdit{n.Range.Start, n.Range.End, strings.Join(lines, "\n")}
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
