package tenn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const fixturesDir = "../../web/fixtures"

// The *.parse.json goldens hold the values the Swift lexer reports. The Go lexer already counts
// Col in UTF-8 bytes, like Swift, so only Size (Swift: Characters in the literal) and Pos
// (Swift: UTF-8 offset of the lexer position at add time minus Size) are converted here.
// This is the same conversion as web/packages/core/test/swift-units.ts.
func toSwiftToken(t Token) map[string]any {
	end := t.Pos + t.Size
	if t.Type == TokSemiColon && t.Literal == ";" {
		end = t.Pos // Swift adds ";" before consuming it
	}
	size := graphemeCount(t.Literal)
	return map[string]any{
		"type": string(t.Type), "literal": t.Literal, "line": t.Line, "col": t.Col,
		"pos": end - size, "size": size,
	}
}

func nodeJSON(n *Node) map[string]any {
	out := map[string]any{"kind": string(n.Kind)}
	if n.Token != nil {
		out["token"] = toSwiftToken(*n.Token)
	}
	if n.Children != nil {
		children := make([]any, 0, len(n.Children))
		for _, c := range n.Children {
			children = append(children, nodeJSON(c))
		}
		out["children"] = children
	}
	return out
}

func buildGolden(source string) map[string]any {
	lexer := NewLexer(source)
	tokens := []any{}
	for t := lexer.GetToken(); t != nil; t = lexer.GetToken() {
		tokens = append(tokens, toSwiftToken(*t))
	}
	parser := NewParser(nil)
	tree := parser.Parse(source)
	errs := []any{}
	for _, e := range parser.Errors.Errors {
		errs = append(errs, map[string]any{
			"code": string(e.ErrorCode), "message": e.Message, "line": e.Line, "col": e.Col,
		})
	}
	return map[string]any{
		"tokens":       tokens,
		"tree":         nodeJSON(tree),
		"printed":      ToStr(tree, 0, false),
		"printedClean": ToStr(tree, 0, true),
		"errors":       errs,
	}
}

// roundTrip puts built values through JSON so numbers and slices have the same types as the golden.
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

// firstDiff returns the path and values of the first difference between two decoded JSON values.
func firstDiff(path string, got, want any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %v, want object", path, got)
		}
		keys := make([]string, 0, len(w)+len(g))
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
	if _, err := os.Stat(fixturesDir); err != nil {
		t.Fatalf("fixtures directory missing: %v", err)
	}
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

// Swift reads fixtures with String(contentsOf:), which drops a leading BOM.
func readSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimPrefix(string(data), "\xef\xbb\xbf")
}

// *.saved.tenn is the persistence layer output (model rewritten by writeTenn), not the printer.
// For these fixtures the model layer reorders or normalizes the tree, so the printer output
// legitimately differs; every other fixture that has a saved golden must match the printer.
var persistenceNormalized = map[string]bool{
	"bom-leading": true, "calc-arith": true, "calc-siblings": true, "calc-template": true,
	"calc-utils": true, "layout-chain": true, "layout-cycle": true, "layout-disconnected": true,
	"layout-single": true, "layout-tree": true, "link-and-style": true, "persist-items": true,
	"persist-links-dup": true, "persist-nested": true, "persist-order": true, "persist-save-loss": true,
}

func TestGolden(t *testing.T) {
	for _, f := range fixtureNames(t) {
		base := strings.TrimSuffix(f, ".tenn")
		t.Run(base, func(t *testing.T) {
			source := readSource(t, f)
			goldenData, err := os.ReadFile(filepath.Join(fixturesDir, base+".parse.json"))
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(goldenData, &want); err != nil {
				t.Fatal(err)
			}
			got := buildGolden(source)
			if d := firstDiff("", roundTrip(t, got), want); d != "" {
				t.Fatalf("parse golden differs, first difference %s", d)
			}

			saved, err := os.ReadFile(filepath.Join(fixturesDir, base+".saved.tenn"))
			if os.IsNotExist(err) || persistenceNormalized[base] {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got["printed"] != string(saved) {
				t.Fatalf("printer output differs from %s.saved.tenn", base)
			}
		})
	}
}
