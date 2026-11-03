// Package bundle inlines the files a document mounts with --from-file: one text the web renderer can
// draw, as it cannot read the neighbours.
package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Text is the text of a document the caller already holds with its mounts expanded; abs names the file
// the text belongs to and root is the workspace root the mounts resolve in.
func Text(root, abs, text string) (string, error) { return expand(root, abs, text, nil) }

// File is the text of abs, a file on disk, with its mounts expanded.
func File(root, abs string) (string, error) { return file(root, abs, nil) }

// bundleFile is the text of abs with the mounts of the file expanded. stack is the chain of files
// being expanded above it, for the message of a cycle.
// file is the text of abs, a file on disk, with its mounts expanded. stack is the chain of files
// being expanded above it, for the message of a cycle.
func file(root, abs string, stack []string) (string, error) {
	src, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return expand(root, abs, string(src), stack)
}

// expand is text with the mounts of abs expanded; stack is the chain of files above abs.
func expand(root, abs, text string, stack []string) (string, error) {
	ast, parser := tenn.Parse(text)
	if es := parser.Errors.Errors; len(es) > 0 {
		return "", fmt.Errorf("%s:%d:%d: %s", relPath(root, abs), es[0].Line+1, es[0].Col, es[0].Message)
	}
	edits, err := mounts(root, abs, text, ast.Children, append(stack, abs))
	if err != nil {
		return "", err
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	for i := len(edits) - 1; i >= 0; i-- { // from the last edit, so the offsets before it stay right
		e := edits[i]
		text = text[:e.start] + e.text + text[e.end:]
	}
	return text, nil
}

// edit is a span of the text replaced by other text.
type edit struct {
	start, end int
	text       string
}

// mounts is the edits of every element --from-file found in commands: the flag and its path
// become a block with the content of the file they mount.
func mounts(root, abs, text string, commands []*tenn.Node, stack []string) ([]edit, error) {
	var edits []edit
	for _, c := range commands {
		if c.Kind == tenn.KindCommand {
			name, _ := c.GetIdent(0)
			if name == "element" {
				if e, ok, err := mount(root, abs, text, c, stack); err != nil {
					return nil, err
				} else if ok {
					edits = append(edits, e)
					continue // a mounted element holds no block of its own
				}
			}
			if name == "template" {
				continue // the block is text the element generates, not commands the file mounts
			}
		}
		inner, err := mounts(root, abs, text, c.Children, stack)
		if err != nil {
			return nil, err
		}
		edits = append(edits, inner...)
	}
	return edits, nil
}

// mount is the edit of one element command with --from-file; ok is false for any other command.
func mount(root, abs, text string, c *tenn.Node, stack []string) (edit, bool, error) {
	args := c.Children[1:]
	hasBlock := c.Children[len(c.Children)-1].Kind == tenn.KindBlockExpr
	if hasBlock {
		args = args[:len(args)-1]
	}
	flagAt, ok := mountFlag(args)
	if !ok {
		return edit{}, false, nil
	}
	if hasBlock {
		name, _ := c.GetIdent(1)
		return edit{}, false, fmt.Errorf("%s: element %q takes its content from %s: remove the block", relPath(root, abs), name, argText(args[flagAt+1]))
	}
	rel := argText(args[flagAt+1])
	if rest := strings.TrimSpace(text[argEnd(args[flagAt+1]):c.End]); rest != "" {
		return edit{}, false, fmt.Errorf("%s: %q after the --from-file path is not supported", relPath(root, abs), rest)
	}
	target := filepath.Join(filepath.Dir(abs), filepath.FromSlash(rel))
	target, err := inside(root, target, rel)
	if err != nil {
		return edit{}, false, fmt.Errorf("%s: %v", relPath(root, abs), err)
	}
	for i, s := range stack {
		if s == target {
			var hops []string
			for k := i; k < len(stack); k++ {
				next := target
				if k+1 < len(stack) {
					next = stack[k+1]
				}
				hops = append(hops, fmt.Sprintf("%s imports %s", relPath(root, stack[k]), relPath(root, next)))
			}
			return edit{}, false, fmt.Errorf("import cycle: %s", strings.Join(hops, ", "))
		}
	}
	content, err := file(root, target, stack)
	if err != nil {
		return edit{}, false, err
	}
	line := strings.LastIndexByte(text[:c.Token.Pos], '\n') + 1
	indent := text[line:c.Token.Pos]
	indent = indent[:len(indent)-len(strings.TrimLeft(indent, " \t"))]
	body := indentText(content, indent+"  ")
	if body == "" {
		return edit{start: argStart(args[flagAt]), end: argEnd(args[flagAt+1]), text: "{}"}, true, nil
	}
	return edit{
		start: argStart(args[flagAt]),
		end:   argEnd(args[flagAt+1]),
		text:  "{\n" + body + "\n" + indent + "}",
	}, true, nil
}

// mountFlag is the index of the --from-file argument with its path after it; ok is false when the
// command has none.
func mountFlag(args []*tenn.Node) (int, bool) {
	for i, a := range args {
		if s, _ := a.GetIdentText(); s == "--from-file" && i+1 < len(args) {
			return i, true
		}
	}
	return 0, false
}

// argStart and argEnd are the byte span of an argument in the text, the quotes of a string included.
func argStart(a *tenn.Node) int {
	if a.Token == nil {
		return 0
	}
	pos := a.Token.Pos
	if a.Token.Type == tenn.TokStringLit {
		pos--
	}
	if pos < 0 {
		return 0
	}
	return pos
}

func argEnd(a *tenn.Node) int {
	if a.Token == nil {
		return 0
	}
	return a.Token.EndPos()
}

func argText(a *tenn.Node) string {
	s, _ := a.GetIdentText()
	return s
}

// relPath is the slash path of abs relative to the root, for the messages.
func relPath(root, abs string) string {
	r, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(r)
}

// inside is abs with its symlinks resolved, or the error that it leaves the workspace root.
func inside(root, abs, rel string) (string, error) {
	if r, err := filepath.EvalSymlinks(root); err == nil { // /var on macOS is a symlink to /private/var
		root = r
	}
	p := filepath.Clean(abs)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	up, err := filepath.Rel(root, p)
	if err != nil || up == ".." || strings.HasPrefix(up, ".."+string(filepath.Separator)) {
		return p, fmt.Errorf("--from-file %q leaves the workspace root %s", rel, root)
	}
	return p, nil
}

// indentText is the text of a mounted file ready to sit in a block at indent: its surrounding blank
// lines and the indentation its lines share are cut, and the lines take the indentation. The bytes of
// a multi-line literal and of a template are the value itself, so those lines stay as written.
func indentText(src, indent string) string {
	ast, _ := tenn.Parse(src)
	locked := locked(ast.Children, nil)
	lines := strings.Split(src, "\n")
	starts := make([]int, len(lines))
	for i := 1; i < len(lines); i++ {
		starts[i] = starts[i-1] + len(lines[i-1]) + 1
	}
	lo, hi := 0, len(lines)
	for lo < hi && strings.TrimSpace(lines[lo]) == "" {
		lo++
	}
	for hi > lo && strings.TrimSpace(lines[hi-1]) == "" {
		hi--
	}
	cut := -1
	for i := lo; i < hi; i++ {
		if strings.TrimSpace(lines[i]) == "" || lockedAt(locked, starts[i]) {
			continue
		}
		if n := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t")); cut < 0 || n < cut {
			cut = n
		}
	}
	if cut < 0 {
		cut = 0
	}
	var b strings.Builder
	for i := lo; i < hi; i++ {
		if i > lo {
			b.WriteString("\n")
		}
		switch {
		case lockedAt(locked, starts[i]):
			b.WriteString(lines[i])
		case strings.TrimSpace(lines[i]) == "":
		default:
			b.WriteString(indent)
			b.WriteString(lines[i][cut:])
		}
	}
	return b.String()
}

// span is a byte range whose text is a value, not layout.
type span struct{ start, end int }

// locked is the spans of the multi-line literals and of the templates in commands.
func locked(commands []*tenn.Node, spans []span) []span {
	for _, c := range commands {
		if t := c.Token; t != nil {
			switch t.Type {
			case tenn.TokMarkdownLit, tenn.TokExpression, tenn.TokExpressionBlock, tenn.TokImageData:
				if strings.Contains(t.Literal, "\n") {
					spans = append(spans, span{t.Pos, t.EndPos()})
				}
			}
		}
		if c.Kind == tenn.KindCommand {
			if name, _ := c.GetIdent(0); name == "template" {
				if b := c.Children[len(c.Children)-1]; b.Kind == tenn.KindBlockExpr {
					spans = append(spans, span{b.Token.Pos, c.End})
				}
			}
		}
		spans = locked(c.Children, spans)
	}
	return spans
}

// lockedAt reports whether off starts inside one of the spans.
func lockedAt(spans []span, off int) bool {
	for _, s := range spans {
		if s.start < off && off < s.end {
			return true
		}
	}
	return false
}
