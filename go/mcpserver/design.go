package mcpserver

import (
	"errors"
	"fmt"
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
	Node    string  `json:"node,omitempty" jsonschema:"dotted path of the node from the file root, as get_state prints it"`
	Parent  string  `json:"parent,omitempty" jsonschema:"insert: dotted path of the parent; empty for the file root"`
	Name    string  `json:"name,omitempty" jsonschema:"set: the property name"`
	Value   string  `json:"value,omitempty" jsonschema:"set: the arguments of the property as written after the name; empty removes it"`
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
	if live {
		res, err := s.host.live.text(abs, true)
		if err != nil {
			return "", err
		}
		text = res.Text
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
	tree, err := scene.LoadAuto(s.root, rel, map[string]string{rel: text})
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
		cur, err := s.host.live.text(abs, true)
		if err != nil {
			return "", err
		}
		var res servicewire.ApplyResult
		if err := d.prov.call(servicewire.MethodApply, servicewire.ApplyArgs{Path: d.path, Base: cur.Version, Text: next}, &res); err != nil {
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
			if n.Kind != scene.KItem && n.Kind != scene.KElement && n.Kind != scene.KFile {
				return span{}, "", errors.New("insert takes an item or an element as its parent")
			}
			parent = n
		}
		at, ok := blockEnd(parent)
		if !ok {
			return span{}, "", errors.New("the parent has no block to insert into")
		}
		cmd := strings.TrimSpace(op.Text)
		if cmd == "" {
			return span{}, "", errors.New("text is empty")
		}
		ind, unit := indentOf(text, parent.Range.Start), indentUnitOf(text)
		if parent.Kind == scene.KFile {
			ind, unit = "", indentUnitOf(text)
		}
		body := strings.ReplaceAll(cmd, "\n", "\n"+ind+unit)
		return span{at, at, "\n" + ind + unit + body}, "into " + nameOf(parent), nil
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
		at, ok := blockEnd(n)
		if !ok {
			return span{}, "", errors.New("the node has no block to set a property in")
		}
		cmd := strings.TrimSpace(name + " " + value)
		if pr := ownPropOf(n, name); pr != nil {
			if strings.TrimSpace(value) == "" {
				from, to := lineSpan(text, pr.Range.Start, pr.Range.End)
				if to-from == len(text[from:to]) && !strings.Contains(text[from:to], ";") {
					return span{from, to, ""}, name + " of " + nameOf(n) + " removed", nil
				}
				return span{pr.Range.Start, pr.Range.End, ""}, name + " of " + nameOf(n) + " removed", nil
			}
			return span{pr.Range.Start, pr.Range.End, cmd}, name + " of " + nameOf(n), nil
		}
		if strings.TrimSpace(value) == "" {
			return span{}, "", errors.New("the node has no property " + name)
		}
		ind, unit := indentOf(text, n.Range.Start), indentUnitOf(text)
		return span{at, at, "\n" + ind + unit + cmd}, name + " of " + nameOf(n) + " added", nil
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
		at := strings.Index(text, op.Find)
		if at < 0 {
			return span{}, "", errors.New("find text not found")
		}
		return span{at, at + len(op.Find), op.Replace}, fmt.Sprintf("%d places", strings.Count(text, op.Find)), nil
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

// ownPropOf is the property command of name written on n itself: in its own arguments, or in the block
// of a use, where an override of the root instance is written.
func ownPropOf(n *scene.Node, name string) *scene.Prop {
	var found *scene.Prop
	for _, p := range n.Props {
		if p.Name == name {
			found = p
		}
	}
	for _, ov := range n.Block {
		if ov.Kind == scene.OvProp && ov.Prop != nil && ov.Prop.Name == name {
			found = ov.Prop
		}
	}
	return found
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

// num prints a coordinate without trailing zeros.
func num(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}
