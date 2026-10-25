package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/haiodo/tenniarb/go/tenn"
)

// firstIndexOf is the byte index of the first Character equal to ch (a single ASCII char), -1 if none.
func firstIndexOf(s, ch string) int {
	i := strings.Index(s, ch)
	if i < 0 {
		return i
	}
	prev, _ := utf8.DecodeLastRuneInString(s[:i])
	next, _ := utf8.DecodeRuneInString(s[i+len(ch):])
	if (i == 0 || prev < 0x300) && (i+len(ch) >= len(s) || next < 0x300) {
		return i
	}
	off := 0
	for _, g := range tenn.Graphemes(s) {
		if g == ch {
			return off
		}
		off += len(g)
	}
	return -1
}

func startsWithChar(s, ch string) bool {
	if !strings.HasPrefix(s, ch) {
		return false
	}
	return len(s) == len(ch) || s[len(ch)] < utf8.RuneSelf || tenn.Graphemes(s)[0] == ch
}

// isSpace is CharacterSet.whitespacesAndNewlines: Zs/Zl/Zp, U+0009-U+000D, U+0085.
func isSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r == ' ', r == 0x85, r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a:
		return true
	}
	return r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000
}

func trimmed(s string) string {
	return strings.TrimFunc(s, isSpace)
}
