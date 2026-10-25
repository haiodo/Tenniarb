package model

import (
	"testing"

	"github.com/haiodo/tenniarb/go/tenn"
)

func TestCloneRemapsLinks(t *testing.T) {
	m := NewElementModelFactory().ElementModel
	d := m.Elements[0]
	a := d.AddGet(NewDiagramItem(ItemKindItem, "a"))
	b := d.AddGet(NewDiagramItem(ItemKindItem, "b"))
	d.AddSourceTarget(a, b)

	cp := d.Clone(true, true)
	link := cp.Items[2]
	if link.Source != cp.Items[0] || link.Target != cp.Items[1] {
		t.Fatal("link ends not remapped to cloned items")
	}
	if len(d.FindLinks(a)) != 1 || len(d.GetRelatedItems(a, true, true)) != 2 {
		t.Fatal("FindLinks/GetRelatedItems")
	}
	if a.Parent != d || d.Model != m {
		t.Fatal("parent/model not assigned")
	}
}

func TestInsertOutOfRange(t *testing.T) {
	e := NewElement("x")
	if e.InsertItem(1, NewDiagramItem(ItemKindItem, "i")) == nil || e.InsertElement(-1, NewElement("c")) == nil {
		t.Fatal("expected range error")
	}
}

func TestUnresolvedLinkKeptVerbatim(t *testing.T) {
	m := ReadTenn("element \"A\" { item \"x\"\n link \"x\" \"nope\" }\n")
	el := m.Elements[0]
	if len(el.Items) != 1 || el.Properties.Count() != 1 || el.Properties.Get("link") == nil {
		t.Fatal("unresolved link not kept in properties")
	}
}

func TestPropsRoundTrip(t *testing.T) {
	item := NewDiagramItem(ItemKindItem, "n")
	item.X = 3
	node := ItemToTennAsProps(item, tenn.KindStatements, true)
	back := NewDiagramItem(ItemKindItem, "")
	ItemFromTennProps(back, node)
	if back.Name != "n" || back.X != 3 {
		t.Fatalf("got %q %v", back.Name, back.X)
	}
}

func TestLinkResolvesAcrossNormalForms(t *testing.T) {
	src := "element \"A\" {\n item \"é\"\n item \"é\"\n link \"é\" \"é\" { source-index 1 }\n}\n"
	el := ReadTenn(src).Elements[0]
	if len(el.Items) != 3 {
		t.Fatalf("items %d, link not resolved", len(el.Items))
	}
	link := el.Items[2]
	if link.Source != el.Items[1] || link.Target != el.Items[0] {
		t.Fatal("source-index 1 must resolve to the second item")
	}
}
