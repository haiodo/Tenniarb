package scene_test

import (
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// A link of the body is its label with a box on the screen and its target: a Russian anchor without a
// file is a heading of the same text, and the heading has a box of its own below the link.
func TestBodyLinkBoxes(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "Doc" {
        width 300
        body %{
            # План

            [Цель](#Цель)

            ## Цель

            Текст.
        }
    }
}`)
	links := s.BodyLinks()
	if len(links) != 1 {
		t.Fatalf("%d links, want one: %+v", len(links), links)
	}
	l := links[0]
	if l.File != "" || l.Anchor != "цель" {
		t.Errorf("link points to %q %q, want this text and the heading цель", l.File, l.Anchor)
	}
	if l.W <= 0 || l.H <= 0 || l.X < 0 || l.X+l.W > 300 {
		t.Errorf("link box %v is not on the page", l.Box)
	}
	anchors := s.BodyAnchors()
	got := map[string]scene.BodyAnchor{}
	for _, a := range anchors {
		got[a.Anchor] = a
	}
	head, ok := got["цель"]
	if !ok {
		t.Fatalf("headings %+v have no цель", anchors)
	}
	if _, ok := got["план"]; !ok {
		t.Errorf("headings %+v have no план", anchors)
	}
	if head.Y <= l.Y {
		t.Errorf("heading Цель at y=%v is not below the link at y=%v", head.Y, l.Y)
	}
	if head.X != l.X {
		t.Errorf("heading starts at x=%v, the link at x=%v, both left of the body", head.X, l.X)
	}
}

// A body of a leaf is a block under its title: the links of it are placed below the title.
func TestBodyLinkInFlow(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "Card" {
        auto-layout column
        width 300
        title "Head"
        body %{
            [Цель](#Цель)

            ## Цель
        }
    }
}`)
	links := s.BodyLinks()
	if len(links) != 1 {
		t.Fatalf("%d links, want one: %+v", len(links), links)
	}
	if l := links[0]; l.Y < 17 {
		t.Errorf("the link at y=%v is not below the 17 point title", l.Y)
	}
	for _, a := range s.BodyAnchors() {
		if a.Anchor == "цель" && a.Y <= links[0].Y {
			t.Errorf("heading at y=%v is not below the link at y=%v", a.Y, links[0].Y)
		}
	}
}

// The link is resolved from the markdown file the body came from: its own file is this text, another file
// is that file, and a URL is no body link.
func TestBodyLinkTargets(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md":  "# План\n\n[сюда](plan.md#Цель) [туда](other.md#раздел) [сайт](https://example.com)\n\n## Цель\n\nтекст\n",
		"docs/other.md": "# Другое\n\n## Раздел\n\nтекст\n",
		"a.tenn":        `element "E" { item "Doc" { width 1000; body --from-file "docs/plan.md" } }`,
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := buildFile(t, tree, "E")
	links := s.BodyLinks()
	if len(links) != 2 {
		t.Fatalf("%d links, want the two section links: %+v", len(links), links)
	}
	want := []struct{ file, anchor string }{{"", "цель"}, {"docs/other.md", "раздел"}}
	for i, w := range want {
		if links[i].File != w.file || links[i].Anchor != w.anchor {
			t.Errorf("link %d points to %q %q, want %q %q", i, links[i].File, links[i].Anchor, w.file, w.anchor)
		}
	}
	if links[0].Y != links[1].Y || links[0].X >= links[1].X {
		t.Errorf("the two links of one line are at %v and %v", links[0].Box, links[1].Box)
	}
}

// A code span is text: its brackets are drawn and never a link.
func TestBodyLinkCodeSpan(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "Doc" {
        width 1000
        body %{
            `+"`[нет](#нет)`"+` and [да](#да)

            ## Да
        }
    }
}`)
	links := s.BodyLinks()
	if len(links) != 1 || links[0].Anchor != "да" {
		t.Fatalf("links %+v, want only the real one", links)
	}
}

// AnchorElement finds the element whose body shows a heading, written there or read from a file.
func TestAnchorElement(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": "# План\n\n## Цель\n",
		"a.tenn": `element "A" { item "One" { body --from-file "docs/plan.md" } }
element "B" { item "Two" { body %{ ## Другое } } }`,
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	for anchor, want := range map[string]string{"цель": "A", "другое": "B"} {
		path, ok := tree.AnchorElement(anchor)
		if !ok || len(path) != 1 || path[0] != want {
			t.Errorf("heading %q is at %v %v, want element %s", anchor, path, ok, want)
		}
	}
	if path, ok := tree.AnchorElement("нигде"); ok {
		t.Errorf("a heading nowhere is at %v", path)
	}
}
