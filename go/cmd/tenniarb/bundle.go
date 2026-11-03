package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

const bundleUsage = "usage: tenniarb bundle [-root DIR] [-o OUT] FILE"

// bundle writes FILE with every "element <name> --from-file <path>" replaced by the block that holds
// the content of the mounted file, mounts of mounts included: one file the web renderer can draw,
// as it cannot read the neighbours. The text of FILE stays as written, comments and blank lines kept;
// the text of a mounted file moves to the indentation of its place.
func bundle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("root", "", "workspace root for --from-file (default: the working directory, or the directory of a file outside it)")
	out := fs.String("o", "", "write to OUT instead of stdout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, bundleUsage)
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 2
	}
	abs, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 2
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 2
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil { // the cycle and root checks compare real paths
		abs = r
	}
	root, err := filepath.EvalSymlinks(genRoot(*rootFlag, cwd, abs))
	if err != nil {
		root = genRoot(*rootFlag, cwd, abs)
	}
	text, err := bundleFile(root, abs, nil)
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 1
	}
	if *out == "" {
		io.WriteString(stdout, text)
		return 0
	}
	if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 1
	}
	return 0
}

// bundleFile is the text of abs with the mounts of the file expanded. stack is the chain of files
// being expanded above it, for the message of a cycle.
func bundleFile(root, abs string, stack []string) (string, error) {
	src, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(src)
	ast, parser := tenn.Parse(text)
	if es := parser.Errors.Errors; len(es) > 0 {
		return "", fmt.Errorf("%s:%d:%d: %s", bundleRel(root, abs), es[0].Line+1, es[0].Col, es[0].Message)
	}
	edits, err := bundleMounts(root, abs, text, ast.Children, append(stack, abs))
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

// bundleMounts is the edits of every element --from-file found in commands: the flag and its path
// become a block with the content of the file they mount.
func bundleMounts(root, abs, text string, commands []*tenn.Node, stack []string) ([]edit, error) {
	var edits []edit
	for _, c := range commands {
		if c.Kind == tenn.KindCommand {
			name, _ := c.GetIdent(0)
			if name == "element" {
				if e, ok, err := bundleMount(root, abs, text, c, stack); err != nil {
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
		inner, err := bundleMounts(root, abs, text, c.Children, stack)
		if err != nil {
			return nil, err
		}
		edits = append(edits, inner...)
	}
	return edits, nil
}

// bundleMount is the edit of one element command with --from-file; ok is false for any other command.
func bundleMount(root, abs, text string, c *tenn.Node, stack []string) (edit, bool, error) {
	args := c.Children[1:]
	hasBlock := c.Children[len(c.Children)-1].Kind == tenn.KindBlockExpr
	if hasBlock {
		args = args[:len(args)-1]
	}
	flagAt, ok := bundleFlag(args)
	if !ok {
		return edit{}, false, nil
	}
	if hasBlock {
		name, _ := c.GetIdent(1)
		return edit{}, false, fmt.Errorf("%s: element %q takes its content from %s: remove the block", bundleRel(root, abs), name, bundleArgText(args[flagAt+1]))
	}
	rel := bundleArgText(args[flagAt+1])
	if rest := strings.TrimSpace(text[argEnd(args[flagAt+1]):c.End]); rest != "" {
		return edit{}, false, fmt.Errorf("%s: %q after the --from-file path is not supported", bundleRel(root, abs), rest)
	}
	target := filepath.Join(filepath.Dir(abs), filepath.FromSlash(rel))
	target, err := bundleInside(root, target, rel)
	if err != nil {
		return edit{}, false, fmt.Errorf("%s: %v", bundleRel(root, abs), err)
	}
	for i, s := range stack {
		if s == target {
			var hops []string
			for k := i; k < len(stack); k++ {
				next := target
				if k+1 < len(stack) {
					next = stack[k+1]
				}
				hops = append(hops, fmt.Sprintf("%s imports %s", bundleRel(root, stack[k]), bundleRel(root, next)))
			}
			return edit{}, false, fmt.Errorf("import cycle: %s", strings.Join(hops, ", "))
		}
	}
	content, err := bundleFile(root, target, stack)
	if err != nil {
		return edit{}, false, err
	}
	line := strings.LastIndexByte(text[:c.Token.Pos], '\n') + 1
	indent := text[line:c.Token.Pos]
	indent = indent[:len(indent)-len(strings.TrimLeft(indent, " \t"))]
	body := bundleIndent(content, indent+"  ")
	if body == "" {
		return edit{start: argStart(args[flagAt]), end: argEnd(args[flagAt+1]), text: "{}"}, true, nil
	}
	return edit{
		start: argStart(args[flagAt]),
		end:   argEnd(args[flagAt+1]),
		text:  "{\n" + body + "\n" + indent + "}",
	}, true, nil
}

// bundleFlag is the index of the --from-file argument with its path after it; ok is false when the
// command has none.
func bundleFlag(args []*tenn.Node) (int, bool) {
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

func bundleArgText(a *tenn.Node) string {
	s, _ := a.GetIdentText()
	return s
}

// bundleRel is the slash path of abs relative to the root, for the messages.
func bundleRel(root, abs string) string {
	r, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(r)
}

// bundleInside is abs with its symlinks resolved, or the error that it leaves the workspace root.
func bundleInside(root, abs, rel string) (string, error) {
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

// bundleIndent is the text of a mounted file ready to sit in a block at indent: its surrounding blank
// lines and the indentation its lines share are cut, and the lines take the indentation. The bytes of
// a multi-line literal and of a template are the value itself, so those lines stay as written.
func bundleIndent(src, indent string) string {
	ast, _ := tenn.Parse(src)
	locked := bundleLocked(ast.Children, nil)
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
		if strings.TrimSpace(lines[i]) == "" || bundleLockedAt(locked, starts[i]) {
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
		case bundleLockedAt(locked, starts[i]):
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

// bundleLocked is the spans of the multi-line literals and of the templates in commands.
func bundleLocked(commands []*tenn.Node, spans []span) []span {
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
		spans = bundleLocked(c.Children, spans)
	}
	return spans
}

// bundleLockedAt reports whether off starts inside one of the spans.
func bundleLockedAt(spans []span, off int) bool {
	for _, s := range spans {
		if s.start < off && off < s.end {
			return true
		}
	}
	return false
}
