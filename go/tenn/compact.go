package tenn

import (
	"bytes"
	"strings"
	"unicode"
)

// CompactLimit is the widest line (bytes) a one-line block may reach; chosen by size
// measurement on imported scene files, see docs/spec/tenn-scene.md section 9.
const CompactLimit = 160

// CompactStyle tunes the compact form. The zero value is Compact.
type CompactStyle struct {
	Indent string // one level; two spaces when empty
	Limit  int    // widest one-line block; CompactLimit when 0
	// Pack puts a run of commands without a block on one line, joined by "; ", inside a block that
	// does not fit on one line. The run ends where the line would pass Limit.
	Pack bool
}

// Compact prints a tree in the compact canonical form: 2 spaces per level, a block of plain
// commands on one line (`item "A" { title x; color red }`), quotes dropped where a bare word reads
// back as the same text. ToStr stays the byte-exact Swift printer; both parse in the Swift app.
func Compact(node *Node) string {
	return CompactAs(node, CompactStyle{})
}

// CompactWidth is Compact with an explicit line limit.
func CompactWidth(node *Node, limit int) string {
	return CompactAs(node, CompactStyle{Limit: limit})
}

// CompactAs is Compact with the given style.
func CompactAs(node *Node, st CompactStyle) string {
	if st.Indent == "" {
		st.Indent = "  "
	}
	if st.Limit == 0 {
		st.Limit = CompactLimit
	}
	var buf bytes.Buffer
	if node.Kind == KindStatements || node.Kind == KindBlockExpr {
		for i, c := range node.Children {
			if i > 0 {
				buf.WriteByte('\n')
			}
			st.cmd(&buf, c, 0)
		}
	} else {
		st.cmd(&buf, node, 0)
	}
	return buf.String()
}

func (st CompactStyle) cmd(buf *bytes.Buffer, cmd *Node, ind int) {
	buf.WriteString(strings.Repeat(st.Indent, ind))
	if cmd.Kind != KindCommand {
		buf.WriteString(compactAtom(cmd))
		return
	}
	for i, c := range cmd.Children {
		if i > 0 {
			buf.WriteByte(' ')
		}
		if c.Kind != KindBlockExpr {
			buf.WriteString(compactAtom(c))
			continue
		}
		line := buf.Bytes()[bytes.LastIndexByte(buf.Bytes(), '\n')+1:]
		if one, ok := oneLineBlock(c); ok && len(line)+len(one) <= st.Limit {
			buf.WriteString(one)
			continue
		}
		buf.WriteString("{\n")
		st.block(buf, c.Children, ind+1)
		buf.WriteString("\n" + strings.Repeat(st.Indent, ind) + "}")
	}
}

// block prints the commands of a multi-line block, one per line, or packed.
func (st CompactStyle) block(buf *bytes.Buffer, cmds []*Node, ind int) {
	for j := 0; j < len(cmds); {
		if j > 0 {
			buf.WriteByte('\n')
		}
		if run := st.packRun(cmds[j:], ind); len(run) > 1 {
			buf.WriteString(strings.Repeat(st.Indent, ind) + strings.Join(run, "; "))
			j += len(run)
			continue
		}
		st.cmd(buf, cmds[j], ind)
		j++
	}
}

// packRun is the text of the commands from the start of cmds that share a line; none when Pack is off.
func (st CompactStyle) packRun(cmds []*Node, ind int) []string {
	if !st.Pack {
		return nil
	}
	width := ind * len(st.Indent)
	var run []string
	for _, c := range cmds {
		text, ok := plainCommand(c)
		if !ok {
			break
		}
		if len(run) > 0 {
			if width+2+len(text) > st.Limit {
				break
			}
			width += 2
		}
		width += len(text)
		run = append(run, text)
	}
	return run
}

// plainCommand renders a command without a block or a line break.
func plainCommand(c *Node) (string, bool) {
	if c.Kind != KindCommand {
		return "", false
	}
	parts := make([]string, 0, len(c.Children))
	for _, a := range c.Children {
		if a.Kind == KindBlockExpr {
			return "", false
		}
		parts = append(parts, compactAtom(a))
	}
	text := strings.Join(parts, " ")
	return text, !strings.Contains(text, "\n")
}

// oneLineBlock renders a block whose commands hold no blocks and no line breaks.
func oneLineBlock(b *Node) (string, bool) {
	if len(b.Children) == 0 {
		return "{}", true
	}
	parts := make([]string, 0, len(b.Children))
	for _, c := range b.Children {
		if c.Kind != KindCommand {
			return "", false
		}
		var sb strings.Builder
		for i, a := range c.Children {
			if a.Kind == KindBlockExpr {
				return "", false
			}
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(compactAtom(a))
		}
		if strings.Contains(sb.String(), "\n") {
			return "", false
		}
		parts = append(parts, sb.String())
	}
	return "{ " + strings.Join(parts, "; ") + " }", true
}

func compactAtom(n *Node) string {
	if n.Kind == KindStringLit && n.Token != nil && bareSafe(n.Token.Literal) {
		return n.Token.Literal
	}
	return ToStr(n, 0, false)
}

// bareSafe: the word lexes as one symbol token, never a number, so the parser gives an Ident
// that reads as the same text (GetIdentText).
func bareSafe(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_-.#", r) {
			return false
		}
	}
	return DetectSymbolType(s) == TokSymbol
}
