package optimize

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/haiodo/tenn/go"
	"github.com/haiodo/tenniarb/go/scene"
)

// Edit replaces Text[Start:End) of the main file by New. Offsets are bytes of the text of that file, as
// scene.Range holds them.
type Edit struct {
	Start, End int
	New        string
}

// errDifferent is the refusal of a finding whose nodes live under different elements: a style sits in
// the block of one element, so one file cannot hold such a change.
var errDifferent = errors.New("nodes are in different elements")

// Apply builds the text edits of one finding of Analyze: the ranges of the main file to replace, and a
// short human summary of the lines that go and come. The tree is not changed; nothing is written.
func Apply(tree *scene.Tree, f Finding) ([]Edit, string, error) {
	src, ok := tree.Source(f.File)
	if !ok {
		return nil, "", fmt.Errorf("%s is not a file of this scene", f.File)
	}
	var edits []Edit
	var err error
	switch {
	case f.Rule == RuleStyles || f.Rule == RuleLiteral:
		edits, err = styleEdits(src, f)
	case strings.HasPrefix(f.Rule, RuleDead):
		edits, err = deadEdits(src, f)
	case f.Rule == RuleNoMerge:
		return nil, "", errors.New("no-merge changes nothing: it warns against merging the nodes")
	case f.Rule == RuleRepeat:
		edits, err = repeatEdits(tree, src, f)
	default:
		return nil, "", fmt.Errorf("nothing to apply for %s", f.Rule)
	}
	if err != nil {
		return nil, "", err
	}
	out, err := applyText(src, edits)
	if err != nil {
		return nil, "", err
	}
	if _, p := tenn.Parse(out); p.Errors.HasErrors() {
		e := p.Errors.Errors[0]
		return nil, "", fmt.Errorf("the change would break the file at %d:%d: %s", e.Line+1, e.Col, e.Message)
	}
	return edits, diffPreview(src, out), nil
}

// ApplyText applies edits to text and returns one edit that makes the same change, so a caller applies
// it as one undo step.
func ApplyText(text string, edits []Edit) (Edit, error) {
	es, err := mergeEdits(text, edits)
	if err != nil {
		return Edit{}, err
	}
	if len(es) == 0 {
		return Edit{}, errors.New("nothing to change")
	}
	var b strings.Builder
	at := es[0].Start
	for _, e := range es {
		if e.Start < at || e.End < e.Start || e.End > len(text) {
			return Edit{}, fmt.Errorf("range %d..%d is outside the text of %d bytes", e.Start, e.End, len(text))
		}
		b.WriteString(text[at:e.Start])
		b.WriteString(e.New)
		at = e.End
	}
	return Edit{es[0].Start, es[len(es)-1].End, b.String()}, nil
}

// applyText is text with edits applied.
func applyText(text string, edits []Edit) (string, error) {
	e, err := ApplyText(text, edits)
	if err != nil {
		return "", err
	}
	return text[:e.Start] + e.New + text[e.End:], nil
}

