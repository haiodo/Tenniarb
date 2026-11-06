package mcpserver

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// refactorIn is one restructuring of a node: a new name, a new parent, or both. Both paths are read in
// the tree of file, so a node of a mounted library and a parent in another file are named the way
// PlatformDesign.tenn names them.
type refactorIn struct {
	File   string `json:"file" jsonschema:"path of a .tenn file whose tree holds the node, usually the entry document"`
	Node   string `json:"node" jsonschema:"dotted path of the node, as get_state prints it"`
	Rename string `json:"rename,omitempty" jsonschema:"new name of the node; every use, at and replace path that names it is updated, here and in the files that mount it"`
	Parent string `json:"parent,omitempty" jsonschema:"dotted path of the new parent, an item, an element or . for the file root; the node moves to the end of its block and every use path that names it or a part of it is updated"`
}

// maxRefLines bounds the places an answer lists: a library component is used in hundreds of them, and
// the count and the files are what the agent acts on.
const maxRefLines = 12

// refactor renames or moves a node and every reference to it as one change: the new texts are checked
// together, and nothing is written when the result adds an error.
func (s *Server) refactor(in refactorIn) (string, error) {
	name := strings.TrimSpace(in.Rename)
	switch {
	case in.Node == "":
		return "", errors.New("node is empty")
	case name == "" && in.Parent == "":
		return "", errors.New("nothing to change: set rename or parent")
	}
	abs, rel, err := s.resolve(in.File)
	if err != nil {
		return "", err
	}
	s.host.editMu.Lock()
	defer s.host.editMu.Unlock()

	s.openForEdit(abs)
	before, tree, err := s.diagnose(rel)
	if err != nil {
		return "", err
	}
	if tree == nil || before.Fatal() {
		return "", fmt.Errorf("cannot refactor, fix the errors first:\n%s", summary(before, false, rel))
	}
	n, err := nodeAt(tree, rel, in.Node)
	if err != nil {
		return "", err
	}
	if n.Kind == scene.KFile {
		return "", errors.New("refactor takes a node of the file, not the file")
	}
	if name != "" && nameTaken(n, name) {
		return "", fmt.Errorf("nothing changed: something is named %q here already", name)
	}
	refs, blocked := tree.References(n)
	if blocked != "" {
		return "", errors.New("nothing changed: " + blocked)
	}

	spans := map[string][]span{}
	add := func(file string, sp span) { spans[file] = append(spans[file], sp) }
	var lines []string
	moved := in.Parent != ""
	switch {
	case moved:
		lines, err = s.moveNode(tree, n, in.Parent, name, refs, add)
	case name != "":
		lines, err = renameNode(tree, n, name, refs, add)
	default:
		err = errors.New("nothing to change: set rename or parent")
	}
	if err != nil {
		return "", err
	}

	// The new text of every file the change reaches, from the text the references were found in.
	next := map[string]string{}
	for file, sp := range spans {
		src, ok := tree.Source(file)
		if !ok {
			return "", fmt.Errorf("%s is not loaded", file)
		}
		if next[file], err = applySpans(src, sp); err != nil {
			return "", fmt.Errorf("%s: %v", file, err)
		}
	}
	texts, _, err := s.liveTexts()
	if err != nil {
		return "", err
	}
	if texts == nil {
		texts = map[string]string{}
	}
	for file, text := range next {
		texts[file] = text
	}
	after, _, err := scene.DiagnoseAuto(s.root, rel, texts)
	if err != nil {
		return "", err
	}
	if bad := newErrors(before, after); len(bad) > 0 {
		var b strings.Builder
		for i, e := range bad {
			if i == maxDiags {
				break
			}
			b.WriteString("  " + e.Error() + "\n")
		}
		return "", fmt.Errorf("nothing changed: the refactor adds errors:\n%s", strings.TrimRight(b.String(), "\n"))
	}
	if err := s.applyTexts(tree, next); err != nil {
		return "", err
	}
	shown := len(lines)
	more := ""
	if shown > maxRefLines {
		shown = maxRefLines
		more = fmt.Sprintf("\n… %d more places: read the files", len(lines)-shown)
	}
	return fmt.Sprintf("ok: %s, %d places\n%s%s\n%d files: %s\n%s",
		describe(in, name, n, moved), len(lines), strings.Join(lines[:shown], "\n"), more,
		len(next), strings.Join(sortedKeys(next), ", "), summary(after, true, rel)), nil
}

