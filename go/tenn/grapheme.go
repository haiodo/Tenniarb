package tenn

import (
	"sort"
	"unicode"
)

// The standard library has no grapheme segmenter. This is an approximation of UAX #29 extended
// grapheme clusters (CR LF, controls, Extend/ZWJ/SpacingMark, Hangul, regional indicator pairs,
// emoji ZWJ sequences, Prepend, Indic conjuncts); Extended_Pictographic and Indic consonants come from grapheme_tables.go.
// Swift counts Characters, so token size and detectSymbolType depend on it.

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isControl(r rune) bool {
	if r == 0x200c || r == 0x200d || r >= 0xe0020 && r <= 0xe007f {
		return false
	}
	// Unassigned default-ignorable code points are Control too.
	if r == 0x2065 || r >= 0xfff0 && r <= 0xfff8 || r >= 0xe0000 && r <= 0xe0fff && !(r >= 0xe0100 && r <= 0xe01ef) {
		return true
	}
	return !isPrepend(r) && unicode.In(r, unicode.Cc, unicode.Zl, unicode.Zp, unicode.Cf)
}

// isPrepend lists Grapheme_Cluster_Break=Prepend (Unicode 16).
func isPrepend(r rune) bool {
	switch {
	case r >= 0x600 && r <= 0x605, r == 0x6dd, r == 0x70f, r >= 0x890 && r <= 0x891, r == 0x8e2, r == 0xd4e:
		return true
	case r == 0x110bd, r == 0x110cd, r >= 0x111c2 && r <= 0x111c3, r == 0x113d1, r == 0x1193f, r == 0x11941:
		return true
	}
	return r >= 0x11a84 && r <= 0x11a89 || r == 0x11d46 || r == 0x11f02
}

func isExtend(r rune) bool {
	switch {
	case r == 0x200c, r == 0x200d, r == 0xff9e, r == 0xff9f:
		return true
	case r >= 0x1f3fb && r <= 0x1f3ff, r >= 0xe0020 && r <= 0xe007f:
		return true
	}
	if r == 0xe33 || r == 0xeb3 {
		return true // SpacingMark letters
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc) && !isMcOther(r)
}

// isMcOther lists Mc characters whose Grapheme_Cluster_Break is Other (not SpacingMark).
func isMcOther(r rune) bool {
	switch {
	case r == 0x102b, r == 0x102c, r == 0x1038, r >= 0x1062 && r <= 0x1064, r >= 0x1067 && r <= 0x106d:
		return true
	case r == 0x1083, r >= 0x1087 && r <= 0x108c, r == 0x108f, r >= 0x109a && r <= 0x109c:
		return true
	case r == 0x1a61, r == 0x1a63, r == 0x1a64, r == 0xaa7b, r == 0xaa7d, r == 0x11720, r == 0x11721:
		return true
	}
	return false
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1f1e6 && r <= 0x1f1ff
}

func isExtPict(r rune) bool {
	return inRanges(r, extPictRanges)
}

func inRanges(r rune, table [][2]rune) bool {
	i := sort.Search(len(table), func(i int) bool { return table[i][1] >= r })
	return i < len(table) && table[i][0] <= r
}

// hangulClass: 'L', 'V', 'T', 'A' (LV syllable), 'B' (LVT syllable) or 0.
func hangulClass(r rune) byte {
	switch {
	case r >= 0x1100 && r <= 0x115f, r >= 0xa960 && r <= 0xa97c:
		return 'L'
	case r >= 0x1160 && r <= 0x11a7, r >= 0xd7b0 && r <= 0xd7c6:
		return 'V'
	case r >= 0x11a8 && r <= 0x11ff, r >= 0xd7cb && r <= 0xd7fb:
		return 'T'
	case r >= 0xac00 && r <= 0xd7a3:
		if (r-0xac00)%28 == 0 {
			return 'A'
		}
		return 'B'
	}
	return 0
}

func hangulJoins(p, r rune) bool {
	hp, hr := hangulClass(p), hangulClass(r)
	switch hp {
	case 'L':
		return hr == 'L' || hr == 'V' || hr == 'A' || hr == 'B'
	case 'A', 'V':
		return hr == 'V' || hr == 'T'
	case 'B', 'T':
		return hr == 'T'
	}
	return false
}

// graphemes splits s into extended grapheme clusters.
func graphemes(s string) []string {
	if isASCII(s) {
		// CR LF is the only multi-byte ASCII cluster.
		out := make([]string, 0, len(s))
		for i := 0; i < len(s); i++ {
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				out = append(out, s[i:i+2])
				i++
				continue
			}
			out = append(out, s[i:i+1])
		}
		return out
	}
	var out []string
	start := 0
	var prev rune
	var st clusterState
	for i, r := range s {
		if i > 0 && !joins(prev, r, st) {
			out = append(out, s[start:i])
			start = i
		}
		st.advance(r)
		prev = r
	}
	if len(s) > 0 {
		out = append(out, s[start:])
	}
	return out
}

// clusterState is what the join rules need to know about the cluster before the next rune.
type clusterState struct {
	riRun int // consecutive regional indicators ending at the previous rune
	pict  int // 1: ExtPict Extend* seen, 2: ... followed by ZWJ
	incb  int // 1: Indic consonant [Extend]* seen, 2: ... and a linker after it
}

func (st *clusterState) advance(r rune) {
	if isRegionalIndicator(r) {
		st.riRun++
	} else {
		st.riRun = 0
	}
	switch {
	case isExtPict(r):
		st.pict = 1
	case st.pict == 1 && r == 0x200d:
		st.pict = 2
	case st.pict == 1 && isExtend(r):
	default:
		st.pict = 0
	}
	switch {
	case isIndicConsonant(r):
		st.incb = 1
	case isIndicLinker(r):
		if st.incb > 0 {
			st.incb = 2
		}
	case isExtend(r):
	default:
		st.incb = 0
	}
}

func isIndicLinker(r rune) bool {
	switch r {
	case 0x94d, 0x9cd, 0xacd, 0xb4d, 0xc4d, 0xd4d:
		return true
	}
	return false
}

// isIndicConsonant is InCB=Consonant (GB9c).
func isIndicConsonant(r rune) bool {
	return inRanges(r, indicConsonantRanges)
}

// joins reports whether r continues the cluster that ends with p.
func joins(p, r rune, st clusterState) bool {
	switch {
	case p == '\r' && r == '\n':
		return true
	case p == '\r', p == '\n', isControl(p), r == '\r', r == '\n', isControl(r):
		return false
	case isPrepend(p), hangulJoins(p, r), isExtend(r):
		return true
	case st.pict == 2 && isExtPict(r):
		return true
	case st.incb == 2 && isIndicConsonant(r):
		return true
	case isRegionalIndicator(p) && isRegionalIndicator(r):
		return st.riRun%2 == 1
	}
	return false
}

func graphemeCount(s string) int {
	return len(graphemes(s))
}

// Graphemes splits s into extended grapheme clusters (Swift Characters).
func Graphemes(s string) []string {
	return graphemes(s)
}

// GraphemeCount is the number of extended grapheme clusters in s (Swift String.count).
func GraphemeCount(s string) int {
	return graphemeCount(s)
}
