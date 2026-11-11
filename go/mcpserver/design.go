package mcpserver

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// DesignOp is one operation of the design tool: an edit of a node by its path, not by the text around it.
type DesignOp struct {
	Op      string  `json:"op" jsonschema:"set, move, insert, delete or replace_all"`
	Node    string  `json:"node,omitempty" jsonschema:"dotted path of the node from the file root, as get_state prints it; replace_all searches only this node"`
	Parent  string  `json:"parent,omitempty" jsonschema:"insert: dotted path of the parent; empty for the file root"`
	Name    string  `json:"name,omitempty" jsonschema:"set: the property name"`
	Value   string  `json:"value,omitempty" jsonschema:"set: the arguments of the property as written after the name; empty removes it"`
	Index   *int    `json:"index,omitempty" jsonschema:"set: which of the properties of that name to change, 0-based, since path and shadow repeat; default the last one"`
	Text    string  `json:"text,omitempty" jsonschema:"insert: the commands to add, one or more lines"`
	Dx      float64 `json:"dx,omitempty" jsonschema:"move: change of pos x"`
	Dy      float64 `json:"dy,omitempty" jsonschema:"move: change of pos y, y up"`
	Find    string  `json:"find,omitempty" jsonschema:"replace_all: the text to find"`
	Replace string  `json:"replace,omitempty" jsonschema:"replace_all: the text to put in its place"`
}

// span is a replacement of [start, end) of the text of one file.
type span struct {
	start, end int
	text       string
}

// design applies ops to a file as one change: a path addresses the node, so no text has to match.
func (s *Server) design(file string, ops []DesignOp) (string, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	if len(ops) == 0 {
		return "", errors.New("ops is empty")
	}
	s.host.editMu.Lock()
	defer s.host.editMu.Unlock()

	d := s.openForEdit(abs)
	live := d != nil
	var text string
	var base int64
	if live {
		res, err := s.host.live.text(abs, true)
		if err != nil {
			return "", err
		}
		text, base = res.Text, res.Version
	} else {
		b, err := os.ReadFile(abs)
		if err != nil {
			return "", err
		}
		text = string(b)
	}
	before, _, err := s.diagnose(rel)
	if err != nil {
		return "", err
	}
	// The tree of the entry document, not of this file alone: a library names what its entry mounts,
	// and a path through a mount is what design addresses.
	texts, _, err := s.liveTexts()
	if err != nil {
		return "", err
	}
	if texts == nil {
		texts = map[string]string{}
	}
	texts[rel] = text
	tree, err := scene.LoadAuto(s.root, rel, texts)
	if err != nil {
		return "", err
	}
	var spans []span
	var lines []string
	for i, op := range ops {
		sp, msg, err := designSpan(tree, rel, text, op)
		if err != nil {
			return "", fmt.Errorf("op %d (%s): %v", i+1, op.Op, err)
		}
		spans = append(spans, sp)
		lines = append(lines, fmt.Sprintf("%d. %s: %s", i+1, op.Op, msg))
	}
	next, err := applySpans(text, spans)
	if err != nil {
		return "", err
	}
	if e, bad := parseError(next); bad {
		return "", fmt.Errorf("nothing written: parse error at %s:%d:%d: %s", rel, e.Line+1, e.Col, e.Message)
	}
	after, _, err := s.check(rel, next, before)
	if err != nil {
		return "", err
	}
	head := fmt.Sprintf("ok: %d operations in %s\n%s\n", len(ops), rel, strings.Join(lines, "\n"))
	if live {
		var res servicewire.ApplyResult
		if err := d.prov.call(servicewire.MethodApply, servicewire.ApplyArgs{Path: d.path, Base: base, Text: next}, &res); err != nil {
			return "", fmt.Errorf("nothing changed: Tenniarb did not apply the change: %v", err)
		}
		s.noteEdit(abs, rel, tree, text, next)
		return head + "changed in Tenniarb as one undo step, not saved\n" + summary(after, true, rel), nil
	}
	if err := writeAtomic(abs, next); err != nil {
		return "", err
	}
	return head + "written\n" + summary(after, true, rel), nil
}

// inFile refuses a node written in another file: its offsets belong to the text of that file.
func inFile(n *scene.Node, rel string) error {
	if n != nil && n.Kind != scene.KFile && n.Pos.File != "" && n.Pos.File != rel {
		return fmt.Errorf("%q is written in %s: run design on that file", n.Key, n.Pos.File)
	}
	return nil
}

