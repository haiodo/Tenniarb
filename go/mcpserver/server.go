// Package mcpserver is a headless MCP server for .tenn workspaces: agents read, edit, validate
// and render UI scenes and classic diagrams through text tools.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server serves one workspace root for one client.
type Server struct {
	root  string // absolute, symlinks resolved
	host  *Host
	agent *agentSession // this client, as the presence of the open documents knows it
	// cursor is how far wait_comment has told this session of the comments of the user.
	cursor commentCursor
}

// Close ends the session: the documents it worked on lose its presence.
func (s *Server) Close() {
	if s.agent != nil {
		s.host.presence.close(s.agent)
	}
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
		Topic string `json:"topic,omitempty" jsonschema:"'examples' for the examples only; a reference section number such as 3, or a word of its heading such as styles; empty for examples plus the whole reference"`
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
		File  string   `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Path  string   `json:"path,omitempty" jsonschema:"dotted node path inside the file, a name with punctuation in double quotes (S.\"2 Пространства - пусто\"); empty for the whole file"`
		Paths []string `json:"paths,omitempty" jsonschema:"several nodes of the file in one call, printed one after another"`
	}
	editIn struct {
		File  string   `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Edits []EditOp `json:"edits" jsonschema:"find/replace operations applied in order; to create a missing file send one edit with empty find"`
	}
	designIn struct {
		File string     `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Ops  []DesignOp `json:"ops" jsonschema:"operations in order; each addresses a node by its dotted path"`
	}
	layoutIn struct {
		File    string `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Element string `json:"element,omitempty" jsonschema:"dotted path of a scene element; empty for the content of the file itself"`
		Depth   int    `json:"depth,omitempty" jsonschema:"levels of items to list, default 0: one line per top-level item, then only the problems; 3 lists the box of every item"` //nolint:lll
	}
	renderIn struct {
		File    string  `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		Element string  `json:"element,omitempty" jsonschema:"dotted path of an element; empty for the content of the file itself"`
		Scale   float64 `json:"scale,omitempty" jsonschema:"device pixels per scene unit, default 1, at most 8"`
		View    bool    `json:"view,omitempty" jsonschema:"render what the Tenniarb window of this file shows instead; needs the file open in the app"`
		Inspect bool    `json:"inspect,omitempty" jsonschema:"add the computed geometry to the text: the box of every item, the items that leave their parent and overlapping pairs, in scene units"`
	}
	selectIn struct {
		File   string   `json:"file" jsonschema:"path of a .tenn file open in Tenniarb, relative to the workspace root; or a link from the selection tool, File.tenn#Element/node (docs/links.md), which also gives nodes"`
		Nodes  []string `json:"nodes,omitempty" jsonschema:"dotted node paths to select; empty clears the selection, or takes the place of a link"`
		Reveal bool     `json:"reveal,omitempty" jsonschema:"bring the window to front and scroll the first node into view"`
	}
	validateIn struct {
		File string `json:"file,omitempty" jsonschema:"path of a .tenn file; empty validates every .tenn file of the workspace"`
	}
)

// MCP returns the MCP server with all tools registered.
func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "tenniarb-mcp", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Edit .tenn files (UI scenes and classic diagrams) as text. To draw a screen or a diagram call read_skill first; for the format call guide. Y axis points up. Paths are relative to the workspace root. Change a file only through edit, never with your own Write/Edit tools: the app holds the document and the user follows the edits in its window. Files open in the Tenniarb app are read and edited live, through its undo. The user may leave comments on a design: after a task call comments, answer with reply or resolve, and loop wait_comment to stay on duty.",
	})
	srv.AddReceivingMiddleware(s.learnClient)
	add(srv, "guide", "Return short examples and a reference of the .tenn commands the scene engine runs: elements, kinds and --from-file, items and auto-layout, pos and the Y axis, use with at/replace/slot, parameters, styles and use-style, colors and text, traps, and what is not supported. topic narrows it to one section by number (3) or heading word (styles); 'examples' returns the examples only.",
		func(in guideIn) (string, error) { return guide(in.Topic), nil })
	add(srv, "read_skill", "Return the Tenniarb design skill: how to draw UI screens and diagrams in .tenn through these tools (the loop, the format, components, layout recipes, review). No path returns SKILL.md; path returns a file it names, such as layout.md.",
		func(in skillIn) (string, error) { return readSkill(in.Path) })
	add(srv, "get_state", "List the .tenn files of the workspace with the node tree of each, 2 levels deep by default (dotted paths, usable in read, render and use), the renderer of each element (scene or diagram), whether Tenniarb has the file open with unsaved changes, and the errors and warnings of each file. file, path and depth list one subtree; find searches a library for components by name.",
		func(in getStateIn) (string, error) {
			if in.File != "" {
				s.noteNode("get_state", in.File, in.Path)
			}
			return s.state(in)
		})
	add(srv, "design", "Change a .tenn file by operations on nodes: set a property, move a node by dx dy, insert commands into a parent, delete a node, replace text everywhere. Paths are the dotted ones of get_state, so nothing has to match as text. All operations of one call go as one change: nothing is written if the result has a parse error or adds a validation error, and a file open in Tenniarb takes it as one undo step, unsaved.",
		func(in designIn) (string, error) {
			return s.design(in.File, in.Ops)
		})
	add(srv, "layout", "Report the geometry of a scene element without drawing it: the size of the scene, the box of every item a few levels deep, the items that leave the plate they sit on, and overlapping siblings, in scene units. Use it instead of a render when the question is about sizes, overflow or spacing.",
		func(in layoutIn) (string, error) {
			s.noteNode("layout", in.File, in.Element)
			return s.layoutOf(in.File, in.Element, in.Depth)
		})
	add(srv, "read", "Read a .tenn file or one node of it by dotted path, as source text with line numbers. A name with punctuation is written in quotes, as get_state prints it. The text is exactly what edit matches against. A path that names an element with --from-file or a library of them gives the content of the mounted file, not the one-line declaration.",
		func(in readIn) (string, error) {
			s.noteNode("read", in.File, in.Path)
			if len(in.Paths) == 0 {
				return s.read(in.File, in.Path)
			}
			var b strings.Builder
			for i, p := range in.Paths {
				got, err := s.read(in.File, p)
				if err != nil {
					return "", err
				}
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString(got)
			}
			return b.String(), nil
		})
	add(srv, "edit", "Apply find/replace edits in order to a .tenn file. Atomic: if an edit does not match exactly once (unless all), the result has a parse error or adds a validation error, nothing is written and the errors are returned. Otherwise the file is replaced and the remaining warnings returned. A file open in Tenniarb is changed in the app as one undo step and stays unsaved there; the file on disk is not touched.",
		func(in editIn) (string, error) { return s.edit(in.File, in.Edits) })
	add(srv, "gen", "Print the text the template of a component generates (spec 5c): the template with its ${parameters} filled and its slots taken from the use, for code generation. set gives the parameter values by name; without it the defaults of the element are used, and where the element is the use, the block of the use.",
		func(in genToolIn) (string, error) { return s.gen(in) })
	add(srv, "suggest", "Report what could be tidied in a .tenn file, as tenniarb suggest does: repeated structures that could be one component, nodes that differ only in a colour or a size and could be one style, literals used again and again, dead styles and overrides that change nothing. Reads only, changes nothing. file is one file; empty lists the whole workspace.",
		func(in suggestToolIn) (string, error) { return s.suggest(in) })
	add(srv, "validate", "Parse and validate one .tenn file (with the files it mounts) or the whole workspace. Returns file:line:col errors and warnings. use paths are dotted.",
		func(in validateIn) (string, error) {
			if in.File != "" {
				s.noteNode("validate", in.File, "")
			}
			return s.validate(in.File)
		})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "render",
		Description: "Render an element of a .tenn file to a PNG image (plus warnings as text). Elements (and the file root) with a use, or an item with children, a slot or auto-layout, draw as UI scenes; other elements as classic diagrams (brain maps: styles, links, layouts, ${...} evaluated). Y axis points up in scenes: pos y grows upward, so a node with larger y is drawn higher. File must have no errors. view renders the Tenniarb window of the file instead. inspect adds the geometry: sizes, boxes, items leaving their parent, overlaps — read the numbers instead of measuring the image.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in renderIn) (*mcp.CallToolResult, any, error) {
		started := time.Now()
		s.noteNode("render", in.File, in.Element)
		r, err := s.render(in.File, in.Element, in.Scale, in.View)
		if err == nil && in.Inspect && r.screen != nil {
			if g := inspectScene(r.screen, inspectDepth); g != "" {
				r.text += "\n" + g
			}
		}
		if err != nil {
			recordCall("render", in, len(err.Error()), started)
			return failure(err), nil, nil
		}
		recordCall("render", in, len(r.text)+len(r.png), started)
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.ImageContent{Data: r.png, MIMEType: "image/png"},
			&mcp.TextContent{Text: r.text},
		}}, nil, nil
	})
	add(srv, "selection", "Show what the user has in Tenniarb: the open documents of this workspace (unsaved or not) and the selection of the front window as dotted paths, with a link to it (link, link_text) to share. Fails when the app is not connected.",
		func(noIn) (string, error) { return s.selection() })
	add(srv, "select", "Select nodes in a file open in Tenniarb (dotted paths), optionally revealing the first one, so the user sees what you mean.",
		func(in selectIn) (string, error) {
			in, err := s.expandLink(in)
			if err != nil {
				return "", err
			}
			first := ""
			if len(in.Nodes) > 0 {
				first = in.Nodes[0]
			}
			s.noteNode("select", in.File, first)
			return s.selectNodes(in.File, in.Nodes, in.Reveal)
		})
	s.addCommentTools(srv)
	return srv
}

// learnClient takes the client name from the initialize request at the first tool call: the SDK's
// InitializedHandler was never called by a v1.8.0 server in a test.
func (s *Server) learnClient(next mcp.MethodHandler) mcp.MethodHandler {
	var named atomic.Bool
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if ss, ok := req.GetSession().(*mcp.ServerSession); ok && !named.Load() && method == "tools/call" && s.agent != nil {
			if p := ss.InitializeParams(); p != nil && p.ClientInfo != nil {
				named.Store(true)
				s.host.presence.setName(s.agent, p.ClientInfo.Name)
			}
		}
		return next(ctx, method, req)
	}
}

// add registers a tool that takes In and answers with text; an error becomes a tool error result.
func add[In any](srv *mcp.Server, name, desc string, fn func(In) (string, error)) {
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: desc},
		func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			started := time.Now()
			text, err := fn(in)
			if err != nil {
				recordCall(name, in, len(err.Error())+6, started)
				return failure(err), nil, nil
			}
			recordCall(name, in, len(text), started)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		})
}

// callLog is where recordCall writes: TENNIARB_MCP_LOG when set ("off" silences it), else a file in the
// temporary directory. Always on, because the harness that starts the server decides its environment:
// setting the variable there is a change nobody remembers to make.
func callLog() string {
	if p, ok := os.LookupEnv("TENNIARB_MCP_LOG"); ok {
		if p == "off" {
			return ""
		}
		return p
	}
	return filepath.Join(os.TempDir(), "tenniarb-mcp-calls.log")
}

// recordCall appends one line per tool call to callLog, so a run can be measured: the size of what the
// agent sent and got back is what costs tokens, and the time is what costs waiting.
func recordCall(name string, in any, out int, started time.Time) {
	path := callLog()
	if path == "" {
		return
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxCallLog {
		os.Rename(path, path+".1")
	}
	args, _ := json.Marshal(in)
	line, _ := json.Marshal(map[string]any{
		"tool": name, "in": len(args), "out": out, "ms": time.Since(started).Milliseconds(),
	})
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}

func failure(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}

const (
	defaultDepth = 2
	maxFound     = 100
	maxDiags     = 10
	// maxState bounds the whole-workspace listing: the overview an agent starts from, not every node there is.
	maxState = 12000
	// maxCallLog bounds the call log; it is rolled to .1 and started again.
	maxCallLog = 1 << 20
)

// cutState keeps the head of the workspace listing, at a line boundary, and says how to see the rest.
func cutState(s string) string {
	if len(s) <= maxState {
		return s
	}
	at := strings.LastIndexByte(s[:maxState], '\n') + 1
	if at <= 0 {
		at = maxState
	}
	return s[:at] + "… cut: get_state {file} lists one file in full, find searches every file\n"
}

// stateHead explains the node lines of get_state.
const stateHead = "node lines: path kind line [renderer] [kind=TYPE] [params=a,b] [styles=a,b] [groups=name:one-of(a b)] [args=a=v,b=v] [+N nodes below, cut by depth]\n"

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
	writeNodeNotes(b, n)
}

// writeNodeNotes adds what a node declares about itself, so that an agent picks a component without
// reading it: its kind, the parameters a use may set, the styles it holds and the groups it offers
// (spec 5b, 5c).
func writeNodeNotes(b *strings.Builder, n *scene.Node) {
	if n.Type != "" {
		b.WriteString(" kind=" + n.Type)
	}
	if len(n.Params) > 0 {
		names := make([]string, len(n.Params))
		for i, p := range n.Params {
			names[i] = p.Name
			if p.Default == nil {
				names[i] += "!"
			}
		}
		b.WriteString(" params=" + strings.Join(names, ","))
	}
	if len(n.Styles) > 0 {
		names := make([]string, 0, len(n.Styles))
		for name := range n.Styles {
			names = append(names, name)
		}
		sort.Strings(names)
		b.WriteString(" styles=" + strings.Join(names, ","))
	}
	if len(n.StyleGroups) > 0 {
		var gs []string
		for _, g := range n.StyleGroups {
			kind := "one-of"
			if g.AnyOf {
				kind = "any-of"
			}
			gs = append(gs, fmt.Sprintf("%s:%s(%s)", g.Name, kind, strings.Join(g.Styles, " ")))
		}
		b.WriteString(" groups=" + strings.Join(gs, ","))
	}
	if n.Kind == scene.KUse {
		if args := useArgs(n); args != "" {
			b.WriteString(" args=" + args)
		}
	}
}

// useArgs is the parameter values a use sets, as name=value, so an agent knows the variant of an
// instance without reading it. A value with a space or a quote is quoted back.
func useArgs(n *scene.Node) string {
	var parts []string
	for _, ov := range n.Block {
		if ov.Kind != scene.OvProp || ov.Prop == nil {
			continue
		}
		var vals []string
		for _, a := range ov.Prop.Args {
			s, _ := a.GetIdentText()
			if strings.ContainsAny(s, " \t\"") {
				s = strconv.Quote(s)
			}
			vals = append(vals, s)
		}
		parts = append(parts, ov.Prop.Name+"="+strings.Join(vals, " "))
	}
	return strings.Join(parts, " ")
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

// keyOf is the path segment of n, quoted when the name needs it (spec 4.2), so that the path get_state
// prints goes straight into read, at or select.
func keyOf(n *scene.Node) string {
	if n.Kind == scene.KSlot && n.Name == "" {
		return "default"
	}
	return scene.SegText(n.Name)
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
		if tree != nil && tree.Entry() != "" {
			b.WriteString(" (in " + tree.Entry() + ")")
		}
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
	if in.File == "" {
		return cutState(b.String()), nil
	}
	return b.String(), nil
}

// mountedReadLines bounds what read prints of a mounted file: enough for a board, not the whole library.
const mountedReadLines = 400

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
		show := min(total, mountedReadLines)
		head := fmt.Sprintf("%s (%d lines%s)\n", rel, total, note)
		if show < total {
			head += "… the first " + fmt.Sprint(show) + " lines; get_state lists the nodes, read one by path (or several with paths) to see the rest\n"
		}
		return head + numbered(text, 1, show), nil
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
	// An element with --from-file shows what its file has: its own line says nothing about the screen.
	if n.Mount != nil {
		file := n.Mount.Pos.File
		src, ok := tree.Source(file)
		if !ok || strings.TrimSpace(src) == "" {
			return "", fmt.Errorf("%s is mounted but its file %s is not loaded", path, file)
		}
		lines := strings.Count(src, "\n") + 1
		show := min(lines, mountedReadLines)
		head := fmt.Sprintf("%s %q lines 1-%d of %d, the content of the element (mounted file)\n", file, path, show, lines)
		if show < lines {
			head += "… more lines; get_state lists the nodes, read one by path\n"
		}
		return head + numbered(src, 1, show), nil
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
	var notes string
	seen := map[scene.Error]bool{}
	for _, rel := range files {
		es, tree, err := s.diagnose(rel)
		if err != nil {
			return "", err
		}
		if tree != nil && tree.Entry() != "" {
			notes += fmt.Sprintf("%s checked in the context of %s\n", rel, tree.Entry())
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
	return fmt.Sprintf("%d files checked\n%s%s", len(files), notes, summary(all, false, own)), nil
}
