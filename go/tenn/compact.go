package tenn

import (
	"bytes"
	"strings"
	"unicode"
)

// CompactLimit is the widest line (bytes) a one-line block may reach; chosen by size
// measurement on imported scene files, see docs/spec/tenn-scene.md section 9.
const CompactLimit = 160

// Compact prints a tree in the compact canonical form: 2 spaces per level, a block of plain
// commands on one line (`item "A" { title x; color red }`), quotes dropped where a bare word reads
// back as the same text. ToStr stays the byte-exact Swift printer; both parse in the Swift app.
func Compact(node *Node) string {
	return CompactWidth(node, CompactLimit)
}

// CompactWidth is Compact with an explicit line limit.
func CompactWidth(node *Node, limit int) string {
	var buf bytes.Buffer
	if node.Kind == KindStatements || node.Kind == KindBlockExpr {
		for i, c := range node.Children {
			if i > 0 {
				buf.WriteByte('\n')
			}
			compactCmd(&buf, c, 0, limit)
		}
	} else {
		compactCmd(&buf, node, 0, limit)
	}
	return buf.String()
}

func compactCmd(buf *bytes.Buffer, cmd *Node, ind, limit int) {
	buf.WriteString(strings.Repeat("  ", ind))
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
		if one, ok := oneLineBlock(c); ok && len(line)+len(one) <= limit {
			buf.WriteString(one)
			continue
		}
		buf.WriteString("{\n")
		for j, cc := range c.Children {
			if j > 0 {
				buf.WriteByte('\n')
			}
			compactCmd(buf, cc, ind+1, limit)
		}
		buf.WriteString("\n" + strings.Repeat("  ", ind) + "}")
	}
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
