package scene

import (
	"sort"
	"strconv"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Built straight from go/tenn nodes. Ceiling: once go/model carries the scene-graph format, an
// adapter from the model to this Node tree replaces the parser input; resolve/expand/build stay.

type Kind uint8

const (
	KFile    Kind = iota // root of a file: acts as an unnamed element
	KElement             // element "Name" { ... }
	KItem
	KLink
	KUse
	KSlot    // slot definition in a component; Content is the fallback
	KMount   // import ... as alias: Target is the root of the imported file
	KVirtual // a dotted alias prefix: "my" of "my.lib"
)

// Range is the byte range [Start, End) of a command in the text of its file.
type Range struct{ Start, End int }

func (k Kind) String() string {
	return [...]string{"file", "element", "item", "link", "use", "slot", "import", "namespace"}[k]
}

// Prop is a command that is not structure: a property of its node, in file order.
type Prop struct {
	Name  string
	Args  []*tenn.Node // arguments after the name, a block excluded
	Block *tenn.Node
	Pos   Pos
	Range Range
}

func (p *Prop) text(i int) (string, bool) {
	if i >= len(p.Args) {
		return "", false
	}
	return p.Args[i].GetIdentText()
}

func (p *Prop) num(i int) (float64, bool) {
	if i >= len(p.Args) {
		return 0, false
	}
	a := p.Args[i]
	if a.Kind != tenn.KindIntLit && a.Kind != tenn.KindFloatLit {
		return 0, false
	}
	s, _ := a.GetIdentText()
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// OvKind is the kind of one command in the block of a use.
type OvKind uint8

const (
	OvProp    OvKind = iota // override of a property of the root
	OvNode                  // item, link or use: goes to the default slot
	OvAt                    // at <path> { ... }
	OvReplace               // replace <path> { nodes }
	OvSlot                  // slot <name> { nodes }
)

type Override struct {
	Kind  OvKind
	Pos   Pos
	Prop  *Prop
	Node  *Node
	Path  string // OvAt, OvReplace
	Slot  string // OvSlot
	Block []*Override
}

// Node is a node of the file tree: what was written, before use is expanded.
type Node struct {
	Kind   Kind
	Name   string
	Key    string // Name normalized, the segment of a path
	Pos    Pos
	Range  Range // the whole command with its block; zero for a file root
	Body   Range // the block of the command from { to after }; zero when it has none
	Parent *Node // scope: lookups go up this chain; stays inside one file

	Props   []*Prop
	Content []*Node // item, link, use, slot in file order
	Elems   []*Node // nested elements
	Mounts  []*Node // imports and their alias prefixes

	Target   *Node       // KMount: the root of the imported file
	From, To string      // KLink
	Path     string      // KUse
	Block    []*Override // KUse
}

// Nodes lists the elements, items and uses written directly in n, in file order: the nodes a path
// can name, without imports and without what a use passes in its block.
func (n *Node) Nodes() []*Node {
	if n.Kind == KUse {
		return nil
	}
	var out []*Node
	out = append(out, n.Elems...)
	for _, c := range n.Content {
		if c.Kind == KItem || c.Kind == KUse {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Range.Start < out[j].Range.Start })
	return out
}

func (n *Node) holdsContent() bool {
	return n.Kind == KFile || n.Kind == KElement || n.Kind == KItem || n.Kind == KSlot
}

func (n *Node) isContainer() bool { return n.Kind == KFile || n.Kind == KElement }

// named lists what a path segment can pick under n: elements, items, uses and import aliases.
func (n *Node) named() []*Node {
	if n.Kind == KMount {
		n = n.Target
	}
	out := make([]*Node, 0, len(n.Elems)+len(n.Content)+len(n.Mounts))
	out = append(out, n.Elems...)
	for _, c := range n.Content {
		if c.Kind == KItem || c.Kind == KUse {
			out = append(out, c)
		}
	}
	return append(out, n.Mounts...)
}
