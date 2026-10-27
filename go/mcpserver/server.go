// Package mcpserver is a headless MCP server for .tenn workspaces: agents read, edit, validate
// and render the scene-graph format of docs/spec/tenn-scene.md through text tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server serves one workspace root for one client.
type Server struct {
	root string // absolute, symlinks resolved
	host *Host
}

// New opens the workspace at root with a host of its own.
func New(root string) (*Server, error) { return NewHost().Session(root) }

func open(root string) (*Server, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	return &Server{root: abs}, nil
}

type (
	guideIn struct {
		Topic string `json:"topic,omitempty" jsonschema:"'examples', a spec heading number such as 4.7a, or a word of a heading; empty for examples plus the whole spec"`
	}
	noIn    struct{}
	skillIn struct {
		Path string `json:"path,omitempty" jsonschema:"a file the skill names, such as layout.md; empty for SKILL.md"`
	}
	getStateIn struct {
		File  string `json:"file,omitempty" jsonschema:"only this .tenn file, relative to the workspace root"`
		Path  string `json:"path,omitempty" jsonschema:"with file: dotted path of the node to list from; a mounting element lists the mounted file with paths ready for use"`
		Depth int    `json:"depth,omitempty" jsonschema:"levels of the tree to list, default 2; a node cut off says how many nodes are below it. With find: levels of parts listed under each match, default none"`
		Find  string `json:"find,omitempty" jsonschema:"instead of the tree, every node at any depth whose name contains this text (case-insensitive), at most 100"`
	}
	readIn struct {
		File string `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Path string `json:"path,omitempty" jsonschema:"dotted node path (spec 4.2) inside the file; empty for the whole file"`
	}
	editIn struct {
		File  string   `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Edits []EditOp `json:"edits" jsonschema:"find/replace operations applied in order; to create a missing file send one edit with empty find"`
	}
	renderIn struct {
		File    string  `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Element string  `json:"element,omitempty" jsonschema:"dotted path of an element; empty for the content of the file itself"`
		Scale   float64 `json:"scale,omitempty" jsonschema:"device pixels per scene unit, default 1, at most 8"`
		View    bool    `json:"view,omitempty" jsonschema:"render what the Tenniarb window of this file shows instead; needs the file open in the app"`
	}
	selectIn struct {
		File   string   `json:"file" jsonschema:"path of a .tenn file open in Tenniarb, relative to the workspace root"`
		Nodes  []string `json:"nodes" jsonschema:"dotted node paths (spec 4.2) to select; empty clears the selection"`
		Reveal bool     `json:"reveal,omitempty" jsonschema:"bring the window to front and scroll the first node into view"`
	}
	validateIn struct {
		File string `json:"file,omitempty" jsonschema:"path of a .tenn file; empty validates every .tenn file of the workspace"`
	}
)

// MCP returns the MCP server with all tools registered.
func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "tenniarb-mcp", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Edit .tenn files (UI scenes and classic diagrams) as text. To draw a screen or a diagram call read_skill first; for the format call guide. Y axis points up. Paths are relative to the workspace root. Files open in the Tenniarb app are read and edited live, through its undo.",
	})
	add(srv, "guide", "Return the .tenn scene-graph spec (docs/spec/tenn-scene.md) with short examples. topic narrows it to one section. Y axis points up; use paths follow spec section 4.2.",
		func(in guideIn) (string, error) { return guide(in.Topic), nil })
	add(srv, "read_skill", "Return the Tenniarb design skill: how to draw UI screens and diagrams in .tenn through these tools (the loop, the format, components, layout recipes, review). No path returns SKILL.md; path returns a file it names, such as layout.md.",
		func(in skillIn) (string, error) { return readSkill(in.Path) })
	add(srv, "get_state", "List the .tenn files of the workspace with the node tree of each, 2 levels deep by default (dotted paths per spec 4.2, usable in read, render and use), the renderer of each element (scene or diagram), whether Tenniarb has the file open with unsaved changes, and the errors and warnings of each file. file, path and depth list one subtree; find searches a library for components by name.",
		func(in getStateIn) (string, error) { return s.state(in) })
	add(srv, "read", "Read a .tenn file or one node of it by dotted path (spec 4.2), as source text with line numbers. The text is exactly what edit matches against.",
		func(in readIn) (string, error) { return s.read(in.File, in.Path) })
	add(srv, "edit", "Apply find/replace edits in order to a .tenn file. Atomic: if an edit does not match exactly once (unless all), the result has a parse error or adds a validation error, nothing is written and the errors are returned. Otherwise the file is replaced and the remaining warnings returned. A file open in Tenniarb is changed in the app as one undo step and stays unsaved there; the file on disk is not touched.",
		func(in editIn) (string, error) { return s.edit(in.File, in.Edits) })
	add(srv, "validate", "Parse and validate one .tenn file (with the files it mounts) or the whole workspace. Returns file:line:col errors and warnings. use paths follow spec section 4.2.",
		func(in validateIn) (string, error) { return s.validate(in.File) })
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "render",
		Description: "Render an element of a .tenn file to a PNG image (plus warnings as text). Elements (and the file root) with a use, or an item with children, a slot or auto-layout, draw as UI scenes; other elements as classic diagrams (brain maps: styles, links, layouts, ${...} evaluated). Y axis points up in scenes: pos y grows upward, so a node with larger y is drawn higher. File must have no errors. view renders the Tenniarb window of the file instead.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in renderIn) (*mcp.CallToolResult, any, error) {
		r, err := s.render(in.File, in.Element, in.Scale, in.View)
		if err != nil {
			return failure(err), nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.ImageContent{Data: r.png, MIMEType: "image/png"},
			&mcp.TextContent{Text: r.text},
		}}, nil, nil
	})
	add(srv, "selection", "Show what the user has in Tenniarb: the open documents of this workspace (unsaved or not) and the selection of the front window as dotted paths. Fails when the app is not connected.",
		func(noIn) (string, error) { return s.selection() })
	add(srv, "select", "Select nodes in a file open in Tenniarb (dotted paths, spec 4.2), optionally revealing the first one, so the user sees what you mean.",
		func(in selectIn) (string, error) { return s.selectNodes(in.File, in.Nodes, in.Reveal) })
	return srv
}

// add registers a tool that takes In and answers with text; an error becomes a tool error result.
func add[In any](srv *mcp.Server, name, desc string, fn func(In) (string, error)) {
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: desc},
		func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			text, err := fn(in)
			if err != nil {
				return failure(err), nil, nil
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		})
}

func failure(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}

const (
	defaultDepth = 2
	maxFound     = 100
	maxDiags     = 10
)

// stateHead explains the node lines of get_state.
const stateHead = "node lines: path kind line [renderer] [+N nodes below, cut by depth]\n"

// writeDiags lists the problems of file rel, errors first, at most maxDiags.
func writeDiags(b *strings.Builder, es scene.Errors, rel string) {
	var own scene.Errors
	for _, e := range es {
		if e.Pos.File == rel {
			own = append(own, e)
		}
	}
	sort.SliceStable(own, func(i, j int) bool { return !own[i].Warn && own[j].Warn })
	for i, e := range own {
		if i == maxDiags {
			fmt.Fprintf(b, "  ... %d more: validate %s\n", len(own)-maxDiags, rel)
			break
		}
		b.WriteString("  " + e.Error() + "\n")
	}
}

func writeNode(b *strings.Builder, n *scene.Node, path string, level int, rend *renderers) {
	fmt.Fprintf(b, "%s%s %s %d", strings.Repeat("  ", level), path, n.Kind, n.Pos.Line)
	if n.Kind == scene.KElement && hasContent(n) {
		b.WriteString(" " + rend.of(n))
	}
}

// children are the nodes get_state lists under parent: Nodes and the slots, which a use fills by
// name; the unnamed slot is "default", its path segment (spec 4.7).
func children(parent *scene.Node) []*scene.Node {
	out := parent.Nodes()
	for _, c := range parent.Content {
		if c.Kind == scene.KSlot {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Range.Start < out[j].Range.Start })
	return out
}

func keyOf(n *scene.Node) string {
	if n.Kind == scene.KSlot && n.Key == "" {
		return "default"
	}
	return n.Key
}

func writeTree(b *strings.Builder, parent *scene.Node, prefix string, level, depth int, rend *renderers) {
	for _, n := range children(parent) {
		if keyOf(n) == "" {
			continue
		}
		p := prefix + keyOf(n)
		writeNode(b, n, p, level, rend)
		if depth > 1 {
			b.WriteString("\n")
			writeTree(b, n, p+".", level+1, depth-1, rend)
			continue
		}
		if below := countNodes(n); below > 0 {
			fmt.Fprintf(b, " +%d", below)
		}
		b.WriteString("\n")
	}
}

func countNodes(parent *scene.Node) int {
	n := 0
	for _, c := range children(parent) {
		if keyOf(c) != "" {
			n += 1 + countNodes(c)
		}
	}
	return n
}

// finder lists the nodes whose key contains q (lower case), at most maxFound, each with its parts
// depth levels down, in the paths at and replace take.
type finder struct {
	b     *strings.Builder
	q     string
	depth int
	rend  *renderers
	count int
}

func (f *finder) walk(parent *scene.Node, prefix string) {
	for _, n := range children(parent) {
		if keyOf(n) == "" {
			continue
		}
		p := prefix + keyOf(n)
		if strings.Contains(strings.ToLower(keyOf(n)), f.q) {
			if f.count++; f.count <= maxFound {
				writeNode(f.b, n, p, 1, f.rend)
				f.b.WriteString("\n")
				if f.depth > 0 {
					writeTree(f.b, n, "", 2, f.depth, f.rend)
				}
			}
		}
		f.walk(n, p+".")
	}
}

// listRoot resolves the path a get_state lists from; a mounting element lists the mounted file.
func listRoot(tree *scene.Tree, root *scene.Node, path string) (*scene.Node, error) {
	n, err := tree.Resolve(root, path)
	if err != nil {
		return nil, err
	}
	switch n.Kind {
	case scene.KFile, scene.KElement, scene.KItem, scene.KUse:
		return n, nil
	}
	return nil, fmt.Errorf("path %q names a %s: list its parent", path, n.Kind)
}

// state lists files, their problems and node trees cut at in.Depth. Text, one line a node: JSON of
// a converted library of 3000 nodes took 610 KB.
func (s *Server) state(in getStateIn) (string, error) {
	if in.Path != "" && in.File == "" {
		return "", errors.New("path needs file")
	}
	depth := in.Depth
	if depth <= 0 {
		depth = defaultDepth
	}

	var files []string
	if in.File != "" {
		_, rel, err := s.resolve(in.File)
		if err != nil {
			return "", err
		}
		files = []string{rel}
	} else {
		all, err := s.files()
		if err != nil {
			return "", err
		}
		files = all
	}
	var b strings.Builder
	b.WriteString(stateHead)
	for _, rel := range files {
		es, tree, err := s.diagnose(rel)
		if err != nil {
			return "", err
		}
		f := fileRoot(tree, rel)
		var rend *renderers
		if f != nil {
			rend = newRenderers(tree)
		}
		b.WriteString(rel)
		if f != nil && in.Path == "" && hasContent(f) {
			b.WriteString(" " + rend.of(f))
		}
		if d := s.host.live.lookup(filepath.Join(s.root, filepath.FromSlash(rel))); d != nil {
			b.WriteString(", open in Tenniarb")
			if d.doc.Dirty {
				b.WriteString(", unsaved")
			}
		}
		nerr, nwarn := 0, 0
		for _, e := range es {
			switch {
			case e.Pos.File != rel:
			case e.Warn:
				nwarn++
			default:
				nerr++
			}
		}
		fmt.Fprintf(&b, ": %d errors, %d warnings\n", nerr, nwarn)
		if in.Path == "" && in.Find == "" {
			writeDiags(&b, es, rel)
		}
		if f == nil {
			continue
		}
		from, prefix := f, ""
		if in.Path != "" {
			if from, err = listRoot(tree, f, in.Path); err != nil {
				return "", err
			}
			prefix = strings.TrimSuffix(in.Path, ".") + "."
		}
		if in.Find == "" && in.Path != "" {
			// Relative paths are what at and replace take in an instance of the node.
			fmt.Fprintf(&b, "  paths below %s, as at and replace address them in its instance:\n", in.Path)
			prefix = ""
		}
		if in.Find == "" {
			writeTree(&b, from, prefix, 1, depth, rend)
			continue
		}
		fd := finder{b: &b, q: strings.ToLower(in.Find), depth: in.Depth, rend: rend}
		if fd.walk(from, prefix); fd.count > maxFound {
			fmt.Fprintf(&b, "  ... %d matches, %d listed: make find longer or list from a path\n", fd.count, maxFound)
		} else if fd.count == 0 {
			fmt.Fprintf(&b, "  nothing named like %q\n", in.Find)
		}
	}
	return b.String(), nil
}

func (s *Server) read(file, path string) (string, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	text, live, err := s.text(abs)
	if err != nil {
		return "", err
	}
	total := strings.Count(text, "\n") + 1
	if path == "" {
		note := ""
		if live != nil && live.doc.Dirty {
			note = ", open in Tenniarb with unsaved changes"
		}
		return fmt.Sprintf("%s (%d lines%s)\n%s", rel, total, note, numbered(text, 1, total)), nil
	}
	if e, bad := parseError(text); bad {
		return "", fmt.Errorf("%s has a parse error at %d:%d: %s", rel, e.Line+1, e.Col, e.Message)
	}
	_, tree, err := s.diagnose(rel)
	if err != nil || tree == nil {
		return "", fmt.Errorf("cannot load %s: %v", rel, err)
	}
	n, err := nodeAt(tree, rel, path)
	if err != nil {
		return "", err
	}
	// The node may come from an imported file; its text is the one the tree was built from.
	src, _ := tree.Source(n.Pos.File)
	first := strings.Count(src[:n.Range.Start], "\n") + 1
	last := first + strings.Count(src[n.Range.Start:n.Range.End], "\n")
	return fmt.Sprintf("%s %s %q lines %d-%d\n%s", n.Pos.File, n.Kind, path, first, last, numbered(src, first, last)), nil
}

func (s *Server) validate(file string) (string, error) {
	files := []string{}
	if file != "" {
		_, rel, err := s.resolve(file)
		if err != nil {
			return "", err
		}
		files = append(files, rel)
	} else {
		all, err := s.files()
		if err != nil {
			return "", err
		}
		files = all
	}
	own := ""
	if file != "" {
		own = files[0]
	}
	var all scene.Errors
	seen := map[scene.Error]bool{}
	for _, rel := range files {
		es, _, err := s.diagnose(rel)
		if err != nil {
			return "", err
		}
		for _, e := range es {
			if !seen[e] {
				seen[e] = true
				all = append(all, e)
			}
		}
	}
	if len(files) == 0 {
		return "", errors.New("no .tenn files in the workspace")
	}
	return fmt.Sprintf("%d files checked\n%s", len(files), summary(all, false, own)), nil
}
