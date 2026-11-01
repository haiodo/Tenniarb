package optimize

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// analyze loads text as a.tenn of a fresh root and reports the findings for it.
func analyze(t *testing.T, text string) []Finding {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", text)
	if tree == nil {
		t.Fatalf("load: %v", err)
	}
	return Analyze(tree)
}

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	return out
}

func only(t *testing.T, fs []Finding, rule string) []Finding {
	t.Helper()
	var out []Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

// TestRepeat: two blocks of one structure that differ in a name are one component with a parameter;
// blocks that differ in a property or a child are not, and nothing is reported (rule 1).
func TestRepeat(t *testing.T) {
	same := `element "E" {
    item Card {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
    item Other {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
}
`
	fs := analyze(t, same)
	if n := len(only(t, fs, RuleRepeat)); n != 1 {
		t.Fatalf("repeat findings %d, want 1: %v", n, rules(fs))
	}
	r := only(t, fs, RuleRepeat)[0]
	if len(r.Paths) != 2 || r.Line != 2 || r.Col != 5 || !strings.Contains(r.Detail, "the node name") {
		t.Errorf("repeat %+v", r)
	}

	// one child more, one property more, one property less: a different structure, no suggestion
	for _, other := range []string{
		`element "E" {
    item Card {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
    item Other {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
    }
}
`,
		`element "E" {
    item Card {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
    item Other {
        auto-layout column
        gap 8
        padding 16
        align center
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
}
`,
	} {
		// no-merge may warn that the children look alike under other layouts: that is not a repeat
		if fs := analyze(t, other); len(only(t, fs, RuleRepeat)) != 0 || len(only(t, fs, RuleStyles)) != 0 {
			t.Errorf("different structures reported: %v", rules(fs))
		}
	}
}

// TestRepeatParams: a set needing more parameters than structure is not a component: no finding.
func TestRepeatParams(t *testing.T) {
	// every child is a use with its own name and its own style: 9 parameters for 10 commands
	text := `element "E" {
    styles {
        a { color red }
        b { color blue }
    }
    item R1 {
        item c1 { title x; use-style a }
        item c2 { title y; use-style b }
    }
    item R2 {
        item c3 { title z; use-style b }
        item c4 { title w; use-style a }
    }
}
`
	if fs := analyze(t, text); len(fs) != 0 {
		t.Errorf("over-parameterised set reported: %v", rules(fs))
	}
}

// TestStyles: nodes that differ only in a look property are variants; a property outside the style set
// makes them different structures instead (rule 2).
func TestStyles(t *testing.T) {
	text := `element "E" {
    item A { width 40; height 20; color #ff0000 }
    item B { width 40; height 20; color #00ff00 }
}
`
	fs := analyze(t, text)
	if n := len(only(t, fs, RuleStyles)); n != 1 {
		t.Fatalf("styles findings %d, want 1: %v", n, rules(fs))
	}
	s := only(t, fs, RuleStyles)[0]
	if !strings.Contains(s.Detail, "color") || !strings.Contains(s.Detail, "width 40") || len(s.Paths) != 2 {
		t.Errorf("styles %+v", s)
	}

	// the second node has a layout of its own: not a variant of the first
	other := `element "E" {
    item A { width 40; height 20; color #ff0000 }
    item B { width 40; height 20; color #00ff00; auto-layout row }
}
`
	if fs := analyze(t, other); len(fs) != 0 {
		t.Errorf("different structures reported: %v", rules(fs))
	}
}

// TestLiteral: one set of look properties repeated by many nodes gets a name even when the nodes have
// nothing else in common; two nodes are too few (rule 3).
func TestLiteral(t *testing.T) {
	text := `element "E" {
    item A { color #112233; font-size 11; padding 4 }
    item B { color #112233; font-size 11; padding 4; pos 10 0 }
    item C { color #112233; font-size 11; padding 4; title x }
}
`
	fs := analyze(t, text)
	if n := len(only(t, fs, RuleLiteral)); n != 1 {
		t.Fatalf("literal findings %d, want 1: %v", n, rules(fs))
	}
	if l := only(t, fs, RuleLiteral)[0]; len(l.Paths) != 3 || !strings.Contains(l.Detail, "font-size 11") {
		t.Errorf("literal %+v", l)
	}

	tooFew := `element "E" {
    item A { color #112233; font-size 11; padding 4 }
    item B { color #112233; font-size 11; padding 4; pos 10 0 }
}
`
	if fs := analyze(t, tooFew); len(fs) != 0 {
		t.Errorf("two nodes reported: %v", rules(fs))
	}
}

// TestDead: a style nobody names, a group nobody chooses from, a pos a flow layout ignores, an override
// that repeats the component or has nothing in it (rule 4).
func TestDead(t *testing.T) {
	text := `element "Card" {
    item Root { width 100; height 40 }
}
element "E" {
    styles {
        dead { color red }
        live { color blue }
        alone { color green }
        pick one-of live alone
        pick2 one-of alone
    }
    item A {
        auto-layout column
        item B { use-style live }
        item Moved { pos 10 10; width 5 }
        use Card as one { at Root { width 100 } }
        use Card as two { at Root { } }
    }
}
`
	fs := analyze(t, text)
	if n := len(only(t, fs, "dead-style")); n != 1 {
		t.Errorf("dead-style %d, want 1 (dead only): %v", n, rules(fs))
	}
	if n := len(only(t, fs, "dead-group")); n != 1 {
		t.Errorf("dead-group %d, want 1 (pick2 only): %v", n, rules(fs))
	}
	if n := len(only(t, fs, "dead-pos")); n != 1 {
		t.Errorf("dead-pos %d, want 1 (Moved only): %v", n, rules(fs))
	}
	if n := len(only(t, fs, "dead-override")); n != 2 {
		t.Errorf("dead-override %d, want 2 (the same value and the empty block): %v", n, rules(fs))
	}
	for _, f := range fs {
		if f.Rule == "dead-style" && !strings.Contains(f.Detail, `"dead"`) {
			t.Errorf("wrong style reported dead: %s", f.Detail)
		}
		if f.Rule == "dead-group" && !strings.Contains(f.Detail, `"pick2"`) {
			t.Errorf("wrong group reported dead: %s", f.Detail)
		}
	}
}

// TestDeadDefaultStyles: "item" and "line" are the defaults of the classic layer, not dead styles.
func TestDeadDefaultStyles(t *testing.T) {
	text := `element "E" {
    styles {
        item { font-size 14 }
        line { color #888888 }
    }
    item A
}
`
	if fs := analyze(t, text); len(fs) != 0 {
		t.Errorf("default styles reported: %v", rules(fs))
	}
}

// TestNoMerge: one structure under two different layouts is not one component, and no component is
// suggested; the set is named instead so the reader does not merge it (rule 5).
func TestNoMerge(t *testing.T) {
	text := `element "E" {
    item Row {
        auto-layout row
        item Cell1 { width 10; height 10; title a }
        item Cell2 { width 10; height 10; title b }
    }
    item Col {
        auto-layout column
        item Cell3 { width 10; height 10; title c }
        item Cell4 { width 10; height 10; title d }
    }
}
`
	fs := analyze(t, text)
	if n := len(only(t, fs, RuleRepeat)); n != 0 {
		t.Errorf("component suggested across layouts: %v", rules(fs))
	}
	if n := len(only(t, fs, RuleNoMerge)); n != 1 {
		t.Fatalf("no-merge %d, want 1: %v", n, rules(fs))
	}
	if paths := only(t, fs, RuleNoMerge)[0].Paths; len(paths) != 4 {
		t.Errorf("no-merge paths %v", paths)
	}
}

// TestPositions: positions come from the text, not from scene.Pos.Line, which is one line high for every
// // comment above the node.
func TestPositions(t *testing.T) {
	text := `// one
// two
element "E" {
    item A { width 40; height 20; color #ff0000 }
    item B { width 40; height 20; color #00ff00 }
}
`
	fs := analyze(t, text)
	if len(fs) != 1 {
		t.Fatalf("findings %v", rules(fs))
	}
	if f := fs[0]; f.Line != 4 || f.Col != 5 {
		t.Errorf("position %d:%d, want 4:5", f.Line, f.Col)
	}
}

// TestJSON: -json is a valid array whose entries carry every field of the report.
func TestJSON(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.tenn")
	text := `element "E" {
    item Card {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
    item Other {
        auto-layout column
        gap 8
        padding 16
        item Head { font-size 18 }
        item Body { font-size 13 }
    }
}
`
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"-json", "-root", dir, file}, &out, &errOut); code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var fs []map[string]any
	if err := json.Unmarshal(out.Bytes(), &fs); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if len(fs) != 1 {
		t.Fatalf("findings %d, want 1: %s", len(fs), out.String())
	}
	for _, key := range []string{"rule", "file", "line", "col", "paths", "detail", "saving"} {
		if _, ok := fs[0][key]; !ok {
			t.Errorf("field %q missing: %v", key, fs[0])
		}
	}
	if fs[0]["rule"] != RuleRepeat || fs[0]["file"] != "a.tenn" || fs[0]["line"].(float64) != 2 {
		t.Errorf("entry %v", fs[0])
	}
	if paths, ok := fs[0]["paths"].([]any); !ok || len(paths) != 2 {
		t.Errorf("paths %v", fs[0]["paths"])
	}

	// the text form says the same, one finding per line
	out.Reset()
	if code := Run([]string{"-root", dir, file}, &out, &errOut); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if line := out.String(); !strings.Contains(line, "a.tenn:2:5: repeat: ") || !strings.Contains(line, "1 files, 1 findings") {
		t.Errorf("text output:\n%s", line)
	}
}

