package main

import (
	"fmt"
	"strings"
)

type lcg uint32

func (r *lcg) next(n int) int {
	*r = *r*1664525 + 1013904223
	return int(uint32(*r)>>8) % n
}

var words = strings.Fields("alpha beta gamma delta service gateway queue cache worker scheduler index storage router balancer metric trace")

var emoji = []string{"😀", "🚀", "🔥", "👨‍👩‍👧‍👦", "🇺🇸", "👍🏽", "❤️", "✌🏽", "1️⃣", "🏴󠁧󠁢󠁥󠁮󠁧󠁿"}
var nonASCII = []string{"Café", "日本語", "Ñandú", "Привет", "naïve", "Ελληνικά", "العربية", "한국어", "हिन्दी", "ภาษาไทย"}

// genDiagram builds a deterministic ASCII diagram with n items and n links.
func genDiagram(n int) string { return gen(n, false) }

// genEmoji is the same shape with emoji and non-ASCII text in names, bodies and expressions.
func genEmoji(n int) string { return gen(n, true) }

func gen(n int, uni bool) string {
	r := lcg(42)
	name := func(i int) string {
		s := fmt.Sprintf("%s %s %d", words[r.next(len(words))], words[r.next(len(words))], i)
		if uni {
			s = nonASCII[r.next(len(nonASCII))] + " " + emoji[r.next(len(emoji))] + " " + s
		}
		return s
	}
	var b strings.Builder
	names := make([]string, n)
	b.WriteString("element \"Bench\" {\n    styles {\n")
	for s := 0; s < 8; s++ {
		fmt.Fprintf(&b, "        item \"s%d\" {\n            fill-color #%06x\n            shadow -5 -5 %d\n            font-size %d\n        }\n", s, r.next(0xffffff), s+1, 10+s)
	}
	b.WriteString("    }\n")
	for i := 0; i < n; i++ {
		names[i] = name(i)
		fmt.Fprintf(&b, "    item %q {\n", names[i])
		fmt.Fprintf(&b, "        pos %d.%d %d.%d\n", r.next(4000)-2000, r.next(1000), r.next(4000)-2000, r.next(1000))
		fmt.Fprintf(&b, "        style \"s%d\"\n", r.next(8))
		if i%3 == 0 {
			fmt.Fprintf(&b, "        font-size %d\n", 8+r.next(20))
		}
		if i%5 == 0 {
			fmt.Fprintf(&b, "        title ${%d * utils.textWidth(name) + %d}\n", r.next(9)+1, r.next(100))
		}
		if i%2 == 0 {
			b.WriteString("        body {\n            text %{\n")
			fmt.Fprintf(&b, "        # Item %d\n", i)
			for l := 0; l < 3+r.next(3); l++ {
				b.WriteString("        * ")
				for w := 0; w < 4+r.next(6); w++ {
					b.WriteString(words[r.next(len(words))])
					b.WriteByte(' ')
				}
				if l == 1 {
					fmt.Fprintf(&b, "${items.x%d.width + %d} ", r.next(50), r.next(9))
				}
				if uni {
					b.WriteString(emoji[r.next(len(emoji))] + " " + nonASCII[r.next(len(nonASCII))] + " ")
				}
				b.WriteString("`code` **bold** _it_\n")
			}
			b.WriteString("    }\n        }\n")
		}
		b.WriteString("    }\n")
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "    link %q %q {\n        display arrow\n", names[i], names[r.next(n)])
		if i%4 == 0 {
			fmt.Fprintf(&b, "        source-index %d\n", r.next(3))
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n")
	return b.String()
}
