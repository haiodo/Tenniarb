package mcpserver

import (
	_ "embed"
	"fmt"
	"strings"
)

// specText is a copy of docs/spec/tenn-scene.md; TestSpecCopyInSync fails when they differ.
//
//go:embed spec/tenn-scene.md
var specText string

const examples = `To draw a screen or a diagram, read the design skill first: read_skill (no arguments).

# Examples

Y axis points up: pos 10 20 puts the top left corner of an item 20 units above the origin.
Paths of use follow spec section 4.2: element names, then node names, joined by dots; a name
is normalized (every non-letter, non-digit becomes "_"), so "Primary button" is Primary_button.

A card with auto-layout:

` + "```tenn" + `
element "Login" {
    item "Card" {
        auto-layout column
        gap 8
        padding 16
        item "Title" { title "Sign in" }
        item "Go" { title "Go"; color green }
    }
}
` + "```" + `

A component and two instances with a slot and an override:

` + "```tenn" + `
element "Kit" {
    item "Window" {
        auto-layout column
        gap 4
        item "Head" { title "Window" }
        slot {
            item "Empty" { title "Empty" }
        }
    }
}
element "Screens" {
    use Kit.Window as w1 {
        at Head { title "Sign in" }
        item "Email" { title "Email" }
    }
    use Kit.Window as w2
}
` + "```" + `

A classic diagram (brain map): items placed by pos, links between items of one element.

` + "```tenn" + `
element "Plan" {
    item "Idea" { pos 0 0 }
    item "Screen" {
        pos 160 40
        color blue
    }
    link "Idea" "Screen" { display arrow }
}
` + "```" + `

# Scenes and diagrams

One rule decides how an element draws, by what is written directly in it: a use, or an item with
children, a slot or auto-layout, makes it a UI scene (components, auto-layout; spec below, 5a.17);
content at the file root, outside any element, follows the same rule. A slot written directly in an
element is a plain property. Otherwise it is a classic diagram, drawn as the Tenniarb
canvas draws it: items at pos, links, styles, element layouts, ${...} expressions evaluated.
get_state shows the renderer of each element. read, edit and validate work the same on both.

# The Tenniarb app

When the Tenniarb app has a file open, read, get_state and validate see its unsaved text, and edit
changes the document in the app as one undo step (the file on disk is untouched until the user
saves). If the user typed since you read, edit fails: read again. selection shows the open
documents and what the user selected; select points at nodes; render with view shows the window.
Without the app every tool works on the files on disk.

Typical loop: get_state, read the file or one node, edit with find/replace, validate, render.
edit is atomic: on any mismatch, parse error or new validation error nothing is written.
`

// guide returns the examples and the spec, or the spec sections that match topic: a heading
// number such as "4.7a" or a word of the heading.
func guide(topic string) string {
	topic = strings.TrimSpace(topic)
	switch strings.ToLower(topic) {
	case "":
		return examples + "\n# Spec (docs/spec/tenn-scene.md)\n\n" + specText
	case "examples":
		return examples
	}
	secs := specSections()
	var out []string
	for _, sec := range secs {
		if matchHeading(sec.title, topic) {
			out = append(out, sec.body)
		}
	}
	if len(out) == 0 {
		var hs []string
		for _, sec := range secs {
			hs = append(hs, "  "+sec.title)
		}
		return fmt.Sprintf("no section matches %q. Topics: examples, or a heading number or word:\n%s", topic, strings.Join(hs, "\n"))
	}
	return strings.Join(out, "\n")
}

type section struct{ title, body string }

// specSections cuts the spec at "##" and "###" headings; a "##" section includes its "###" parts.
func specSections() []section {
	lines := strings.Split(specText, "\n")
	var out []section
	for i, ln := range lines {
		level := headingLevel(ln)
		if level < 2 {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if l := headingLevel(lines[j]); l > 0 && l <= level {
				end = j
				break
			}
		}
		out = append(out, section{strings.TrimSpace(strings.TrimLeft(ln, "#")), strings.Join(lines[i:end], "\n")})
	}
	return out
}

func headingLevel(ln string) int {
	n := 0
	for n < len(ln) && ln[n] == '#' {
		n++
	}
	if n == 0 || n > 3 || len(ln) <= n || ln[n] != ' ' {
		return 0
	}
	return n
}

func matchHeading(title, topic string) bool {
	t, q := strings.ToLower(title), strings.ToLower(topic)
	if q[0] >= '0' && q[0] <= '9' {
		num, _, _ := strings.Cut(t, " ")
		return strings.TrimRight(num, ".") == strings.TrimRight(q, ".")
	}
	return strings.Contains(t, q)
}
