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
	KSlot // slot definition in a component; Content is the fallback
)

// Range is the byte range [Start, End) of a command in the text of its file.
type Range struct{ Start, End int }

func (k Kind) String() string {
	return [...]string{"file", "element", "item", "link", "use", "slot"}[k]
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
	Kind    OvKind
	Pos     Pos
	Range   Range // the whole command with its block
	Body    Range // the block of the command, zero when it has none
	Prop    *Prop
	Node    *Node
	Path    string // OvAt, OvReplace
	PathArg Range  // the path in the text of its file
	Slot    string // OvSlot
	Block   []*Override
}

// Node is a node of the file tree: what was written, before use is expanded.
type Node struct {
	Kind   Kind
	Name   string
	Key    string // Name normalized, the segment of a path
	Pos    Pos
	Range  Range // the whole command with its block; zero for a file root
	Body   Range // the block of the command from { to after }; zero when it has none
	Parent *Node // scope: lookups go up this chain, and on from a file root to its mount point (resolve.go)

	Props   []*Prop
	Content []*Node // item, link, use, slot in file order
	Elems   []*Node // nested elements

	Mount    *Node       // KElement with --from-file: the root of the file that holds its content
	From, To string      // KLink
	Path     string      // KUse
	Block    []*Override // KUse

	// Styles are the named styles of an element from its styles block: a set of properties for the node
	// that names it with use-style, and for the kids its blocks name.
	Styles map[string]*Style

	// StyleGroups are the groups the same block declares, in declaration order: the styles the inspector
	// offers as a choice among them (one-of) or as flags (any-of).
	StyleGroups []StyleGroup

	// The arguments of the command in the text of its file, delimiters included; zero when it has none.
	NameArg        Range // the name, for a use the one after as
	PathArg        Range // KUse
	FromArg, ToArg Range // KLink
}

// Style is one named style: properties of the node that names it, and of the kids its blocks name.
type Style struct {
	Props []*Prop
	Kids  []StyleKid
}

// StyleKid is the part of a style for a kid with that key, as a CSS descendant selector.
type StyleKid struct {
	Key   string
	Style *Style
}

// StyleGroup is one group declaration of a styles block, "name one-of a b c" or "name any-of a b": the
// styles a node of the element may name, in the order they were written.
type StyleGroup struct {
	Name   string   // normalized, as a style name
	AnyOf  bool     // any number of the styles; otherwise a one-of group takes one of them
	Styles []string // normalized style names
}

// Nodes lists the elements, items and uses written directly in n, in file order: the nodes a path
// can name, without what a use passes in its block.
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

// named lists what a path segment can pick under n: elements, items and uses.
func (n *Node) named() []*Node {
	out := make([]*Node, 0, len(n.Elems)+len(n.Content))
	out = append(out, n.Elems...)
	for _, c := range n.Content {
		if c.Kind == KItem || c.Kind == KUse {
			out = append(out, c)
		}
	}
	return out
}
