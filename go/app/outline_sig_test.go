package app

import "testing"

func TestOutlineSignature(t *testing.T) {
	a := ParseText(sample).Model
	b := ParseText(sample).Model
	if OutlineSignature(&a.Element) != OutlineSignature(&b.Element) {
		t.Fatal("equal models, different signatures")
	}
	base := OutlineSignature(&a.Element)
	a.Elements[0].Name = "A2"
	if OutlineSignature(&a.Element) == base {
		t.Fatal("rename not seen")
	}
	b.Elements[0].Elements = b.Elements[0].Elements[:1]
	if OutlineSignature(&b.Element) == base {
		t.Fatal("removal not seen")
	}
	c := ParseText(sample).Model
	c.Elements[0], c.Elements[1] = c.Elements[1], c.Elements[0]
	if OutlineSignature(&c.Element) == base {
		t.Fatal("move not seen")
	}
	c = ParseText(sample).Model
	c.Elements[0].Elements[0].Items = nil
	if OutlineSignature(&c.Element) != base {
		t.Fatal("items must not change the outline")
	}
}