// mergeEdits joins the edits that touch or overlap, so the text between them is kept and the caller
// sees one change per place. An edit nested in another is refused: the generators never make one.
func mergeEdits(text string, edits []Edit) ([]Edit, error) {
	es := append([]Edit{}, edits...)
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].Start != es[j].Start {
			return es[i].Start < es[j].Start
		}
		return es[i].End < es[j].End
	})
	var out []Edit
	for _, e := range es {
		if n := len(out); n > 0 && e.Start <= out[n-1].End {
			prev := &out[n-1]
			if e.End < prev.End {
				return nil, fmt.Errorf("the edit at %d..%d is nested in %d..%d", e.Start, e.End, prev.Start, prev.End)
			}
			if e.Start > prev.End {
				prev.New += text[prev.End:e.Start]
			}
			prev.New += e.New
			prev.End = max(prev.End, e.End)
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// styleEdits names the look two or more nodes share once and gives every node a use-style: the style
// goes into the styles block of the nearest element that holds them all, created after its header when
// the element has none.
func styleEdits(src string, f Finding) ([]Edit, error) {
	if len(f.nodes) < 2 || len(f.props) == 0 {
		return nil, errors.New("the finding carries no nodes to change")
	}
	for _, n := range f.nodes {
		if n.Pos.File != f.File {
			return nil, errDifferent
		}
	}
	elem, err := commonOwner(f.nodes)
	if err != nil {
		return nil, err
	}
	props := movableProps(f.nodes, f.props)
	if len(props) == 0 {
		return nil, errors.New("the shared properties are the layout, a size the layout checks or an expression, which a style cannot hold safely: name the style by hand")
	}
	s := newSrc(src)
	name := styleName(elem, props)
	var edits []Edit
	if block := s.cmd(elem, "styles"); block != nil {
		e, ok := insertIntoBlock(src, rangeOf(block), block.Token.Pos, styleDecl(name, props))
		if !ok {
			return nil, errors.New("the styles block has no block to add to")
		}
		edits = append(edits, e)
	} else {
		e, err := insertStyles(src, elem, styleDecl(name, props))
		if err != nil {
			return nil, err
		}
		edits = append(edits, e)
	}
	for _, n := range f.nodes {
		edits = append(edits, nodeEdits(src, n, name, props)...)
	}
	return edits, nil
}

// repeatEdits makes a component of a repeat finding: the block of the first node becomes the body of an
// element at the end of the file, the literals that differ become its parameters, and every node of the
// set a use that passes them (spec 5c). The pos of a node goes to its use as an override: a use drops
// the pos written on the element, which is the place of the definition.
func repeatEdits(tree *scene.Tree, src string, f Finding) ([]Edit, error) {
	if f.why != "" {
		return nil, fmt.Errorf("%s: apply by hand", f.why)
	}
	if f.comp == nil || len(f.nodes) < 2 {
		return nil, errors.New("the finding carries no nodes to fold into a component")
	}
	// stale offsets would cut the wrong text: every node must still be an item where the finding found it
	s := newSrc(src)
	for _, n := range f.nodes {
		switch {
		case n.Pos.File != f.File:
			return nil, errDifferent
		case insideUse(n):
			return nil, errors.New("a node sits in the block of a use or an at: apply by hand")
		case n.Range.Start < 0 || n.Range.End > len(src):
			return nil, errors.New("the finding is out of date: run Suggest Improvements again")
		}
		if c := s.at(n.Range.Start); c != nil {
			if id, _ := c.GetIdent(0); id == "item" {
				continue
			}
		}
		return nil, errors.New("the finding is out of date: run Suggest Improvements again")
	}
	name := componentName(tree, f)
	c := f.comp
	first := f.nodes[0]
	body := ""
	if inner := first.Body; inner != (scene.Range{}) && inner.End-1 <= len(src) {
		base := inner.Start + 1
		body = src[base : inner.End-1]
		var bodyEdits []Edit
		for _, p := range c.params {
			if p.rng.Start < base || p.rng.End > inner.End-1 {
				return nil, errors.New("the finding is out of date: run Suggest Improvements again")
			}
			bodyEdits = append(bodyEdits, Edit{p.rng.Start - base, p.rng.End - base, "${" + p.name + "}"})
		}
		if c.pos != nil {
			pos := findProp(first.Props, "pos")
			if pos == nil || pos.Range.Start < base || pos.Range.End > inner.End-1 {
				return nil, errors.New("the finding is out of date: run Suggest Improvements again")
			}
			bodyEdits = append(bodyEdits, removeEdit(body, scene.Range{Start: pos.Range.Start - base, End: pos.Range.End - base}))
		}
		if len(bodyEdits) > 0 {
			var err error
			if body, err = applyText(body, bodyEdits); err != nil {
				return nil, err
			}
		}
	}
	// the body sits one level inside the element: take away the indentation of the node's block
	ind, unit := indentAt(src, first.Range.Start), indentUnit(src)
	if !strings.Contains(body, "\n") {
		body = strings.TrimSpace(body)
	}
	body = trimBlank(dedent(body, ind+unit))
	if body != "" {
		body = indentLines(body, unit)
	}
	var b strings.Builder
	b.WriteString("element " + scene.QuoteName(name) + " {\n")
	b.WriteString(unit + "kind Component\n")
	for _, p := range c.params {
		b.WriteString(unit + "export let " + p.name + " = " + p.source + "\n")
	}
	if body != "" {
		b.WriteString(body + "\n")
	}
	b.WriteString("}\n")

	var edits []Edit
	for j, n := range f.nodes {
		cmds := make([]string, 0, len(c.params)+1)
		for _, p := range c.params {
			cmds = append(cmds, commandText(p.name, p.args[j]))
		}
		if c.pos != nil {
			cmds = append(cmds, commandText("pos", c.pos[j]...))
		}
		line := "use " + scene.SegText(name) + " as " + scene.SegText(n.Name)
		if len(cmds) > 0 {
			line += " { " + strings.Join(cmds, "; ") + " }"
		}
		edits = append(edits, Edit{n.Range.Start, n.Range.End, line})
	}
	sep := "\n"
	switch {
	case strings.HasSuffix(src, "\n\n"):
		sep = ""
	case !strings.HasSuffix(src, "\n"):
		sep = "\n\n"
	}
	return append(edits, Edit{len(src), len(src), sep + b.String()}), nil
}

// componentName is the name of the element a repeat finding adds: the name of its first node, with -2,
// -3 when the file or one of the instances already uses it. A use named as its own path would find
// itself, so the name of an instance is taken too.
func componentName(tree *scene.Tree, f Finding) string {
	taken := map[string]bool{}
	if root := tree.File(f.File); root != nil {
		a := &analyzer{file: f.File}
		a.eachNode(root, func(n *scene.Node) { taken[n.Key] = true })
	}
	for _, n := range f.nodes {
		taken[n.Key] = true
	}
	base := scene.NormalizeName(f.nodes[0].Name)
	if base == "" {
		base = "Component"
	}
	name := base
	for i := 2; taken[scene.NormalizeName(name)]; i++ {
		name = base + "-" + strconv.Itoa(i)
	}
	return name
}

// trimBlank drops the blank lines at the edges of a block, and the spaces of every line after the last.
func trimBlank(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// dedent removes the indentation of the node from every line of the text copied out of it.
func dedent(s, ind string) string {
	if ind == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(strings.TrimPrefix(l, ind), " \t")
	}
	return strings.Join(lines, "\n")
}

// nodeEdits removes the shared properties from n and adds use-style name to it, appending the name to
// the use-style line it already has.
func nodeEdits(src string, n *scene.Node, name string, props []propVal) []Edit {
	var edits []Edit
	var use *scene.Prop
	for _, p := range n.Props {
		if p.Name == "use-style" {
			use = p
		}
	}
	if use != nil {
		edits = append(edits, Edit{use.Range.Start, use.Range.End, src[use.Range.Start:use.Range.End] + " " + name})
	} else if n.Body != (scene.Range{}) {
		if e, ok := insertIntoBlock(src, n.Body, n.Range.Start, commandText("use-style", name)); ok {
			edits = append(edits, e)
		}
	} else {
		edits = append(edits, Edit{n.Range.End, n.Range.End, " { " + commandText("use-style", name) + " }"})
	}
	for _, p := range props {
		if own := findProp(n.Props, p.name); own != nil && sameArgs(propArgs(own), p.args) {
			edits = append(edits, removeEdit(src, own.Range))
		}
	}
	return edits
}

// contextWord reports a property the layout of the whole file checks against its context: hug and fill
// sizes need an auto-layout, and the check of a file lays a style out on its own, without the node it
// is used on. Such a property has to stay on the node, and so has the auto-layout a kid fills in.
func contextWord(p propVal) bool {
	if p.name == "auto-layout" {
		return true
	}
	if p.name != "width" && p.name != "height" {
		return false
	}
	return len(p.args) > 0 && (p.args[0] == "hug" || p.args[0] == "fill")
}

// movableProps are the shared properties that can move into a style without changing the look: not an
// expression (a style is not evaluated, spec 6), not a size the layout checks in context, and not
// written before the use-style of its node (the style there would win over the node, so the moved value
// would win over both).
func movableProps(nodes []*scene.Node, props []propVal) []propVal {
	var out []propVal
	for _, p := range props {
		if contextWord(p) {
			continue
		}
		move := true
		for _, n := range nodes {
			at := -1
			last := -1
			for i, q := range n.Props {
				switch {
				case q.Name == "use-style":
					at = i
				case q.Name == p.name:
					last = i
				}
			}
			if last <= at || isExpression(n.Props[last]) {
				move = false
				break
			}
		}
		if move {
			out = append(out, p)
		}
	}
	return out
}

func isExpression(p *scene.Prop) bool {
	for _, a := range p.Args {
		if a.Kind == tenn.KindExpression || a.Kind == tenn.KindExpressionBlock {
			return true
		}
	}
	return false
}

// commonOwner is the deepest element or item that holds every node: the element whose styles block the
// named style belongs to.
func commonOwner(nodes []*scene.Node) (*scene.Node, error) {
	for a := nodes[0]; a != nil; a = a.Parent {
		if (a.Kind != scene.KElement && a.Kind != scene.KItem) || a.Pos.File != nodes[0].Pos.File || insideUse(a) {
			continue
		}
		ok := true
		for _, n := range nodes[1:] {
			if !isUnder(n, a) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for _, n := range nodes {
			if n == a {
				continue // a style written on a belongs to a itself
			}
			scope := n
			if len(n.Styles) == 0 && len(n.StyleGroups) == 0 {
				scope = scene.StyleScope(n)
			}
			if scope != nil && scope != a {
				return nil, errors.New("the nodes take their styles from another element: name the style there")
			}
		}
		return a, nil
	}
	return nil, errDifferent
}

// insideUse reports a node written inside the block of a use: its use-style names a style of the
// component, not of the place the instance sits in.
func insideUse(n *scene.Node) bool {
	for ; n != nil; n = n.Parent {
		if n.Kind == scene.KUse {
			return true
		}
	}
	return false
}

func isUnder(n, a *scene.Node) bool {
	for ; n != nil; n = n.Parent {
		if n == a {
			return true
		}
	}
	return false
}

// styleName is a name for the look in props that the styles block of elem does not hold yet: w18-h18,
// then w18-h18-2.
func styleName(elem *scene.Node, props []propVal) string {
	taken := map[string]bool{}
	for name := range elem.Styles {
		taken[name] = true
	}
	for _, g := range elem.StyleGroups {
		taken[g.Name] = true
	}
	base := styleBase(props)
	name := base
	for i := 2; taken[scene.NormalizeName(name)]; i++ {
		name = base + "-" + strconv.Itoa(i)
	}
	return name
}

// styleAbbrev is the short word a property takes in a style name.
var styleAbbrev = map[string]string{
	"width": "w", "height": "h", "color": "c", "border-color": "border",
	"text-color": "text", "corner-radius": "r", "padding": "p", "gap": "gap",
	"font-size": "fs", "font-weight": "fw", "font-family": "font", "line-width": "lw",
	"min-width": "minw", "max-width": "maxw", "min-height": "minh", "max-height": "maxh",
	"letter-spacing": "ls", "line-height": "lh", "font-style": "italic", "opacity": "opacity",
	"shadow": "shadow",
}

// styleBase is the name the properties themselves give: w18-h18, cff0000, fs14-fw500.
func styleBase(props []propVal) string {
	var parts []string
	for _, p := range props {
		if len(parts) == 3 {
			break
		}
		abbr := styleAbbrev[p.name]
		if abbr == "" {
			abbr = scene.NormalizeName(p.name)
		}
		v := ""
		if len(p.args) > 0 {
			v = namePart(p.args[0])
		}
		part := abbr + v
		if len(part) > 24 {
			part = part[:24]
		}
		if !partsEqual(parts, part) {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "style"
	}
	return strings.Join(parts, "-")
}

func partsEqual(parts []string, s string) bool {
	for _, p := range parts {
		if p == s {
			return true
		}
	}
	return false
}

// namePart turns a value into a piece of an identifier: #ff0000 gives ff0000, "Big" gives Big.
func namePart(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '#' || r == '_' || r == '-' || r == '.':
			// dropped: a name holds letters and digits only, so it reads as one word
		default:
			b.WriteByte('_')
		}
	}
	s := strings.Trim(b.String(), "_")
	if len(s) > 12 {
		s = s[:12]
	}
	return s
}

// styleDecl is one named style as the format writes it in a styles block: name { a b; c d }.
func styleDecl(name string, props []propVal) string {
	cmds := make([]string, 0, len(props))
	for _, p := range props {
		cmds = append(cmds, commandText(p.name, p.args...))
	}
	return name + " { " + strings.Join(cmds, "; ") + " }"
}

// insertStyles puts the first styles block of an element right after its header line: the element has
// no styles yet, or the finding would not add one.
func insertStyles(src string, elem *scene.Node, decl string) (Edit, error) {
	ind := indentAt(src, elem.Range.Start)
	unit := indentUnit(src)
	block := "styles {\n" + indentLines(decl, ind+unit) + "\n" + ind + "}"
	if elem.Body == (scene.Range{}) {
		return Edit{elem.Range.End, elem.Range.End, " {\n" + indentLines(block, ind+unit) + "\n" + ind + "}"}, nil
	}
	first := elem.Body.Start + 1
	for first < elem.Body.End-1 && strings.ContainsRune(" \t\r\n", rune(src[first])) {
		first++
	}
	ls := lineStart(src, first)
	if first >= elem.Body.End-1 || ls <= elem.Body.Start {
		// the body is empty or starts on the line of the header: the block goes in through the brace
		if e, ok := insertIntoBlock(src, elem.Body, elem.Range.Start, block); ok {
			return e, nil
		}
		return Edit{}, errors.New("the element has no block to add a styles block to")
	}
	return Edit{ls, ls, indentLines(block, src[ls:first]) + "\n"}, nil
}

// insertIntoBlock adds cmd, one or more lines, as the last command of a block: body is the range from
// its { to after its }, start the offset of the command that holds the block, for the indentation.
func insertIntoBlock(text string, body scene.Range, start int, cmd string) (Edit, bool) {
	if body.End-body.Start < 2 {
		return Edit{}, false
	}
	open, closing := body.Start, body.End-1
	unit := indentUnit(text)
	ind := indentAt(text, start) + unit
	inner := text[open+1 : closing]
	if blank(strings.ReplaceAll(inner, "\n", "")) && !strings.Contains(cmd, "\n") {
		return Edit{open + 1, closing, " " + cmd + " "}, true
	}
	if !strings.Contains(inner, "\n") && !strings.Contains(cmd, "\n") {
		end := open + 1 + len(strings.TrimRight(inner, " \t"))
		return Edit{end, end, "; " + cmd}, true
	}
	ls := lineStart(text, closing)
	if blank(text[ls:closing]) {
		return Edit{ls, ls, indentLines(cmd, ind) + "\n"}, true
	}
	return Edit{closing, closing, "\n" + indentLines(cmd, ind) + "\n" + indentAt(text, start)}, true
}

// deadEdits deletes what a dead finding names: the style or group itself, an at override, a pos.
func deadEdits(src string, f Finding) ([]Edit, error) {
	switch {
	case f.Rule == "dead-style" || f.Rule == "dead-group":
		if f.elem == nil || f.name == "" {
			return nil, errors.New("the finding does not say what to delete")
		}
		s := newSrc(src)
		block := s.cmd(f.elem, "styles")
		if block == nil {
			return nil, errors.New("the element has no styles block any more")
		}
		c := s.cmdIn(block, f.name)
		if c == nil {
			return nil, fmt.Errorf("%q is not in the styles block any more", f.name)
		}
		return []Edit{removeEdit(src, rangeOf(c))}, nil
	case f.del != nil:
		return []Edit{removeEdit(src, *f.del)}, nil
	}
	return nil, errors.New("nothing to delete")
}

// fileSrc is the text of the main file with its commands parsed, for finding where a node or a style is
// written. The scene tree keeps no range for a styles block or a style name.
type fileSrc struct {
	ast *tenn.Node
}

func newSrc(text string) *fileSrc {
	ast, _ := tenn.Parse(text)
	return &fileSrc{ast: ast}
}

// cmd is the command named name written in the body of owner; the file root has no body, so its
// commands are the top level ones.
func (s *fileSrc) cmd(owner *scene.Node, name string) *tenn.Node {
	cmds := s.ast.Children
	if owner.Kind != scene.KFile {
		c := s.at(owner.Range.Start)
		if c == nil {
			return nil
		}
		cmds = blockCmds(c)
	}
	return named(cmds, name)
}

// cmdIn is the command named name in the block of c.
func (s *fileSrc) cmdIn(c *tenn.Node, name string) *tenn.Node { return named(blockCmds(c), name) }

// at is the command that starts at off.
func (s *fileSrc) at(off int) *tenn.Node {
	var found *tenn.Node
	var walk func(*tenn.Node)
	walk = func(n *tenn.Node) {
		if found != nil {
			return
		}
		if n.Kind == tenn.KindCommand && n.Token != nil && n.Token.Pos == off {
			found = n
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(s.ast)
	return found
}

func blockCmds(c *tenn.Node) []*tenn.Node {
	if c == nil || len(c.Children) == 0 {
		return nil
	}
	if last := c.Children[len(c.Children)-1]; last.Kind == tenn.KindBlockExpr {
		return last.Children
	}
	return nil
}

func named(cmds []*tenn.Node, name string) *tenn.Node {
	for _, c := range cmds {
		if id, ok := c.GetIdent(0); ok && id == name {
			return c
		}
	}
	return nil
}

func rangeOf(c *tenn.Node) scene.Range {
	if c.Token == nil {
		return scene.Range{}
	}
	return scene.Range{Start: c.Token.Pos, End: c.End}
}

// commandText prints one command as the compact printer writes it: a word, a number or a colour bare,
// anything else quoted.
func commandText(name string, args ...string) string {
	nodes := make([]*tenn.Node, 0, len(args))
	for _, a := range args {
		nodes = append(nodes, argNode(a))
	}
	return strings.TrimSpace(tenn.Compact(tenn.NewCommand(name, nodes...)))
}

func argNode(a string) *tenn.Node {
	if a == "" {
		return tenn.NewStrNode(a)
	}
	ast, p := tenn.Parse("x " + a)
	if !p.Errors.HasErrors() && len(ast.Children) == 1 && len(ast.Children[0].Children) == 2 {
		n := ast.Children[0].Children[1]
		if n.Kind != tenn.KindStringLit && n.Kind != tenn.KindExpressionBlock && n.Token != nil && n.Token.Literal == a {
			return n
		}
	}
	return tenn.NewStrNode(a)
}

// diffPreview says what the change does to the lines: the ones that go, the ones that come, and the
// count of lines saved.
func diffPreview(old, out string) string {
	o, n := strings.Split(old, "\n"), strings.Split(out, "\n")
	pre := 0
	for pre < len(o) && pre < len(n) && o[pre] == n[pre] {
		pre++
	}
	suf := 0
	for suf < len(o)-pre && suf < len(n)-pre && o[len(o)-1-suf] == n[len(n)-1-suf] {
		suf++
	}
	gone, added := o[pre:len(o)-suf], n[pre:len(n)-suf]
	const most = 12
	var b strings.Builder
	lines := func(sign string, ls []string) {
		for i, l := range ls {
			if i == most {
				fmt.Fprintf(&b, "%s ... %d more lines\n", sign, len(ls)-i)
				break
			}
			b.WriteString(sign + " " + l + "\n")
		}
	}
	lines("-", gone)
	lines("+", added)
	switch d := len(gone) - len(added); {
	case d > 0:
		fmt.Fprintf(&b, "%d lines fewer\n", d)
	case d < 0:
		fmt.Fprintf(&b, "%d lines more\n", -d)
	default:
		b.WriteString("the same number of lines\n")
	}
	return b.String()
}

// lineStart is the offset of the start of the line that holds off.
func lineStart(text string, off int) int { return strings.LastIndexByte(text[:off], '\n') + 1 }

// indentAt is the white space that starts the line of off.
func indentAt(text string, off int) string {
	ls := lineStart(text, off)
	i := ls
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return text[ls:i]
}

func blank(s string) bool { return strings.TrimLeft(s, " \t") == "" }

// indentUnit guesses the indentation step of the file from its first indented line; two spaces when none.
func indentUnit(text string) string {
	for _, l := range strings.SplitN(text, "\n", 200) {
		if t := strings.TrimLeft(l, " \t"); t != "" && len(t) < len(l) {
			return l[:len(l)-len(t)]
		}
	}
	return "  "
}

func indentLines(s, ind string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = ind + l
		}
	}
	return strings.Join(lines, "\n")
}

// removeEdit deletes a command: with its line when it is alone on it, else with the ; before or after it.
func removeEdit(text string, r scene.Range) Edit {
	ls := lineStart(text, r.Start)
	le := strings.IndexByte(text[r.End:], '\n')
	if le < 0 {
		le = len(text)
	} else {
		le += r.End
	}
	if blank(text[ls:r.Start]) && blank(text[r.End:le]) {
		if le < len(text) {
			le++
		}
		return Edit{ls, le, ""}
	}
	after := r.End
	for after < len(text) && (text[after] == ' ' || text[after] == '\t') {
		after++
	}
	if after < len(text) && text[after] == ';' {
		after++
		for after < len(text) && text[after] == ' ' {
			after++
		}
		return Edit{r.Start, after, ""}
	}
	// the last command of a line has no ; after it: the one before goes, with the spaces
	before := strings.TrimRight(text[:r.Start], " \t")
	if strings.HasSuffix(before, ";") {
		return Edit{len(before) - 1, r.End, ""}
	}
	return Edit{r.Start, r.End, ""}
}
