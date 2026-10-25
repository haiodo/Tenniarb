package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Port of web/packages/core/test/sync.test.ts, plus a byte-exact check against the text JSON.stringify
// produced for the same model (number forms, escapes, empty and absent keys).

func syncModel(t *testing.T) *Element {
	t.Helper()
	m := ReadTenn("element \"A/B\" {\n item \"x\\ny\" {\n pos 10 20.5\n description \"d\"\n fill red\n }\n item \"z\"\n}")
	if m == nil {
		t.Fatal("model did not load")
	}
	return m.Elements[0]
}

func TestToSyncJSONItemsEdgesSwiftLayout(t *testing.T) {
	el := syncModel(t)
	a, b := el.Items[0], el.Items[1]
	el.AddItem(NewLinkItem(ItemKindLink, "", a, b))
	js := ToSyncJSON(el)

	var o struct {
		Name        string
		Description *string
		Items       []struct {
			Name        string
			Description string
			Pos         struct{ X, Y float64 }
			Properties  [][]string
		}
		Edges []struct{ Source, Target, ID string }
	}
	if err := json.Unmarshal([]byte(strings.ReplaceAll(js, `\/`, "/")), &o); err != nil {
		t.Fatal(err)
	}
	eq(t, o.Name, "A/B", "name")
	eq(t, len(o.Items), 2, "items")
	eq(t, o.Items[0].Name, "x\\ny", "newline stays escaped inside the string")
	eq(t, o.Items[0].Pos.X, 10.0, "x")
	eq(t, o.Items[0].Pos.Y, 20.5, "y")
	eq(t, o.Items[0].Description, "d", "description")
	eq(t, fmt.Sprint(o.Items[0].Properties), "[[fill red]]", "properties")
	if o.Items[1].Properties != nil || o.Description != nil {
		t.Fatal("absent keys must stay absent")
	}
	eq(t, len(o.Edges), 1, "edges")
	eq(t, o.Edges[0].Source, a.ID, "source")
	eq(t, o.Edges[0].Target, b.ID, "target")
	if !strings.Contains(js, "\n  \"name\" : \"A\\/B\",") {
		t.Fatalf("layout:\n%s", js)
	}
}

func TestToSyncJSONMatchesJSONStringify(t *testing.T) {
	m := ReadTenn("element \"A/B\" {\n item \"x\\ny\" {\n pos 10 20.5\n description \"d/é<>\"\n fill red\n }\n item \"z\"\n}")
	el := m.Elements[0]
	d := "top\nline"
	el.Description = &d
	a, b := el.Items[0], el.Items[1]
	b.X, b.Y = 1.0000000116860974e-7, 1.0000000200408773e+21
	link := NewLinkItem(ItemKindLink, "", a, b)
	el.AddItem(link)

	want := `{
  "name" : "A\/B",
  "description" : "top\nline",
  "items" : [
    {
      "kind" : "item",
      "name" : "x\\ny",
      "id" : "%[1]s",
      "pos" : {
        "x" : 10,
        "y" : 20.5
      },
      "description" : "d\/é<>",
      "properties" : [
        [
          "fill",
          "red"
        ]
      ]
    },
    {
      "kind" : "item",
      "name" : "z",
      "id" : "%[2]s",
      "pos" : {
        "x" : 1.0000000116860974e-7,
        "y" : 1.0000000200408773e+21
      }
    }
  ],
  "edges" : [
    {
      "kind" : "link",
      "name" : "",
      "id" : "%[3]s",
      "pos" : {
        "x" : 0,
        "y" : 0
      },
      "source" : "%[1]s",
      "target" : "%[2]s"
    }
  ]
}`
	eq(t, ToSyncJSON(el), fmt.Sprintf(want, a.ID, b.ID, link.ID), "json")
}

func TestToSyncJSONEmptyElement(t *testing.T) {
	eq(t, ToSyncJSON(NewElement("E")), "{\n  \"name\" : \"E\"\n}", "json")
}
