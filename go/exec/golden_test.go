package exec

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Port of web/packages/core/test/execution-golden.test.ts. Schema: see evalJSON in TenniarbTests/TennGoldenTests.swift.

const fixturesDir = "../../web/fixtures"

type tagged = map[string]any

func tag(v any, depth int) tagged {
	switch x := v.(type) {
	case UndefinedType:
		return tagged{"t": "undefined"}
	case nil:
		return tagged{"t": "null"}
	case bool:
		return tagged{"t": "boolean", "v": x}
	case float64:
		switch {
		case math.IsNaN(x):
			return tagged{"t": "number", "v": "NaN"}
		case math.IsInf(x, 1):
			return tagged{"t": "number", "v": "Infinity"}
		case math.IsInf(x, -1):
			return tagged{"t": "number", "v": "-Infinity"}
		}
		return tagged{"t": "number", "v": x}
	case string:
		return tagged{"t": "string", "v": x}
	case Error:
		return tagged{"t": "error", "name": x.Name}
	case Function:
		return tagged{"t": "function"}
	}
	if depth > 6 {
		return tagged{"t": "truncated"}
	}
	switch x := v.(type) {
	case []any:
		arr := make([]any, len(x))
		for i, e := range x {
			arr[i] = tag(e, depth+1)
		}
		return tagged{"t": "array", "v": arr}
	case map[string]any:
		obj := map[string]any{}
		for k, e := range x {
			obj[k] = tag(e, depth+1)
		}
		return tagged{"t": "object", "v": obj}
	}
	panic("unexpected exported value")
}

type entry struct {
	path    []int
	kind    string
	literal string
	value   tagged
}

func entries(t *testing.T, props *tenn.Node, ev map[*tenn.Token]any) []entry {
	var result []entry
	var walk func(n *tenn.Node, path []int)
	walk = func(n *tenn.Node, path []int) {
		if n.Token != nil {
			if v, ok := ev[n.Token]; ok {
				result = append(result, entry{append([]int(nil), path...), string(n.Kind), n.Token.Literal, tag(v, 0)})
			}
		}
		for i, c := range n.Children {
			walk(c, append(path, i))
		}
	}
	for i, p := range props.Children {
		walk(p, []int{i})
	}
	if len(result) != len(ev) {
		t.Fatalf("evaluated tokens not reachable from properties: %d of %d", len(result), len(ev))
	}
	return result
}

type elementEval struct {
	path    []int
	name    string
	element []entry
	items   []itemEval
}

type itemEval struct {
	index   int
	kind    string
	name    string
	entries []entry
}

// Same setup as the Swift side; width/height come from a fixed size since drawables are not ported yet.
func evaluateAll(t *testing.T, root *model.Element) []elementEval {
	var out []elementEval
	var visit func(e *model.Element, path []int)
	visit = func(e *model.Element, path []int) {
		ctx := NewExecutionContext(Options{ItemSize: func(*model.DiagramItem, model.IExecutionContext) *model.DrawableSize {
			return &model.DrawableSize{Width: 100, Height: 40}
		}})
		ctx.SetElement(e)
		own := entries(t, e.Properties.AsNode(), ctx.GetEvaluated(e, nil, nil))
		var items []itemEval
		for index, itm := range e.Items {
			ens := entries(t, itm.Properties.AsNode(), ctx.GetEvaluated(itm, nil, nil))
			if len(ens) > 0 {
				kind := "Item"
				if itm.Kind == model.ItemKindLink {
					kind = "Link"
				}
				items = append(items, itemEval{index, kind, itm.Name, ens})
			}
		}
		if len(own) > 0 || len(items) > 0 {
			out = append(out, elementEval{append([]int(nil), path...), e.Name, own, items})
		}
		for i, c := range e.Elements {
			visit(c, append(path, i))
		}
	}
	for i, c := range root.Elements {
		visit(c, []int{i})
	}
	return out
}

func jsonRoundTrip(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// JSC error messages are engine specific, so errors keep only the tag and the name.
func checkEntries(t *testing.T, where string, actual []entry, expected []entry) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s: %d entries, want %d", where, len(actual), len(expected))
	}
	for i, e := range expected {
		a := actual[i]
		if !reflect.DeepEqual(a.path, e.path) || a.kind != e.kind || a.literal != e.literal {
			t.Fatalf("%s[%d]: got %v %s %q, want %v %s %q", where, i, a.path, a.kind, a.literal, e.path, e.kind, e.literal)
		}
		got := jsonRoundTrip(a.value)
		if e.value["t"] == "error" {
			e.value = tagged{"t": "error", "name": e.value["name"]}
		}
		if !reflect.DeepEqual(got, jsonRoundTrip(e.value)) {
			t.Fatalf("%s[%d] %q: got %v, want %v", where, i, e.literal, got, e.value)
		}
	}
}

type jsonEntry struct {
	Path    []int  `json:"path"`
	Kind    string `json:"kind"`
	Literal string `json:"literal"`
	Value   tagged `json:"value"`
}

func (j jsonEntry) entry() entry { return entry{j.Path, j.Kind, j.Literal, j.Value} }

func toEntries(js []jsonEntry) []entry {
	var out []entry
	for _, j := range js {
		out = append(out, j.entry())
	}
	return out
}

func TestEvalGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(fixturesDir, "*.tenn"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no .tenn fixtures in %s", fixturesDir)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, ".saved.tenn") {
			continue
		}
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.TrimSuffix(f, ".tenn") + ".eval.json"
			_, statErr := os.Stat(golden)
			root := model.ReadTenn(string(src))
			if root == nil {
				if statErr == nil {
					t.Fatal("golden exists for a document with parse errors")
				}
				return
			}
			actual := evaluateAll(t, &root.Element)
			if statErr != nil {
				if len(actual) != 0 {
					t.Fatal("Go evaluated something Swift did not")
				}
				return
			}
			checked++
			data, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			var expected struct {
				Elements []struct {
					Path    []int       `json:"path"`
					Name    string      `json:"name"`
					Element []jsonEntry `json:"element"`
					Items   []struct {
						Index   int         `json:"index"`
						Kind    string      `json:"kind"`
						Name    string      `json:"name"`
						Entries []jsonEntry `json:"entries"`
					} `json:"items"`
				} `json:"elements"`
			}
			if err := json.Unmarshal(data, &expected); err != nil {
				t.Fatal(err)
			}
			if len(actual) != len(expected.Elements) {
				t.Fatalf("%d evaluated elements, want %d", len(actual), len(expected.Elements))
			}
			for i, e := range expected.Elements {
				a := actual[i]
				if !reflect.DeepEqual(a.path, e.Path) || a.name != e.Name {
					t.Fatalf("element %d: got %v %q, want %v %q", i, a.path, a.name, e.Path, e.Name)
				}
				checkEntries(t, "element", a.element, toEntries(e.Element))
				if len(a.items) != len(e.Items) {
					t.Fatalf("element %q: %d items, want %d", e.Name, len(a.items), len(e.Items))
				}
				for j, it := range e.Items {
					ai := a.items[j]
					if ai.index != it.Index || ai.kind != it.Kind || ai.name != it.Name {
						t.Fatalf("item %d: got %d %s %q, want %d %s %q", j, ai.index, ai.kind, ai.name, it.Index, it.Kind, it.Name)
					}
					checkEntries(t, "item "+it.Name, ai.entries, toEntries(it.Entries))
				}
			}
		})
	}
	if checked != 10 {
		t.Fatalf("checked %d eval goldens, want 10", checked)
	}
}