// designSpan is the replacement one op makes, and the line the result shows.
func designSpan(tree *scene.Tree, rel, text string, op DesignOp) (span, string, error) {
	switch op.Op {
	case "insert":
		parent := tree.File(rel)
		if op.Parent != "" {
			n, err := nodeAt(tree, rel, op.Parent)
			if err != nil {
				return span{}, "", err
			}
			if err := inFile(n, rel); err != nil {
				return span{}, "", err
			}
			if n.Kind != scene.KItem && n.Kind != scene.KElement && n.Kind != scene.KUse && n.Kind != scene.KFile {
				return span{}, "", errors.New("insert takes an item, an element or a use as its parent")
			}
			parent = n
		}
		cmd := strings.TrimSpace(op.Text)
		if cmd == "" {
			return span{}, "", errors.New("text is empty")
		}
		sp, ok := blockInsert(text, parent, cmd)
		if !ok {
			if parent.Kind == scene.KUse {
				return span{}, "", errors.New("the use has no block to insert into: write { } after it first")
			}
			return span{}, "", errors.New("the parent has no block to insert into")
		}
		return sp, "into " + nameOf(parent), nil
	case "set", "move":
		n, err := nodeAt(tree, rel, op.Node)
		if err != nil {
			return span{}, "", err
		}
		if err := inFile(n, rel); err != nil {
			return span{}, "", err
		}
		name, value := op.Name, op.Value
		if op.Op == "move" {
			x, y := propNums(n, "pos")
			name, value = "pos", num(x+op.Dx)+" "+num(y+op.Dy)
		}
		if name == "" {
			return span{}, "", errors.New("name is empty")
		}
		cmd := strings.TrimSpace(name + " " + value)
		props := ownPropsOf(n, name)
		pr, at, err := pickProp(n, name, props, op.Index)
		if err != nil {
			return span{}, "", err
		}
		where := fmt.Sprintf("%s of %s (index %d of %d)", name, nameOf(n), at, len(props))
		if pr != nil {
			if strings.TrimSpace(value) == "" {
				from, to := lineSpan(text, pr.Range.Start, pr.Range.End)
				if to-from == len(text[from:to]) && !strings.Contains(text[from:to], ";") {
					return span{from, to, ""}, where + " removed", nil
				}
				return span{pr.Range.Start, pr.Range.End, ""}, where + " removed", nil
			}
			return span{pr.Range.Start, pr.Range.End, cmd}, where, nil
		}
		if strings.TrimSpace(value) == "" {
			return span{}, "", errors.New("the node has no property " + name)
		}
		sp, ok := blockInsert(text, n, cmd)
		if !ok {
			return span{}, "", errors.New("the node has no block to set a property in")
		}
		return sp, name + " of " + nameOf(n) + " added", nil
	case "delete":
		n, err := nodeAt(tree, rel, op.Node)
		if err != nil {
			return span{}, "", err
		}
		if err := inFile(n, rel); err != nil {
			return span{}, "", err
		}
		if n.Kind == scene.KFile || n.Kind == scene.KElement && n.Parent == nil {
			return span{}, "", errors.New("delete takes an item or a use, not the file")
		}
		from, to := lineSpan(text, n.Range.Start, n.Range.End)
		return span{from, to, ""}, nameOf(n) + " deleted", nil
	case "replace_all":
		if op.Find == "" {
			return span{}, "", errors.New("find is empty")
		}
		area, from, where := text, 0, "the file"
		if op.Node != "" { // the node bounds the search: a plain find would change every frame
			n, err := nodeAt(tree, rel, op.Node)
			if err != nil {
				return span{}, "", err
			}
			if err := inFile(n, rel); err != nil {
				return span{}, "", err
			}
			area, from, where = text[n.Range.Start:n.Range.End], n.Range.Start, nameOf(n)
		}
		at := strings.Index(area, op.Find)
		if at < 0 {
			return span{}, "", fmt.Errorf("find text not found in %s", where)
		}
		return span{from + at, from + at + len(op.Find), op.Replace},
			fmt.Sprintf("%d places in %s", strings.Count(area, op.Find), where), nil
	}
	return span{}, "", fmt.Errorf("unknown op %q", op.Op)
}

// applySpans puts the replacements into text, right to left; overlapping ones are refused.
func applySpans(text string, spans []span) (string, error) {
	sorted := append([]span(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start > sorted[j].start })
	out := text
	last := len(text) + 1
	for _, sp := range sorted {
		if sp.start < 0 || sp.end < sp.start || sp.end > len(text) {
			return "", fmt.Errorf("range %d..%d is outside the text", sp.start, sp.end)
		}
		if sp.end > last {
			return "", errors.New("two operations change the same text: the second needs the result of the first")
		}
		out = out[:sp.start] + sp.text + out[sp.end:]
		last = sp.start
	}
	return out, nil
}

