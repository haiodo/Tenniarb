package tenn

import (
	"reflect"
	"testing"
)

func TestParseAndPrint(t *testing.T) {
	tree, parser := Parse("element \"Root\" {\n  item First {\n    pos 10 20\n  }\n}")
	if parser.Errors.HasErrors() {
		t.Fatalf("errors %v", parser.Errors.Errors)
	}
	if got, want := ToStr(tree, 0, false), "element \"Root\" {\n    item First {\n        pos 10 20\n    }\n}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ToStr(tree, 0, true), "element Root {\n    item First {\n        pos 10 20\n    }\n}"; got != want {
		t.Errorf("clean: got %q, want %q", got, want)
	}
}

func TestNodeAccessors(t *testing.T) {
	tree, _ := Parse("a { pos 10 20\n name \"x y\"\n flag true }")
	block := tree.GetChild(0, 1)
	if block == nil || block.Kind != KindBlockExpr {
		t.Fatalf("block = %+v", block)
	}
	if v, ok := block.GetNamedElement("pos").GetInt(1); !ok || v != 10 {
		t.Errorf("pos = %d, %v", v, ok)
	}
	if got := block.GetValueString("name", ""); got != "x y" {
		t.Errorf("name = %q", got)
	}
	if !block.GetValueBool("flag", false) {
		t.Error("flag")
	}
	if got := block.GetValueInt("missing", 7); got != 7 {
		t.Errorf("missing = %d", got)
	}
	if got, _ := tree.GetIdent(0, 0); got != "a" {
		t.Errorf("ident = %q", got)
	}
	if tree.GetChild(0, 9) != nil {
		t.Error("out of range child must be nil")
	}
}

func TestNodeFloat(t *testing.T) {
	tests := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1.5", 1.5, true}, {"-.5", -0.5, true}, {"5.", 5, true}, {"1e3", 1000, true},
		{"0.1", float64(float32(0.1)), true}, {"abc", 0, false}, {"1.2.3", 0, false},
	}
	for _, tc := range tests {
		n := NewCommand("k", NewStrNode(tc.in))
		got, ok := n.GetFloat(1)
		if ok != tc.ok || got != tc.want {
			t.Errorf("GetFloat(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestBuildersPrint(t *testing.T) {
	cmd := NewCommand("el", NewStrNode("A"), NewBlockExpr(NewCommand("pos", NewIntNode(1), NewIntNode(2))))
	if got, want := ToStr(cmd, 0, false), "el \"A\" {\n    pos 1 2\n}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := ToStr(NewIdent("x"), 0, false); got != "x" {
		t.Errorf("got %q", got)
	}
}

func TestSameParseRegardlessOfFactory(t *testing.T) {
	src := "element Root {\n  item A {\n    pos 1 2\n  }\n}"
	p := NewParser(func(s string) LexerProtocol { return NewLexer(s) })
	tree, _ := Parse(src)
	if ToStr(p.Parse(src), 0, false) != ToStr(tree, 0, false) {
		t.Error("trees differ")
	}
}

func TestParserErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		codes   []ErrorCode
		message string
	}{
		{"unclosed block", "a { b", []ErrorCode{CodeWrongBlockTerminator}, "Wrong statements terminator"},
		{"unclosed string", `a "abc`, []ErrorCode{CodeEndOfFileDuringStringRead}, "Unclosed string terminal"},
		{"invalid command start", "1 a", []ErrorCode{CodeInvalidCommandStart}, "Invalid command start symbol: 1 "},
		{"unclosed expression", "a $(1", []ErrorCode{CodeWrongBlockTerminator}, "Unclosed expression terminal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, parser := Parse(tc.src)
			errs := parser.Errors.Errors
			var codes []ErrorCode
			for _, e := range errs {
				codes = append(codes, e.ErrorCode)
			}
			if tc.name == "unclosed expression" || tc.name == "unclosed string" {
				codes = codes[:1] // later errors depend on the truncated token stream
			}
			if !reflect.DeepEqual(codes, tc.codes) || errs[0].Message != tc.message {
				t.Errorf("got %v %q, want %v %q", codes, errs[0].Message, tc.codes, tc.message)
			}
		})
	}
}

func TestParseEmptySource(t *testing.T) {
	tree, parser := Parse("")
	if parser.Errors.HasErrors() || tree.Count() != 0 {
		t.Errorf("errors %v, count %d", parser.Errors.Errors, tree.Count())
	}
}

func TestRemoveNamedKeepsLookup(t *testing.T) {
	b := NewBlockExpr(NewCommand("a", NewIntNode(1)), NewCommand("b", NewIntNode(2)))
	if !b.RemoveNamed("a") || b.Count() != 1 {
		t.Fatalf("count %d", b.Count())
	}
	if b.GetNamedElement("a") == nil { // Swift quirk: the lookup table keeps the entry
		t.Error("lookup entry removed")
	}
}

func TestCommandEnd(t *testing.T) {
	src := "item \"a b\" { // c\n  title \"x\\\"y\" ; w 10\n}\nuse a.b as c\nlink A B { display arrow }  \nitem \"é\" {}\nfoo $(1+2) ${z} %{md} @(img)\nbar { }\n"
	ast, p := Parse(src)
	if p.Errors.HasErrors() {
		t.Fatal(p.Errors.Errors)
	}
	want := []string{
		"item \"a b\" { // c\n  title \"x\\\"y\" ; w 10\n}",
		"use a.b as c",
		"link A B { display arrow }",
		"item \"é\" {}",
		"foo $(1+2) ${z} %{md} @(img)",
		"bar { }",
	}
	for i, c := range ast.Children {
		if got := src[c.Token.Pos:c.End]; got != want[i] {
			t.Errorf("command %d = %q, want %q", i, got, want[i])
		}
	}
	block := ast.Children[0].Children[2]
	if got := src[block.Children[0].Token.Pos:block.Children[0].End]; got != `title "x\"y"` {
		t.Errorf("nested = %q", got)
	}
}
