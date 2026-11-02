// Package optimize suggests layout tidy-ups for a loaded .tenn file: repeated structure that wants a
// component, repeated looks that want a named style, and dead commands (spec 5b, 5c). Analyze reports
// them; Apply builds the text edits of one finding, and the caller writes them. A component (repeat) is
// applied as one element and a use per node, unless the values that differ are not written in the text;
// the warning not to merge is reported only.
package optimize

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Rules, most valuable first: a component removes structure, a style removes a look, a dead command
// removes nothing a reader would miss.
const (
	RuleRepeat  = "repeat"   // one structure, other literals: element + use with parameters (5c)
	RuleStyles  = "styles"   // one structure, only the look or the text differs: styles + use-style (5b)
	RuleLiteral = "literal"  // one set of look properties repeated by many nodes
	RuleDead    = "dead-"    // prefix of the dead sub-rules: a style nobody names, an override that changes nothing
	RuleNoMerge = "no-merge" // false duplicates: one structure under different layouts, keep them apart
)

// Exit codes of Run.
const (
	ExitOK    = 0
	ExitUsage = 2
)

const usage = "usage: tenniarb suggest [-json] [-root DIR] FILE..."

// Finding is one suggestion. File, Line and Col are the position of the first node of the finding;
// File is a slash path relative to the workspace root, as every scene position is.
type Finding struct {
	Rule   string   `json:"rule"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Col    int      `json:"col"`
	Paths  []string `json:"paths"`  // the nodes the finding is about, the first node first, each with its position
	Detail string   `json:"detail"` // what repeats, and why it is worth changing
	Saving string   `json:"saving"` // rough gain of applying it

	score int // sort key within a rule, not printed

	// What Apply needs to build the change; not printed and not part of the JSON report.
	nodes []*scene.Node // repeat: the nodes that become use; styles, literal: the nodes that get use-style
	props []propVal     // styles, literal: the look the style holds
	comp  *component    // repeat: the element the finding makes; nil when it cannot be applied
	why   string        // repeat: why Apply refuses it, empty when it can
	elem  *scene.Node   // dead-style, dead-group: the node whose styles block holds the name
	name  string        // dead-style, dead-group: what to delete from that block
	del   *scene.Range  // dead-pos, dead-override: the command to delete
}

// param is one parameter of the element a repeat finding makes: where the first node writes its value,
// that value as the default of the declaration, and what every node writes in its use.
type param struct {
	name   string
	rng    scene.Range // the value in the text of the first node
	source string      // the text at rng, as the default of the export let
	args   []string    // the value of every node of the finding, in the order of its paths
}

// component is what Apply makes of a repeat finding: the parameters of the element, and the pos every
// use writes, since a use drops the pos written on the element it uses.
type component struct {
	params []param
	pos    [][]string // the pos of every node, in the order of the finding's paths; nil when it writes none
}

// Run reads the files named in args and prints what could be tidied in them: 0 always, 2 on usage or a
// file that cannot be read.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("suggest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print the findings as a JSON array")
	root := fs.String("root", "", "workspace root for --from-file (default: the working directory, or the directory of a file outside it)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, usage)
		return ExitUsage
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb suggest:", err)
		return ExitUsage
	}
	findings := []Finding{}
	failed := false
	files := 0
	for _, f := range fs.Args() {
		abs, err := filepath.Abs(f)
		if err != nil {
			fmt.Fprintln(stderr, "tenniarb suggest:", err)
			failed = true
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			fmt.Fprintln(stderr, "tenniarb suggest:", err)
			failed = true
			continue
		}
		tree, err := scene.Load(rootOf(*root, cwd, abs), abs)
		if tree == nil {
			fmt.Fprintln(stderr, "tenniarb suggest:", err)
			failed = true
			continue
		}
		files++
		if err != nil {
			// The structure is still usable: report what the file says, and say it is not clean.
			fmt.Fprintf(stderr, "tenniarb suggest: %s has errors, the report may be incomplete: %v\n", f, err)
		}
		findings = append(findings, Analyze(tree)...)
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(findings)
	} else {
		for _, fd := range findings {
			fmt.Fprintln(stdout, fd.text())
		}
		fmt.Fprintf(stdout, "%d files, %d findings\n", files, len(findings))
	}
	if failed {
		return ExitUsage
	}
	return ExitOK
}

// Text is the human-readable line of a finding, as tenniarb suggest prints it.
func (f Finding) Text() string { return f.text() }

// text is the human-readable line of a finding: position, rule, what repeats, what it saves.
func (f Finding) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d:%d: %s: %s", f.File, f.Line, f.Col, f.Rule, f.Detail)
	paths := f.Paths
	if len(paths) > 3 {
		paths = append(append([]string{}, paths[:3]...), fmt.Sprintf("+%d more", len(f.Paths)-3))
	}
	if len(paths) > 0 {
		b.WriteString(" [" + strings.Join(paths, ", ") + "]")
	}
	if f.Saving != "" {
		b.WriteString(" (" + f.Saving + ")")
	}
	return b.String()
}

// rootOf is the workspace root of file abs: -root, else the working directory as tenniarb check takes
// it, else the directory of the file when it lies outside the working directory.
func rootOf(flagRoot, cwd, abs string) string {
	if flagRoot != "" {
		if r, err := filepath.Abs(flagRoot); err == nil {
			return r
		}
	}
	if rel, err := filepath.Rel(cwd, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return cwd
	}
	return filepath.Dir(abs)
}

// Analyze reports what could be tidied in the main file of tree, with the gain of each suggestion. The
// tree is not changed. Only the nodes written in that file are looked at: a mounted file is reported
// when it is read itself.
func Analyze(tree *scene.Tree) []Finding {
	files := tree.Files()
	if len(files) == 0 {
		return []Finding{}
	}
	a := &analyzer{tree: tree, file: files[0], src: map[string]string{}, covered: map[*scene.Node]bool{}}
	root := tree.File(a.file)
	if root == nil {
		return []Finding{}
	}
	a.collect(root)
	a.groups()
	a.literals()
	a.dead(root)
	sort.SliceStable(a.findings, func(i, j int) bool {
		fi, fj := a.findings[i], a.findings[j]
		if ri, rj := rank(fi.Rule), rank(fj.Rule); ri != rj {
			return ri < rj
		}
		if fi.score != fj.score {
			return fi.score > fj.score
		}
		if fi.File != fj.File {
			return fi.File < fj.File
		}
		if fi.Line != fj.Line {
			return fi.Line < fj.Line
		}
		return fi.Col < fj.Col
	})
	if a.findings == nil {
		return []Finding{}
	}
	return a.findings
}

// rank orders the rules by value: the findings of the first rules are listed first.
func rank(rule string) int {
	switch {
	case rule == RuleRepeat:
		return 0
	case rule == RuleStyles:
		return 1
	case rule == RuleLiteral:
		return 2
	case strings.HasPrefix(rule, RuleDead):
		return 3
	default:
		return 4
	}
}

type analyzer struct {
	tree     *scene.Tree
	file     string // relative slash path of the file analysed
	src      map[string]string
	cands    []*cand
	findings []Finding
	covered  map[*scene.Node]bool // nodes already named by a repeat or styles finding
}

type cand struct {
	n   *scene.Node
	f   *form
	ctx string // the layout the parent gives the node: only one ctx can be merged (rule 5)
}

// leaf is one literal a form took out of a subtree.
type leaf struct {
	label string // dotted place in the subtree, for the report only: leaves are matched by index
	prop  string // the property it came from; "@name" for a node name, "@path" for an at path
	value string
	path  string      // the node that writes it: "" for the node the form is of, its descendants otherwise
	rng   scene.Range // where the text writes it; zero when the node does not write it
	why   string      // why rng is zero, for the refusal of a repeat
}

// form is the shape of a subtree with its literals taken out: two nodes with one form are the same
// structure with other values, so the values are the parameters of a component and the shape is what
// the component holds (spec 5c).
type form struct {
	sig    string
	leaves []leaf
	cmds   int // commands written in the subtree
	lines  int // text lines the subtree takes
}

// collect lists every node written in the file, in file order, and takes the shape of its items.
func (a *analyzer) collect(root *scene.Node) {
	a.eachNode(root, func(n *scene.Node) {
		if n.Kind == scene.KItem {
			a.cands = append(a.cands, &cand{n: n, f: a.form(n), ctx: layoutCtx(n)})
		}
	})
}

// eachNode visits every node written in the file: elements, items, links, uses, and the nodes and
// overrides a use passes in. The content of a use is its Block, so it is not visited twice.
func (a *analyzer) eachNode(n *scene.Node, visit func(*scene.Node)) {
	for _, e := range n.Elems {
		if a.other(e) {
			continue
		}
		visit(e)
		a.eachNode(e, visit)
	}
	if n.Kind == scene.KUse {
		a.eachOverride(n.Block, visit)
		return
	}
	for _, c := range n.Content {
		if a.other(c) {
			continue
		}
		visit(c)
		a.eachNode(c, visit)
	}
}

// eachOverride visits the nodes a use passes in, at every depth of its block.
func (a *analyzer) eachOverride(ovs []*scene.Override, visit func(*scene.Node)) {
	for _, ov := range ovs {
		if ov.Node != nil && !a.other(ov.Node) {
			visit(ov.Node)
			a.eachNode(ov.Node, visit)
		}
		a.eachOverride(ov.Block, visit)
	}
}

func (a *analyzer) other(n *scene.Node) bool { return n.Pos.File != a.file }

// form is the shape of n with every literal value taken out.
func (a *analyzer) form(n *scene.Node) *form {
	f := &form{lines: a.lines(n)}
	var b strings.Builder
	a.walk(n, "", f, &b)
	f.sig = b.String()
	return f
}

func (a *analyzer) walk(n *scene.Node, p string, f *form, b *strings.Builder) {
	b.WriteString("(" + n.Kind.String())
	val(f, b, leaf{label: p + "@name", prop: "@name", value: n.Name, path: p, rng: n.NameArg})
	switch n.Kind {
	case scene.KUse:
		val(f, b, leaf{label: p + "@use", prop: "@use", value: n.Path, path: p, rng: n.PathArg})
	case scene.KLink:
		val(f, b, leaf{label: p + "@from", prop: "@from", value: n.From, path: p, rng: n.FromArg})
		val(f, b, leaf{label: p + "@to", prop: "@to", value: n.To, path: p, rng: n.ToArg})
	}
	props := effectiveProps(n)
	f.cmds += 1 + len(props)
	for _, pr := range props {
		b.WriteString("|" + pr.name + ":" + strconv.Itoa(len(pr.args)))
		for i, v := range pr.args {
			rng, why := leafRange(pr.at, i)
			val(f, b, leaf{label: propLabel(p, pr.name, i, len(pr.args)), prop: pr.name, value: v, path: p, rng: rng, why: why})
		}
	}
	a.walkOverrides(n.Block, p, f, b)
	if n.Kind == scene.KUse {
		b.WriteString(")") // its content is the Block, already walked
		return
	}
	for i, c := range n.Content {
		if a.other(c) {
			continue
		}
		a.walk(c, childLabel(p, c, i), f, b)
	}
	b.WriteString(")")
}

func (a *analyzer) walkOverrides(ovs []*scene.Override, p string, f *form, b *strings.Builder) {
	for _, ov := range ovs {
		f.cmds++
		switch ov.Kind {
		case scene.OvProp:
			args := propArgs(ov.Prop)
			b.WriteString("|@" + ov.Prop.Name + ":" + strconv.Itoa(len(args)))
			for i, v := range args {
				rng, why := leafRange(ov.Prop, i)
				val(f, b, leaf{label: propLabel(p+"@", ov.Prop.Name, i, len(args)), prop: ov.Prop.Name, value: v, path: p, rng: rng, why: why})
			}
		case scene.OvNode:
			a.walk(ov.Node, childLabel(p, ov.Node, 0), f, b)
		case scene.OvAt, scene.OvReplace:
			b.WriteString("|@path")
			val(f, b, leaf{label: p + "@at", prop: "@path", value: ov.Path, path: p, rng: ov.PathArg})
			a.walkOverrides(ov.Block, p+"@"+ov.Path, f, b)
		case scene.OvSlot:
			b.WriteString("|@slot")
			val(f, b, leaf{label: p + "@slot", prop: "@slot", value: ov.Slot, path: p, why: "a slot name is no value of the text"})
			a.walkOverrides(ov.Block, p+"@slot", f, b)
		}
	}
}

// val takes one literal out of the subtree.
func val(f *form, b *strings.Builder, l leaf) {
	b.WriteString("#")
	f.leaves = append(f.leaves, l)
}

// leafRange is where argument i of the command p is written in the text, and why it is not when the node
// does not write it: a property a style gives it, or a computed value, has no literal to replace.
func leafRange(p *scene.Prop, i int) (scene.Range, string) {
	switch {
	case p == nil:
		return scene.Range{}, "a style gives it"
	case isExpression(p), p.Val != nil:
		return scene.Range{}, "it is a computed value"
	case i >= len(p.Args):
		return scene.Range{}, "it is not written on the node"
	}
	return argRange(p.Args[i]), ""
}

// argRange is the span of an argument in the text, with the quotes of a string, as scene.Range holds it.
func argRange(a *tenn.Node) scene.Range {
	if a == nil || a.Token == nil {
		return scene.Range{}
	}
	if t := a.Token; t.Type == tenn.TokStringLit {
		return scene.Range{Start: t.Pos - 1, End: t.EndPos()}
	}
	return scene.Range{Start: a.Token.Pos, End: a.Token.EndPos()}
}

func childLabel(p string, n *scene.Node, i int) string {
	name := n.Name
	if name == "" {
		name = n.Kind.String()
	}
	return p + "." + name + "[" + strconv.Itoa(i) + "]"
}

func propLabel(p, name string, i, n int) string {
	l := p + "." + name
	if n > 1 {
		l += "[" + strconv.Itoa(i) + "]"
	}
	return l
}

// lines is how many text lines the node takes in its file.
func (a *analyzer) lines(n *scene.Node) int {
	s := a.source(n.Pos.File)
	if n.Range.End <= n.Range.Start || n.Range.End > len(s) {
		return 1
	}
	return 1 + strings.Count(s[n.Range.Start:n.Range.End], "\n")
}

func (a *analyzer) source(file string) string {
	s, ok := a.src[file]
	if !ok {
		s, _ = a.tree.Source(file)
		a.src[file] = s
	}
	return s
}

// at turns a byte offset in the text of file into a line and column. It does not use scene.Pos.Line:
// go/tenn counts the newline that ends a // comment twice, so every comment line above a node shifts
// its Pos.Line by one (measured: two comments over an element give it line 4 instead of 2). Range.Start
// is a byte offset, so the text answers exactly.
func (a *analyzer) at(file string, off int) (int, int) {
	s := a.source(file)
	if off < 0 || off > len(s) {
		return 1, 1
	}
	return 1 + strings.Count(s[:off], "\n"), off - strings.LastIndex(s[:off], "\n")
}

// note adds a finding whose position is the byte offset off of file, and keeps the output short.
func (a *analyzer) note(rule, file string, off int, paths []string, detail, saving string, score int) *Finding {
	line, col := a.at(file, off)
	a.findings = append(a.findings, Finding{
		Rule: rule, File: file, Line: line, Col: col,
		Paths: paths, Detail: detail, Saving: saving, score: score,
	})
	return &a.findings[len(a.findings)-1]
}

// propVal is one property in effect: a command of the node, or one a style gives it.
type propVal struct {
	name string
	args []string
	at   *scene.Prop // the command written on the node; nil when a style gives it
}

func propArgs(p *scene.Prop) []string {
	if p.Val != nil { // an evaluated expression (spec 6)
		return p.Val
	}
	return p.Values()
}

func propOf(p *scene.Prop) propVal {
	return propVal{name: p.Name, args: propArgs(p), at: p}
}

// effectiveProps is what the node ends up with: its own commands, with the properties of the styles it
// names in place of its use-style, as scene inserts them (afterUseStyle, spec 5b). The use-style
// command itself is left out: it selects a look, it is not one.
func effectiveProps(n *scene.Node) []propVal {
	var out []propVal
	at := len(n.Props)
	for i, p := range n.Props {
		if p.Name == "use-style" {
			at = i + 1
		}
	}
	addStyled := func() {
		scope := scene.StyleScope(n)
		if scope == nil {
			return
		}
		for _, name := range scene.StyleRefs(n.Props) {
			st := scope.Styles[name]
			if st == nil {
				continue
			}
			for _, p := range st.Props {
				out = append(out, propVal{name: p.Name, args: propArgs(p)}) // no at: the node does not write it
			}
		}
	}
	for i, p := range n.Props {
		if i == at {
			addStyled()
		}
		if p.Name == "use-style" {
			continue
		}
		out = append(out, propOf(p))
	}
	if at == len(n.Props) {
		addStyled()
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// writtenProps are the commands written on the node itself, use-style excluded.
func writtenProps(n *scene.Node) []propVal {
	var out []propVal
	for _, p := range n.Props {
		if p.Name == "use-style" {
			continue
		}
		out = append(out, propOf(p))
	}
	return out
}

func sameProp(a, b propVal) bool { return a.name == b.name && sameArgs(a.args, b.args) }

// layoutProps are the commands of a parent that place its children: two nodes under parents that
// differ in them are not interchangeable, even with the same structure (rule 5).
var layoutProps = map[string]bool{
	"auto-layout": true, "gap": true, "align": true, "justify": true,
	"padding": true, "wrap": true, "line-gap": true,
}

// layoutCtx is the placement the parent gives the node: its kind and the layout in effect.
func layoutCtx(n *scene.Node) string {
	p := n.Parent
	if p == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(p.Kind.String())
	flow := false
	for _, pr := range effectiveProps(p) {
		if !layoutProps[pr.name] {
			continue
		}
		v := strings.Join(pr.args, " ")
		if pr.name == "auto-layout" {
			flow = v != "abs"
			if v == "" {
				v = "row" // row is the default
			}
		}
		b.WriteString("|" + pr.name + "=" + v)
	}
	if !flow {
		b.WriteString("|flow=abs")
	}
	return b.String()
}

// styleable are the properties a named style is meant to hold (spec 5b): a difference limited to them
// means the nodes are variants, not different structures.
var styleable = map[string]bool{
	"color": true, "border-color": true, "text-color": true, "line-width": true,
	"width": true, "height": true, "min-width": true, "max-width": true,
	"min-height": true, "max-height": true, "padding": true, "gap": true,
	"corner-radius": true, "font-size": true, "font-weight": true, "font-family": true,
	"font-style": true, "letter-spacing": true, "line-height": true, "opacity": true,
	"shadow": true, "clip": true,
}

// textProps carry the text of a node; a style may hold them, but a varying text is a parameter of the
// instance, not a variant. A node name is no look at all: it is the name of the instance, so a set that
// differs only in names is a component (rule 1), not a style.
var textProps = map[string]bool{"title": true, "text": true, "body": true}

// repeatedProps are the commands the literal rule collects: the look of a node, its layout included. A
// style holds any of them, so one set of them repeated by many nodes is worth a name (rule 3).
var repeatedProps = map[string]bool{
	"auto-layout": true, "align": true, "justify": true, "wrap": true, "line-gap": true,
}

// groups finds the nodes with one structure and judges what to do with each set of them.
func (a *analyzer) groups() {
	bySig := map[string][]*cand{}
	var sigs []string
	var noMerge []Finding
	for _, c := range a.cands {
		if _, ok := bySig[c.f.sig]; !ok {
			sigs = append(sigs, c.f.sig)
		}
		bySig[c.f.sig] = append(bySig[c.f.sig], c)
	}
	for _, sig := range sigs {
		g := bySig[sig]
		if len(g) < 2 {
			continue
		}
		// rule 5: the same nodes under parents with another layout are not interchangeable
		byCtx := map[string][]*cand{}
		var ctxs []string
		for _, c := range g {
			if _, ok := byCtx[c.ctx]; !ok {
				ctxs = append(ctxs, c.ctx)
			}
			byCtx[c.ctx] = append(byCtx[c.ctx], c)
		}
		if len(ctxs) > 1 && mergeable(byCtx) {
			noMerge = append(noMerge, a.noMerge(byCtx, ctxs))
		}
		for _, ctx := range ctxs {
			if sub := byCtx[ctx]; len(sub) > 1 {
				a.judge(sub)
			}
		}
	}
	// a warning against merging is worth a line, not a page: keep the biggest sets and say how many
	// others there are
	sort.SliceStable(noMerge, func(i, j int) bool { return noMerge[i].score > noMerge[j].score })
	if len(noMerge) > 3 {
		noMerge[0].Detail += fmt.Sprintf("; %d more such sets elsewhere", len(noMerge)-3)
		noMerge = noMerge[:3]
	}
	a.findings = append(a.findings, noMerge...)
}

// mergeable reports whether the sets split by layout context are big enough to look like duplicates.
func mergeable(byCtx map[string][]*cand) bool {
	groups, n := 0, 0
	for _, sub := range byCtx {
		if len(sub) > 1 {
			groups++
			n += len(sub)
		}
	}
	return groups > 1 && n > 3
}

// noMerge reports a set that would look like one component but sits under different layouts. Naming it
// keeps the reader from merging what must stay apart (rule 5); it suggests nothing to change.
func (a *analyzer) noMerge(byCtx map[string][]*cand, ctxs []string) Finding {
	sort.SliceStable(ctxs, func(i, j int) bool { return len(byCtx[ctxs[i]]) > len(byCtx[ctxs[j]]) })
	first := byCtx[ctxs[0]][0].n
	var paths []string
	for _, ctx := range ctxs {
		for _, c := range byCtx[ctx] {
			paths = append(paths, a.withPos(c.n)+" in "+ctx)
		}
	}
	line, col := a.at(first.Pos.File, first.Range.Start)
	return Finding{
		Rule: RuleNoMerge, File: first.Pos.File, Line: line, Col: col,
		Paths:  paths,
		Detail: fmt.Sprintf("%d nodes have one structure but sit under %d different layouts: do not merge them, a component would change the placement", len(paths), len(ctxs)),
		Saving: "none: this only warns against merging",
		score:  len(paths),
	}
}

// judge decides what a set of nodes with one structure is worth: a named style when only the look or
// the text differs, a component when the structure is big enough to pay for its definition.
func (a *analyzer) judge(g []*cand) {
	sort.SliceStable(g, func(i, j int) bool { return g[i].n.Range.Start < g[j].n.Range.Start })
	f0 := g[0].f
	var vary []leaf
	var names []string
	for i, l := range f0.leaves {
		for _, c := range g[1:] {
			if c.f.leaves[i].value != l.value {
				vary = append(vary, l)
				names = append(names, human(l.prop))
				break
			}
		}
	}
	props := distinct(names)
	if len(vary) == 0 {
		a.repeat(g, nil, nil)
		return
	}
	lookOnly, nameOnly := true, true
	for _, p := range vary {
		if !styleable[p.prop] && !textProps[p.prop] {
			lookOnly = false
			if p.prop != "@name" {
				nameOnly = false // a child, a use or an at differs: not a variant
			}
		}
	}
	shared := sharedProps(g)
	if lookOnly && len(shared) > 0 {
		a.styles(g, shared, props)
		return
	}
	if a.repeat(g, vary, props) {
		return
	}
	// a component does not pay off here: the look they share is still worth a name, as long as the nodes
	// are the same thing under other names
	if nameOnly && len(shared) > 0 {
		a.styles(g, shared, props)
	}
}

// sharedProps are the commands written on every node of the set with the same values: what one named
// style would hold. pos is left out: it places the copy, it is no look.
func sharedProps(g []*cand) []propVal {
	var out []propVal
	for _, p := range writtenProps(g[0].n) {
		if p.name == "pos" {
			continue
		}
		inAll := true
		for _, c := range g[1:] {
			found := false
			for _, q := range writtenProps(c.n) {
				if sameProp(p, q) {
					found = true
					break
				}
			}
			if !found {
				inAll = false
				break
			}
		}
		if inAll {
			out = append(out, p)
		}
	}
	return out
}

// styles reports a set whose members differ only in look or text: the look they share becomes a named
// style, the text that differs stays on the node.
func (a *analyzer) styles(g []*cand, shared []propVal, varies []string) {
	first := g[0].n
	var names []string
	for _, p := range shared {
		names = append(names, p.name+" "+strings.Join(p.args, " "))
	}
	paths := make([]string, 0, len(g))
	for _, c := range g {
		paths = append(paths, a.withPos(c.n))
		a.covered[c.n] = true
	}
	f := a.note(RuleStyles, first.Pos.File, first.Range.Start, paths,
		fmt.Sprintf("%d nodes are the same except %s; they share %d properties (%s): name that look once and use-style it",
			len(g), strings.Join(varies, ", "), len(shared), strings.Join(names, ", ")),
		fmt.Sprintf("%d repeated properties", (len(g)-1)*len(shared)), (len(g)-1)*len(shared))
	a.applyNodes(f, g, shared)
}

// applyNodes remembers what Apply changes: the nodes that get use-style, and the look the style holds.
func (a *analyzer) applyNodes(f *Finding, g []*cand, props []propVal) {
	for _, c := range g {
		f.nodes = append(f.nodes, c.n)
	}
	f.props = props
}

// repeat reports a set worth one component: the definition holds the structure once, each copy becomes
// a use whose parameters are the values that differ (spec 5c). It reports whether it did.
//
// The finding carries what Apply needs, or the reason these nodes cannot become one element: a value
// that is not written on the node, a nested name or pos, a use path, a style from outside the set. The
// report says the reason too, so a refused set is not silently left alone.
func (a *analyzer) repeat(g []*cand, vary []leaf, params []string) bool {
	if len(vary) > g[0].f.cmds/2 {
		return false // too many parameters: the component would be a layer of indirection, not a saving
	}
	total, def, lo, hi := 0, 0, g[0].f.lines, g[0].f.lines
	for _, c := range g {
		total += c.f.lines
		def = max(def, c.f.lines)
		lo, hi = min(lo, c.f.lines), max(hi, c.f.lines)
	}
	saved := total - (def + 2 + len(g)) // one copy as the definition, an element line, a close line, one line per use
	if saved <= 0 {
		return false
	}
	size := fmt.Sprintf("%d lines each", lo)
	if lo != hi {
		size = fmt.Sprintf("%d-%d lines", lo, hi)
	}
	first := g[0].n
	detail := fmt.Sprintf("%d nodes have one structure (%s); they differ only in %s: make an element and use it",
		len(g), size, strings.Join(params, ", "))
	if len(params) == 0 {
		detail = fmt.Sprintf("%d nodes have one structure (%s): make an element and use it", len(g), size)
	}
	paths := make([]string, 0, len(g))
	for _, c := range g {
		paths = append(paths, a.withPos(c.n))
		a.covered[c.n] = true
	}
	f := a.note(RuleRepeat, first.Pos.File, first.Range.Start, paths, detail,
		fmt.Sprintf("%d lines fewer, %d parameters", saved, len(params)), saved)
	f.comp, f.why = a.plan(g)
	for _, c := range g {
		f.nodes = append(f.nodes, c.n)
	}
	if f.why != "" {
		f.Detail += "; needs a component with parameters, apply by hand: " + f.why
	}
	return true
}

// plan is the element a repeat finding makes: the body is the block of the first node, the parameters are
// the literals that differ. It returns the reason the set cannot become one element instead.
func (a *analyzer) plan(g []*cand) (*component, string) {
	first := g[0].n
	src := a.source(a.file)
	for _, c := range g {
		switch {
		case c.n.Pos.File != a.file:
			return nil, "a node is written in another file"
		case insideUse(c.n):
			return nil, "a node sits in the block of a use or an at"
		case c.n.Name == "":
			return nil, "a node has no name to give its use"
		case c.n.Range.Start < 0 || c.n.Range.End > len(src):
			return nil, "a node has no text in the file"
		}
	}
	if reason := subtreeReason(a.tree, first); reason != "" {
		return nil, reason
	}
	for _, c := range g[1:] {
		if styleText(c.n) != styleText(first) {
			return nil, "the nodes name different styles, so a style writes what the text does not show"
		}
	}
	if reason := rootReason(first); reason != "" {
		return nil, reason
	}
	inner := first.Body
	c := &component{}
	taken := map[string]bool{}
	for _, p := range first.Params {
		taken[p.Name] = true
	}
	posVaries := false
	for i, l := range g[0].f.leaves {
		varies := false
		for _, other := range g[1:] {
			if other.f.leaves[i].value != l.value {
				varies = true
				break
			}
		}
		if !varies {
			continue
		}
		switch {
		case l.prop == "@name" && l.path == "":
			continue // the name of the node is the alias of its use
		case l.prop == "pos" && l.path == "":
			posVaries = true
			continue
		case l.prop == "@name":
			return nil, "a nested node is named differently: at would be needed"
		case l.prop == "pos":
			return nil, "a nested node sits at another place: at would be needed"
		case l.prop == "@use":
			return nil, "a nested use has another path"
		case l.prop == "@path":
			return nil, "an at or replace path differs"
		case l.prop == "@from" || l.prop == "@to":
			return nil, "an end of a link differs"
		case l.prop == "@slot":
			return nil, "a slot name differs"
		case l.why != "":
			return nil, fmt.Sprintf("the value of %q differs, but %s", l.prop, l.why)
		case inner == (scene.Range{}) || l.rng.Start < inner.Start+1 || l.rng.End > inner.End-1:
			return nil, fmt.Sprintf("the value of %q is not in the body of the node", l.prop)
		}
		values := make([]string, len(g))
		for j, other := range g {
			values[j] = other.f.leaves[i].value
		}
		c.params = append(c.params, param{
			name: paramName(l.prop, taken), rng: l.rng, source: src[l.rng.Start:l.rng.End], args: values,
		})
	}
	sort.SliceStable(c.params, func(i, j int) bool { return c.params[i].name < c.params[j].name })
	// the element of a use loses its own pos (it is the place of the definition), so the place of every
	// node goes to its use, unchanged; a pos a style gives stays in the element with the style
	if findProp(first.Props, "pos") != nil {
		c.pos = make([][]string, len(g))
		for j, other := range g {
			own := findProp(other.n.Props, "pos")
			if own == nil || isExpression(own) {
				return nil, "pos is not written on every node of the set"
			}
			c.pos[j] = propArgs(own)
		}
	} else if posVaries {
		return nil, "pos differs, but a style gives it: the text has no value to pass"
	}
	return c, ""
}

// rootReason is why the root of the set cannot be the body of an element: a component root with items
// draws no title of its own, and one that its parent lays out would take the default box instead.
func rootReason(root *scene.Node) string {
	if findProp(root.Props, "template") != nil {
		return "the node has a template"
	}
	kids := 0
	for _, c := range root.Content {
		if c.Kind == scene.KItem {
			kids++
		}
	}
	if kids == 0 {
		return ""
	}
	for _, p := range effectiveProps(root) {
		if textProps[p.name] {
			return fmt.Sprintf("the %s of a node with children is not drawn on a component", p.name)
		}
	}
	if findProp(root.Props, "auto-layout") == nil && !laidOutByParent(root) {
		return "the parent lays the node out, and a component root would draw its own box instead"
	}
	for _, c := range root.Content {
		for _, p := range effectiveProps(c) {
			if (p.name == "width" || p.name == "height") && len(p.args) > 0 && p.args[0] == "fill" {
				return fmt.Sprintf("the %s of %q fills the node, and an element is laid out on its own", p.name, c.Name)
			}
		}
	}
	return ""
}

// laidOutByParent reports whether the parent of n has an auto-layout that places n: the root of an
// element does not, so its content sits at its own pos.
func laidOutByParent(n *scene.Node) bool {
	p := n.Parent
	if p == nil || p.Kind != scene.KItem {
		return false
	}
	for _, pr := range effectiveProps(p) {
		if pr.name == "auto-layout" {
			return true
		}
	}
	return false
}

// paramName is the name of the parameter a property takes: the property itself, made one word so ${name}
// reads as a variable, and _2, _3 when another parameter wants it.
func paramName(prop string, taken map[string]bool) string {
	base := scene.NormalizeName(strings.TrimPrefix(prop, "@"))
	if base == "" {
		base = "value"
	}
	name := base
	for i := 2; taken[name]; i++ {
		name = base + "_" + strconv.Itoa(i)
	}
	taken[name] = true
	return name
}

// subtreeReason is why the subtree of n cannot move into an element at the top of the file: what it takes
// from around it, a mount, a slot, a computed value or a use path whose scope would change.
func subtreeReason(tree *scene.Tree, n *scene.Node) string {
	reason := ""
	root := tree.File(n.Pos.File)
	a := &analyzer{file: n.Pos.File}
	a.eachNode(n, func(c *scene.Node) {
		if reason != "" {
			return
		}
		switch {
		case c.Mount != nil || findProp(c.Props, "from-file") != nil:
			reason = "the subtree mounts another file"
			return
		case c.Kind == scene.KSlot:
			reason = "the subtree has a slot"
			return
		case hasExpression(c.Props):
			reason = fmt.Sprintf("%q is computed in the place the node sits: a component would lose its scope", expressionName(c.Props))
			return
		case c.Kind == scene.KUse:
			if why := movedPath(tree, root, c); why != "" {
				reason = why
				return
			}
		}
		for _, ov := range c.Block {
			if ov.Prop != nil && isExpression(ov.Prop) {
				reason = fmt.Sprintf("%q of a use is computed in the place the node sits", ov.Prop.Name)
			}
		}
		for _, name := range scene.StyleRefs(c.Props) {
			if !declares(n, name) {
				reason = fmt.Sprintf("the style %q comes from an element outside the set: the component cannot see it", name)
			}
		}
	})
	return reason
}

// movedPath is why the path of a use reads another node from the top of the file, or "" when it does not:
// the component sits above the place the use was written in, so the search starts from there.
func movedPath(tree *scene.Tree, root *scene.Node, use *scene.Node) string {
	was, err := tree.Resolve(use.Parent, use.Path)
	if err != nil {
		return fmt.Sprintf("the path %q does not resolve where the node sits", use.Path)
	}
	now, err := tree.Resolve(root, use.Path)
	if err != nil || now != was {
		return fmt.Sprintf("the path %q reads another node from the top of the file", use.Path)
	}
	return ""
}

// styleText is the use-style of every node of the subtree, in tree order: a copy of the text keeps the
// first styles only, so a set whose nodes name other styles would change its look.
func styleText(root *scene.Node) string {
	var b strings.Builder
	b.WriteString("root:" + strings.Join(scene.StyleRefs(root.Props), ",") + ";")
	a := &analyzer{file: root.Pos.File}
	a.eachNode(root, func(n *scene.Node) {
		b.WriteString(n.Kind.String() + ":" + strings.Join(scene.StyleRefs(n.Props), ",") + ";")
	})
	return b.String()
}

// declares reports whether the styles block of the root or of one of its descendants declares name: only
// such a style moves with the text into the component.
func declares(root *scene.Node, name string) bool {
	found := false
	a := &analyzer{file: root.Pos.File}
	a.eachNode(root, func(n *scene.Node) {
		if _, ok := n.Styles[name]; ok || hasGroup(n, name) {
			found = true
		}
	})
	if _, ok := root.Styles[name]; ok || hasGroup(root, name) {
		found = true
	}
	return found
}

func hasGroup(n *scene.Node, name string) bool {
	for _, g := range n.StyleGroups {
		if g.Name == name {
			return true
		}
	}
	return false
}

// hasExpression reports a command with an expression among its arguments (spec 6).
func hasExpression(props []*scene.Prop) bool {
	for _, p := range props {
		if isExpression(p) {
			return true
		}
	}
	return false
}

// expressionName is the name of the first computed command, for the refusal.
func expressionName(props []*scene.Prop) string {
	for _, p := range props {
		if isExpression(p) {
			return p.Name
		}
	}
	return ""
}

// literals finds one set of look properties repeated by many nodes: a named style is worth it even
// when the nodes themselves have nothing in common (rule 3).
func (a *analyzer) literals() {
	sets := map[string][]*cand{}
	setProps := map[string][]propVal{}
	var keys []string
	for _, c := range a.cands {
		if a.covered[c.n] {
			continue
		}
		var ps []propVal
		for _, p := range writtenProps(c.n) {
			if styleable[p.name] || repeatedProps[p.name] {
				ps = append(ps, p)
			}
		}
		if len(ps) == 0 {
			continue
		}
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].name < ps[j].name })
		var b strings.Builder
		for _, p := range ps {
			b.WriteString(p.name + " " + strings.Join(p.args, " ") + ";")
		}
		k := b.String()
		if _, ok := sets[k]; !ok {
			keys = append(keys, k)
			setProps[k] = ps
		}
		sets[k] = append(sets[k], c)
	}
	for _, k := range keys {
		g := sets[k]
		ps := setProps[k]
		need := 3
		if len(ps) == 1 {
			if !styleable[ps[0].name] {
				continue // a lone layout property is no look to name
			}
			need = 5 // a single property has to repeat more often to be worth a name
		}
		if len(g) < need {
			continue
		}
		first := g[0].n
		paths := make([]string, 0, len(g))
		for _, c := range g {
			paths = append(paths, a.withPos(c.n))
			a.covered[c.n] = true
		}
		f := a.note(RuleLiteral, first.Pos.File, first.Range.Start, paths,
			fmt.Sprintf("%d nodes share the same look (%s): name a style and use-style it", len(g), strings.TrimSuffix(k, ";")),
			fmt.Sprintf("%d repeated properties", (len(g)-1)*len(ps)), (len(g)-1)*len(ps))
		a.applyNodes(f, g, ps)
	}
}

func (a *analyzer) withPos(n *scene.Node) string {
	line, col := a.at(n.Pos.File, n.Range.Start)
	return fmt.Sprintf("%s (%s:%d:%d)", nodePath(n), n.Pos.File, line, col)
}

// nodePath is the way to the node from the root of its file, in the names as written.
func nodePath(n *scene.Node) string {
	var segs []string
	for p := n; p != nil; p = p.Parent {
		if p.Kind == scene.KFile {
			continue
		}
		name := p.Name
		if name == "" {
			name = "<" + p.Kind.String() + ">"
		}
		segs = append(segs, name)
	}
	for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
		segs[i], segs[j] = segs[j], segs[i]
	}
	return strings.Join(segs, ".")
}

// human names a value of a form the way the format writes it, for the report.
func human(prop string) string {
	switch prop {
	case "@name":
		return "the node name"
	case "@use":
		return "the use path"
	case "@path":
		return "the at path"
	case "@from", "@to":
		return "the link end"
	case "@slot":
		return "the slot name"
	}
	return prop
}

func distinct(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func sameArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