// describe is the first line of the answer: what happened to the node.
func describe(in refactorIn, name string, n *scene.Node, moved bool) string {
	at := scene.SegText(n.Name)
	switch {
	case moved && name != "":
		return fmt.Sprintf("%s moved to %s and renamed to %s", at, in.Parent, scene.SegText(name))
	case moved:
		return fmt.Sprintf("%s moved to %s", at, in.Parent)
	default:
		return fmt.Sprintf("%s renamed to %s", at, scene.SegText(name))
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// renameNode changes the name of n and the segment every reference writes for it.
func renameNode(tree *scene.Tree, n *scene.Node, name string, refs []scene.Ref, add func(string, span)) ([]string, error) {
	if n.NameArg == (scene.Range{}) {
		return nil, errors.New("nothing changed: the node has no name argument to rename")
	}
	add(n.Pos.File, span{n.NameArg.Start, n.NameArg.End, scene.SegText(name)})
	lines := []string{lineAt(tree, n.Pos.File, n.NameArg.Start) + ": the name"}
	for _, r := range refs {
		if sp, ok := renameRef(r, name); ok {
			add(r.File, sp)
			lines = append(lines, refLine(tree, r))
		}
	}
	return lines, nil
}

// moveNode takes n out of its parent and puts it at the end of the block of the node at parent, in
// whatever file that is, and rewrites every use path that names it or a part of it.
func (s *Server) moveNode(tree *scene.Tree, n *scene.Node, parent, name string, refs []scene.Ref,
	add func(string, span)) ([]string, error) {
	dst := tree.File(n.Pos.File)
	var err error
	if parent != "." {
		if dst, err = nodeAt(tree, n.Pos.File, parent); err != nil {
			return nil, err
		}
	}
	switch dst.Kind {
	case scene.KFile, scene.KElement, scene.KItem:
	default:
		return nil, fmt.Errorf("%s is a %s: a node moves into an item, an element or the file root", parent, dst.Kind)
	}
	for a := dst; a != nil; a = a.Parent {
		if a == n {
			return nil, fmt.Errorf("%s is inside the node: it cannot move into itself", parent)
		}
	}
	if name == "" {
		name = n.Name
	} else if nameTakenIn(dst, n, name) {
		return nil, fmt.Errorf("nothing changed: something named %q is in %s already", name, nameOf(dst))
	}
	// A path of an at or replace is relative to the instance, not to the file, so it cannot be written
	// again from the new place; a link end would point at whatever takes the old place.
	for _, r := range refs {
		switch r.Kind {
		case scene.RefAt, scene.RefReplace:
			return nil, fmt.Errorf("nothing changed: %s names the node inside a use, where the path is relative to the instance: rename it instead, or move it by hand",
				refLine(tree, r))
		case scene.RefLink:
			return nil, fmt.Errorf("nothing changed: a link ends on the node (%s), and a move would leave it pointing elsewhere", refLine(tree, r))
		}
	}

	// The path the references get: the new place, and the segments below the one that names the node.
	newPath := ""
	for _, r := range refs {
		if newPath == "" {
			if newPath, err = destPath(tree, dst, name); err != nil {
				return nil, err
			}
		}
		segs, ok := scene.SplitPath(r.Text)
		if !ok || r.Seg >= len(segs) {
			continue
		}
		parts := append([]string{newPath}, tailText(segs[r.Seg+1:])...)
		add(r.File, span{r.Arg.Start, r.Arg.End, strings.Join(parts, ".")})
	}

	src, ok := tree.Source(n.Pos.File)
	if !ok {
		return nil, fmt.Errorf("%s is not loaded", n.Pos.File)
	}
	dstText, ok := tree.Source(dst.Pos.File)
	if !ok {
		return nil, fmt.Errorf("%s is not loaded", dst.Pos.File)
	}
	block := src[n.Range.Start:n.Range.End]
	if name != "" && name != n.Name {
		block = spliceName(block, n, name)
	}
	unit := indentUnitOf(dstText)
	ind := unit
	if dst.Kind != scene.KFile {
		ind = indentOf(dstText, dst.Range.Start) + unit
	}
	block = reindent(block, indentOf(src, n.Range.Start), ind)
	at, ok := blockEnd(dst)
	if !ok {
		if dst.Mount != nil {
			return nil, fmt.Errorf("%s mounts %s: name an item inside it as the new parent", nameOf(dst), dst.Mount.Pos.File)
		}
		return nil, errors.New("the new parent has no block to move the node into")
	}
	// The closing brace keeps a line of its own: the white space that sat before it goes.
	from := at
	for from > dst.Body.Start+1 && strings.ContainsRune(" \t\n", rune(dstText[from-1])) {
		from--
	}
	parentInd := ""
	if dst.Kind != scene.KFile {
		parentInd = indentOf(dstText, dst.Range.Start)
	}
	lfrom, lto := lineSpan(src, n.Range.Start, n.Range.End)
	add(n.Pos.File, span{lfrom, lto, ""})
	add(dst.Pos.File, span{from, at, "\n" + ind + block + "\n" + parentInd})

	lines := []string{lineAt(tree, n.Pos.File, n.Range.Start) + ": the node"}
	for _, r := range refs {
		lines = append(lines, refLine(tree, r))
	}
	return lines, nil
}

// destPath is the path a reference from the entry document writes for a node called name under dst.
func destPath(tree *scene.Tree, dst *scene.Node, name string) (string, error) {
	seg := scene.SegText(name)
	if seg == "" {
		return "", errors.New("the node has no name: pass rename to give it one, so the references can be written")
	}
	if dst.Kind == scene.KFile {
		return seg, nil
	}
	path, ok := tree.PathOf(dst)
	if !ok {
		return "", fmt.Errorf("%s has no path from the entry document", nameOf(dst))
	}
	return path + "." + seg, nil
}

// renameRef is the edit that makes r name the node by name. False when the text already does.
func renameRef(r scene.Ref, name string) (span, bool) {
	if r.Seg < 0 { // a link end is a name, not a path
		if r.Text == name {
			return span{}, false
		}
		return span{r.Arg.Start, r.Arg.End, scene.SegText(name)}, true
	}
	segs, ok := scene.SplitPath(r.Text)
	if !ok || r.Seg >= len(segs) || segs[r.Seg].IsName(name) {
		return span{}, false
	}
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = s.Text()
	}
	parts[r.Seg] = scene.SegText(name)
	return span{r.Arg.Start, r.Arg.End, strings.Join(parts, ".")}, true
}

// nameTaken reports whether a sibling of n, which a path can name, already has the key of name.
func nameTaken(n *scene.Node, name string) bool { return nameTakenIn(n.Parent, n, name) }

// nameTakenIn reports whether a child of parent other than n has the key of name.
func nameTakenIn(parent, n *scene.Node, name string) bool {
	key := scene.NormalizeName(name)
	if parent == nil || key == "" {
		return false
	}
	for _, c := range append(parent.Nodes(), parent.Content...) {
		if c != n && c.Key == key {
			return true
		}
	}
	return false
}

// tailText is the segments after the one that names the node, as they are written.
func tailText(segs []scene.Segment) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = s.Text()
	}
	return out
}

