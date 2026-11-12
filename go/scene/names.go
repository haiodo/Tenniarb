package scene

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/haiodo/tenn/go"
)

// NormalizeName is convertNameJS of web/packages/core/src/execution.ts: a rune that is not a letter,
// mark or number becomes "_". An empty name stays empty.
func NormalizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Segment is one step of a dotted path (spec 4.2). A bare segment names a node by its key, the
// normalized name; a segment in double quotes names a node by its Name as written, punctuation and all.
type Segment struct {
	Name   string // a quoted segment: the text between the quotes, \" undone; a bare one: the text
	Quoted bool
}

// Key is the segment as a node key: its name normalized.
func (s Segment) Key() string { return NormalizeName(s.Name) }

// Text is the segment as a path writes it.
func (s Segment) Text() string {
	if s.Quoted {
		return QuoteName(s.Name)
	}
	return s.Name
}

// Matches reports whether the segment names n (spec 4.2): a quoted one compares the name as written,
// a bare one the key.
func (s Segment) Matches(n *Node) bool {
	if s.Quoted {
		return norm.NFC.String(s.Name) == norm.NFC.String(n.Name)
	}
	return n.Key == s.Key()
}

// IsName reports whether the segment is a way to write name: a quoted one when it says the name as
// written, a bare one when it has the key.
func (s Segment) IsName(name string) bool {
	if s.Quoted {
		return norm.NFC.String(s.Name) == norm.NFC.String(name)
	}
	return s.Key() == NormalizeName(name)
}

// QuoteName writes name as a path segment in double quotes; a quote inside is written \", as the lexer
// reads it. A backslash is literal, like in a string of the format.
func QuoteName(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
}

// SegText is the segment a path writes for a node named name: as it is when the name is already a key,
// in quotes otherwise, so a path get_state prints can be copied into at and read (spec 4.2).
func SegText(name string) string {
	switch {
	case name == "":
		return "" // an unnamed node has no path segment
	case NormalizeName(name) == name:
		return name
	}
	return QuoteName(name)
}

// splitPath splits a dotted path into segments; ok is false for an empty segment, an unterminated quote,
// or text after a closing quote that is not a dot.
func splitPath(path string) (segs []Segment, ok bool) {
	for i := 0; i < len(path); {
		if path[i] == '"' {
			var b strings.Builder
			j, closed := i+1, false
			for j < len(path) {
				switch {
				case path[j] == '\\' && j+1 < len(path) && path[j+1] == '"':
					b.WriteByte('"')
					j += 2
				case path[j] == '"':
					closed, j = true, j+1
				default:
					b.WriteByte(path[j])
					j++
				}
				if closed {
					break
				}
			}
			if !closed || j < len(path) && path[j] != '.' {
				return nil, false
			}
			segs = append(segs, Segment{Name: b.String(), Quoted: true})
			if i = j; i == len(path) {
				break
			}
			i++ // the dot
			if i == len(path) {
				return nil, false
			}
			continue
		}
		j := strings.IndexByte(path[i:], '.')
		if j < 0 {
			j = len(path)
		} else {
			j += i
		}
		if j == i {
			return nil, false // an empty segment: two dots or a leading one
		}
		segs = append(segs, Segment{Name: path[i:j]})
		if i = j; i == len(path) {
			break
		}
		i++ // the dot
		if i == len(path) {
			return nil, false
		}
	}
	if len(segs) == 0 {
		return nil, false
	}
	return segs, true
}

// SplitPath is splitPath for a caller outside the package: the segments of a dotted path (spec 4.2),
// with a name in double quotes read as written; ok is false for a path the format cannot read.
func SplitPath(path string) ([]Segment, bool) { return splitPath(path) }

// pathText is the path a command writes, from the argument tokens after it: bare words and names in
// double quotes, joined by dots. ok is false when the arguments do not form one path.
func pathText(args []*tenn.Node) (string, bool) {
	text, n, ok := pathPrefix(args)
	return text, ok && n == len(args)
}

// pathPrefix is the longest path the first argument tokens form, and how many tokens they are. at and
// replace take it, so a command the format joined with a second block keeps its first path.
func pathPrefix(args []*tenn.Node) (text string, n int, ok bool) {
	var b strings.Builder
	best := ""
	need := true // a segment may start here
	for i, a := range args {
		s, has := a.GetIdentText()
		if !has {
			break
		}
		if a.Kind == tenn.KindStringLit {
			if !need {
				break
			}
			b.WriteString(QuoteName(s))
			need = false
		} else {
			var piece strings.Builder
			cur, bad := need, false
			for j, part := range strings.Split(s, ".") {
				if j > 0 {
					piece.WriteByte('.')
					cur = true
				}
				if part == "" {
					continue
				}
				if !cur {
					bad = true
					break
				}
				piece.WriteString(part)
				cur = false
			}
			if bad {
				break
			}
			b.WriteString(piece.String())
			need = cur
		}
		// A path that ends at a dot is not complete yet: it may still become one.
		if _, valid := splitPath(b.String()); valid {
			best, n = b.String(), i+1
		}
	}
	if n == 0 {
		return "", 0, false
	}
	return best, n, true
}
