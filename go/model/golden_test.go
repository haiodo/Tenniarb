package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/tenn"
)

const fixturesDir = "../../web/fixtures"

// Same conversion as toSwiftToken in go/tenn/golden_test.go: Size and Pos are reported as Swift does.
func toSwiftToken(t tenn.Token) map[string]any {
	end := t.Pos + t.Size
	if t.Type == tenn.TokSemiColon && t.Literal == ";" {
		end = t.Pos // Swift adds ";" before consuming it
	}
	size := tenn.GraphemeCount(t.Literal)
	return map[string]any{
		"type": string(t.Type), "literal": t.Literal, "line": t.Line, "col": t.Col,
		"pos": end - size, "size": size,
	}
}

// withTokens false drops positions (round trip).
func nodeJSON(n *tenn.Node, withTokens bool) map[string]any {
	out := map[string]any{"kind": string(n.Kind)}
	if n.Token != nil {
		if withTokens {
			out["token"] = toSwiftToken(*n.Token)
		} else {
			out["token"] = map[string]any{"type": string(n.Token.Type), "literal": n.Token.Literal}
		}
	}
	if n.Children != nil {
		children := make([]any, 0, len(n.Children))
		for _, c := range n.Children {
			children = append(children, nodeJSON(c, withTokens))
		}
		out["children"] = children
	}
	return out
}

func propsJSON(p *ModelProperties, withTokens bool) []any {
	out := []any{}
	for _, n := range p.Nodes() {
		out = append(out, nodeJSON(n, withTokens))
	}
	return out
}

func itemJSON(i *DiagramItem, owner *Element, withTokens bool) map[string]any {
	kind := "Item"
	if i.Kind == ItemKindLink {
		kind = "Link"
	}
	r := map[string]any{
		"kind": kind, "name": i.Name, "x": i.X, "y": i.Y,
		"properties": propsJSON(i.Properties, withTokens),
	}
	if i.Description != nil {
		r["description"] = *i.Description
	}
	if i.Kind == ItemKindLink {
		idx := func(t *DiagramItem) any {
			if k := slices.Index(owner.Items, t); t != nil && k >= 0 {
				return k
			}
			return nil
		}
		r["source"] = idx(i.Source)
		r["target"] = idx(i.Target)
	}
	return r
}

func modelJSON(e *Element, withTokens bool) map[string]any {
	kind := "Element"
	if e.Kind == KindRoot {
		kind = "Root"
	}
	items := []any{}
	for _, i := range e.Items {
		items = append(items, itemJSON(i, e, withTokens))
	}
	elements := []any{}
	for _, c := range e.Elements {
		elements = append(elements, modelJSON(c, withTokens))
	}
	r := map[string]any{
		"kind": kind, "name": e.Name, "properties": propsJSON(e.Properties, withTokens),
		"items": items, "elements": elements,
	}
	if e.Description != nil {
		r["description"] = *e.Description
	}
	return r
}

func roundTrip(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func firstDiff(path string, got, want any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %v, want object", path, got)
		}
		keys := []string{}
		for k := range w {
			keys = append(keys, k)
		}
		for k := range g {
			if _, ok := w[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			if d := firstDiff(path+"."+k, g[k], w[k]); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %v, want array", path, got)
		}
		for i := 0; i < len(w) && i < len(g); i++ {
			if d := firstDiff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length got %d, want %d", path, len(g), len(w))
		}
		return ""
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Sprintf("%s: got %#v, want %#v", path, got, want)
	}
	return ""
}

func fixtureNames(t *testing.T) []string {
	t.Helper()
	all, err := filepath.Glob(filepath.Join(fixturesDir, "*.tenn"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range all {
		if !strings.HasSuffix(f, ".saved.tenn") {
			names = append(names, filepath.Base(f))
		}
	}
	if len(names) == 0 {
		t.Fatalf("no .tenn fixtures in %s", fixturesDir)
	}
	return names
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(name string) bool {
	_, err := os.Stat(filepath.Join(fixturesDir, name))
	return err == nil
}

func load(source string) (*ElementModel, int) {
	tree, parser := tenn.Parse(source)
	return ParseTenn(tree), len(parser.Errors.Errors)
}

func TestPersistenceGolden(t *testing.T) {
	for _, f := range fixtureNames(t) {
		base := strings.TrimSuffix(f, ".tenn")
		t.Run(base, func(t *testing.T) {
			source := strings.TrimPrefix(readFile(t, f), "\xef\xbb\xbf")
			model, errs := load(source)

			// Like Document.read: documents with parse errors are not loaded, and have no model/saved goldens.
			if errs > 0 {
				if exists(base+".model.json") || exists(base+".saved.tenn") {
					t.Fatal("persistence goldens exist for a fixture with parse errors")
				}
				return
			}

			t.Run("model", func(t *testing.T) {
				var want any
				if err := json.Unmarshal([]byte(readFile(t, base+".model.json")), &want); err != nil {
					t.Fatal(err)
				}
				if d := firstDiff("", roundTrip(t, modelJSON(&model.Element, true)), want); d != "" {
					t.Fatalf("model golden differs, first difference %s", d)
				}
			})

			t.Run("saved", func(t *testing.T) {
				if got := ToTennStr(&model.Element, true, true); got != readFile(t, base+".saved.tenn") {
					t.Fatalf("saved golden differs:\n%s", got)
				}
			})

			t.Run("roundtrip", func(t *testing.T) {
				again, errs := load(ToTennStr(&model.Element, true, true))
				if errs != 0 {
					t.Fatalf("saved text has %d parse errors", errs)
				}
				if d := firstDiff("", roundTrip(t, modelJSON(&again.Element, false)), roundTrip(t, modelJSON(&model.Element, false))); d != "" {
					t.Fatalf("round trip differs, first difference %s", d)
				}
			})
		})
	}
}

func TestReadTennStripsBOM(t *testing.T) {
	src := "element \"A\" { item \"x\" { pos 1 2 } }\n"
	withBOM, plain := ReadTenn("\xef\xbb\xbf"+src), ReadTenn(src)
	if withBOM == nil || plain == nil || len(withBOM.Elements) != 1 {
		t.Fatal("BOM source not loaded")
	}
	if d := firstDiff("", roundTrip(t, modelJSON(&withBOM.Element, false)), roundTrip(t, modelJSON(&plain.Element, false))); d != "" {
		t.Fatalf("differs %s", d)
	}
}

func TestReadTennNilOnParseErrors(t *testing.T) {
	if ReadTenn("element \"A\" { item \"x") != nil {
		t.Fatal("expected nil")
	}
}