// spliceName puts name into the name argument of n, which is at the start of block.
func spliceName(block string, n *scene.Node, name string) string {
	from, to := n.NameArg.Start-n.Range.Start, n.NameArg.End-n.Range.Start
	if from < 0 || to < from || to > len(block) {
		return block
	}
	return block[:from] + scene.SegText(name) + block[to:]
}

// reindent moves the lines after the first of block from one level to another.
func reindent(block, from, to string) string {
	if from == to || from == "" {
		return block
	}
	lines := strings.Split(block, "\n")
	for i := 1; i < len(lines); i++ {
		if rest, ok := strings.CutPrefix(lines[i], from); ok {
			lines[i] = to + rest
		}
	}
	return strings.Join(lines, "\n")
}

// refLine is a reference as file:line, so the answer can be checked with read.
func refLine(tree *scene.Tree, r scene.Ref) string { return lineAt(tree, r.File, r.Arg.Start) }

// lineAt is file:line of an offset of a loaded file.
func lineAt(tree *scene.Tree, file string, off int) string {
	src, _ := tree.Source(file)
	return fmt.Sprintf("%s:%d", file, strings.Count(src[:min(off, len(src))], "\n")+1)
}

// applyTexts writes the new text of every changed file: a file open in the app takes the change as one
// undo step, a file on disk is replaced. The texts are checked together first, so a refusal changes
// nothing and only a failure in the middle can leave some files written.
func (s *Server) applyTexts(tree *scene.Tree, next map[string]string) error {
	files := sortedKeys(next)
	// The documents the app has open go first: a refusal there must leave the disk untouched.
	for _, file := range files {
		abs := filepath.Join(s.root, filepath.FromSlash(file))
		d := s.host.live.lookup(abs)
		if d == nil || d.moved {
			continue
		}
		cur, err := s.host.live.text(abs, true)
		if err != nil {
			return err
		}
		var res servicewire.ApplyResult
		if err := d.prov.call(servicewire.MethodApply, servicewire.ApplyArgs{Path: d.path, Base: cur.Version, Text: next[file]}, &res); err != nil {
			return fmt.Errorf("nothing written to disk: Tenniarb did not apply the change to %s: %v", file, err)
		}
		s.noteEdit(abs, file, tree, cur.Text, next[file])
	}
	for _, file := range files {
		abs := filepath.Join(s.root, filepath.FromSlash(file))
		if d := s.host.live.lookup(abs); d != nil && !d.moved {
			continue
		}
		old, _ := tree.Source(file)
		if err := writeAtomic(abs, next[file]); err != nil {
			return fmt.Errorf("%s: %v", file, err)
		}
		s.noteEdit(abs, file, tree, old, next[file])
	}
	return nil
}