// TestJSONEmpty: no findings is an empty array, not null.
func TestJSONEmpty(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.tenn")
	if err := os.WriteFile(file, []byte("element \"E\" {\n    item A\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"-json", "-root", dir, file}, &out, &errOut); code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("-json with no findings = %s", got)
	}
}

// TestUsage: no file and a missing file are usage errors.
func TestUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code != ExitUsage || !strings.Contains(errOut.String(), "usage: tenniarb suggest") {
		t.Errorf("no args: exit %d, %s", code, errOut.String())
	}
	errOut.Reset()
	if code := Run([]string{"nope.tenn"}, &out, &errOut); code != ExitUsage {
		t.Errorf("missing file: exit %d", code)
	}
}

// TestExamples is the quality gate: the report on the three real files. button-styles.tenn is one
// deliberate component with styles, so it may only offer a style for the repeated captions; the
// onboarding flow has no repeat at all; every finding anywhere has at least two nodes and a real
// position.
func TestExamples(t *testing.T) {
	for _, tc := range []struct {
		file  string
		check func(*testing.T, []Finding)
	}{
		{"examples/onboarding-flow.tenn", func(t *testing.T, fs []Finding) {
			if len(fs) != 0 {
				t.Errorf("false positives: %v", rules(fs))
			}
		}},
		{"examples/button-styles.tenn", func(t *testing.T, fs []Finding) {
			for _, f := range fs {
				if f.Rule == RuleRepeat || strings.HasPrefix(f.Rule, RuleDead) {
					t.Errorf("%s: %s", f.Rule, f.Detail)
				}
			}
			styles := only(t, fs, RuleStyles)
			if len(styles) != 1 || !strings.Contains(styles[0].Detail, "title") || !strings.Contains(styles[0].Detail, "width 88") {
				t.Errorf("the 8 captions are not offered a style: %v", fs)
			}
		}},
		{"tenniarb.tenn", func(t *testing.T, fs []Finding) {
			if len(fs) == 0 {
				t.Error("no findings in the big file")
			}
		}},
	} {
		if _, err := os.Stat(filepath.Join("..", "..", tc.file)); err != nil {
			t.Skipf("%s: %v", tc.file, err)
		}
		tree, err := scene.Load(filepath.Join("..", ".."), tc.file)
		if tree == nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		fs := Analyze(tree)
		tc.check(t, fs)
		for _, f := range fs {
			if f.File != tc.file || f.Line < 1 || f.Col < 1 || f.Detail == "" || f.Saving == "" {
				t.Errorf("%s: bad finding %+v", tc.file, f)
			}
			if (f.Rule == RuleRepeat || f.Rule == RuleStyles || f.Rule == RuleLiteral) && len(f.Paths) < 2 {
				t.Errorf("%s: %s on one node: %+v", tc.file, f.Rule, f)
			}
			if f.Rule == RuleRepeat || f.Rule == RuleStyles {
				seen := map[string]bool{}
				for _, p := range f.Paths {
					if seen[p] {
						t.Errorf("%s: the same node twice: %s", tc.file, p)
					}
					seen[p] = true
				}
			}
		}
	}
}