// blockEnd is the offset just before the closing brace of the block of n.
func blockEnd(n *scene.Node) (int, bool) {
	if n.Body.Start == 0 || n.Body.End <= n.Body.Start {
		return 0, false
	}
	return n.Body.End - 1, true
}

// blockInsert is the span that adds cmd as the last lines of the block of n; the closing brace keeps a
// line of its own, or a delete of a sibling would take it along with the line.
func blockInsert(text string, n *scene.Node, cmd string) (span, bool) {
	unit := indentUnitOf(text)
	if n.Kind == scene.KFile { // the root has no block: the text of the file is the block
		at := len(strings.TrimRight(text, " \t\r\n"))
		lead := ""
		if at > 0 {
			lead = "\n"
		}
		return span{at, len(text), lead + strings.ReplaceAll(cmd, "\n", "\n"+unit) + "\n"}, true
	}
	at, ok := blockEnd(n)
	if !ok {
		return span{}, false
	}
	ind := indentOf(text, n.Range.Start)
	body := strings.ReplaceAll(cmd, "\n", "\n"+ind+unit)
	from, lead := at, "\n"
	if lineStart := strings.LastIndexByte(text[:at], '\n') + 1; strings.TrimLeft(text[lineStart:at], " \t") == "" {
		from, lead = lineStart, ""
	}
	return span{from, at, lead + ind + unit + body + "\n" + ind}, true
}

// ownPropsOf is every property command of name written on n itself, in the order they stand: the own
// arguments, then the overrides in the block of a use.
func ownPropsOf(n *scene.Node, name string) []*scene.Prop {
	var out []*scene.Prop
	for _, p := range n.Props {
		if p.Name == name {
			out = append(out, p)
		}
	}
	for _, ov := range n.Block {
		if ov.Kind == scene.OvProp && ov.Prop != nil && ov.Prop.Name == name {
			out = append(out, ov.Prop)
		}
	}
	return out
}

// ownPropOf is the last property command of name written on n itself: the one in effect.
func ownPropOf(n *scene.Node, name string) *scene.Prop {
	if props := ownPropsOf(n, name); len(props) > 0 {
		return props[len(props)-1]
	}
	return nil
}

// pickProp is the property an op addresses: the one at index, or the last one when index is nil. An index
// outside the list is an error that says how many properties of the name the node has.
func pickProp(n *scene.Node, name string, props []*scene.Prop, index *int) (*scene.Prop, int, error) {
	i := len(props) - 1
	if index != nil {
		i = *index
	}
	if i < 0 || i >= len(props) {
		if index == nil {
			return nil, -1, nil // set adds the property
		}
		return nil, i, fmt.Errorf("%s has %d properties named %s: no index %d", nameOf(n), len(props), name, i)
	}
	return props[i], i, nil
}

// propNums reads the two numbers of a property, zero when it has none.
func propNums(n *scene.Node, name string) (x, y float64) {
	pr := ownPropOf(n, name)
	if pr == nil {
		return 0, 0
	}
	v := pr.Values()
	if len(v) != 2 {
		return 0, 0
	}
	return parseFloat(v[0]), parseFloat(v[1])
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// nameOf is the name a message shows: the file for a file root, else the key of the node.
func nameOf(n *scene.Node) string {
	if n.Kind == scene.KFile && n.Pos.File != "" {
		return n.Pos.File
	}
	return n.Key
}

// lineSpan widens [start, end) to whole lines when the node sits alone on them.
func lineSpan(text string, start, end int) (int, int) {
	lineFrom := strings.LastIndexByte(text[:start], '\n') + 1
	if strings.TrimSpace(text[lineFrom:start]) != "" {
		return start, end
	}
	lineTo := strings.IndexByte(text[end:], '\n')
	if lineTo < 0 {
		return lineFrom, end
	}
	return lineFrom, end + lineTo + 1
}

// indentOf is the white space that starts the line of off.
func indentOf(text string, off int) string {
	from := strings.LastIndexByte(text[:off], '\n') + 1
	i := from
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return text[from:i]
}

// indentUnitOf is the indentation the file uses for one level.
func indentUnitOf(text string) string {
	for _, l := range strings.SplitN(text, "\n", 200) {
		if t := strings.TrimLeft(l, " \t"); t != "" && len(t) < len(l) {
			return l[:len(l)-len(t)]
		}
	}
	return "  "
}

// num prints a coordinate to 1/100 of a point without trailing zeros.
func num(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}
