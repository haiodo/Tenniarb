package check

import (
	"os"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// quoteTitles rewrites a title written with spaces and no quotes, `title Gap and padding`, as one quoted
// text, `title "Gap and padding"`: only the first word would be drawn, and the file says so with an error
// a reader has to guess the fix for. It answers the text and how many titles it changed.
func quoteTitles(text string, root *tenn.Node) (string, int) {
	type edit struct {
		start, end int
		repl       string
	}
	var edits []edit
	var walk func(n *tenn.Node)
	walk = func(n *tenn.Node) {
		if n.Kind == tenn.KindCommand && n.Token != nil && strings.EqualFold(n.Token.Literal, "title") {
			if repl, ok := quotedTitle(n); ok {
				// Children[0] is the name of the command itself, so the arguments start at one.
				edits = append(edits, edit{n.Children[1].Token.Pos, n.End, repl})
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	if len(edits) == 0 {
		return text, 0
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		if e.start < 0 || e.end > len(text) || e.start >= e.end {
			continue
		}
		text = text[:e.start] + e.repl + text[e.end:]
	}
	return text, len(edits)
}

// quotedTitle is the one quoted argument for a title, false when the title is not the broken shape: one
// value already, or an argument that is not a bare word (a body, a block, an expression of its own).
func quotedTitle(n *tenn.Node) (string, bool) {
	args := n.Children[1:]
	if len(args) < 2 {
		return "", false
	}
	words := make([]string, 0, len(args))
	for _, c := range args {
		if c.Kind != tenn.KindIdent || c.Token == nil {
			return "", false
		}
		words = append(words, c.Token.Literal)
	}
	return quoteText(strings.Join(words, " ")), true
}

// quoteText is text as a tenn string: in quotes, with the quote and the backslash of the text escaped.
func quoteText(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
	return b.String()
}

// FixTitles is the text with every title of one unquoted value quoted, and how many it changed. A text
// that does not parse comes back as it was: the check says what is wrong with it, and a guess would only
// make it worse. The window calls it for the button that fixes what the banner reports.
func FixTitles(text string) (string, int) {
	root, parser := tenn.Parse(strings.TrimPrefix(text, "\xef\xbb\xbf"))
	if parser.Errors.HasErrors() || root == nil {
		return text, 0
	}
	fixed, n := quoteTitles(text, root)
	if n == 0 {
		return text, 0
	}
	return fixed, n
}

// Fixable reports whether a diagnostic of the check is one the fixer repairs: a title of one unquoted
// value. The window shows its Fix button for a message that names one.
func Fixable(diagnostic string) bool {
	return strings.Contains(diagnostic, "title takes one value")
}

// fixFile applies FixTitles to the file at abs, and answers how many titles it rewrote. A file that does
// not read is left alone: the diagnostics of the check say what is wrong with it.
func fixFile(abs string) (int, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return 0, err
	}
	fixed, n := FixTitles(string(data))
	if n == 0 {
		return 0, nil
	}
	return n, os.WriteFile(abs, []byte(fixed), 0o644)
}
