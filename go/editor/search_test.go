package editor

import (
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

func searchElement() *model.Element {
	return model.ReadTenn(`element "D" {
  item "Zed" { body "first\nsecond" }
  item "alpha" { body { text "Hello World" } }
  item "Mid"
}
element "Other" { item "Hello" }`).Elements[0]
}

func names(items []*model.DiagramItem) []string {
	out := []string{}
	for _, i := range items {
		out = append(out, i.Name)
	}
	return out
}

func TestSearchItems(t *testing.T) {
	el := searchElement()
	eq(t, "body", names(SearchItems(el, "hello")), []string{"alpha"})
	eq(t, "name", names(SearchItems(el, "ZE")), []string{"Zed"})
	eq(t, "second line", names(SearchItems(el, "second")), []string{"Zed"})
	eq(t, "all, by name", names(SearchItems(el, "")), []string{"Mid", "Zed", "alpha"})
	eq(t, "none", names(SearchItems(el, "nothing")), []string{})
}

func TestRowText(t *testing.T) {
	items := SearchItems(searchElement(), "")
	eq(t, "name only", RowText(items[0]), "Mid")
	eq(t, "name and body", RowText(items[1]), `Zed - first\nsecond`)
}
