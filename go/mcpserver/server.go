// Package mcpserver is a headless MCP server for .tenn workspaces: agents read, edit, validate
// and render the scene-graph format of docs/spec/tenn-scene.md through text tools.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server serves one workspace root.
type Server struct {
	root string // absolute, symlinks resolved
	mu   sync.Mutex
}

// New opens the workspace at root.
func New(root string) (*Server, error) {
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
	noIn   struct{}
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
	}
	validateIn struct {
		File string `json:"file,omitempty" jsonschema:"path of a .tenn file; empty validates every .tenn file of the workspace"`
	}
)

// MCP returns the MCP server with all tools registered.
func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "tenniarb-mcp", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Edit .tenn scene-graph files as text. Call guide first. Y axis points up. Paths are relative to the workspace root.",
	})
	add(srv, "guide", "Return the .tenn scene-graph spec (docs/spec/tenn-scene.md) with short examples. topic narrows it to one section. Y axis points up; use paths follow spec section 4.2.",
		func(in guideIn) (string, error) { return guide(in.Topic), nil })
	add(srv, "get_state", "List the .tenn files of the workspace with the element tree of each (dotted paths per spec 4.2, usable in read, render and use) and the errors and warnings of each file.",
		func(noIn) (string, error) { return s.state() })
	add(srv, "read", "Read a .tenn file or one node of it by dotted path (spec 4.2), as source text with line numbers. The text is exactly what edit matches against.",
		func(in readIn) (string, error) { return s.read(in.File, in.Path) })
	add(srv, "edit", "Apply find/replace edits in order to a .tenn file. Atomic: if an edit does not match exactly once (unless all), the result has a parse error or adds a validation error, nothing is written and the errors are returned. Otherwise the file is replaced and the remaining warnings returned.",
		func(in editIn) (string, error) { return s.edit(in.File, in.Edits) })
	add(srv, "validate", "Parse and validate one .tenn file (with its imports) or the whole workspace. Returns file:line:col errors and warnings. use paths follow spec section 4.2.",
		func(in validateIn) (string, error) { return s.validate(in.File) })
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "render",
		Description: "Render an element of a .tenn file to a PNG image (plus warnings as text). Y axis points up: pos y grows upward, so a node with larger y is drawn higher. Needs macOS. File must have no errors.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in renderIn) (*mcp.CallToolResult, any, error) {
		r, err := s.render(in.File, in.Element, in.Scale)
		if err != nil {
			return failure(err), nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.ImageContent{Data: r.png, MIMEType: "image/png"},
			&mcp.TextContent{Text: r.text},
		}}, nil, nil
	})
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

type diagJSON struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type nodeJSON struct {
	Path     string     `json:"path"`
	Kind     string     `json:"kind"`
	Line     int        `json:"line"`
	Children []nodeJSON `json:"children,omitempty"`
}

type fileJSON struct {
	File        string     `json:"file"`
	Diagnostics []diagJSON `json:"diagnostics"`
	Tree        []nodeJSON `json:"tree"`
}

func diagsJSON(es scene.Errors, only string) []diagJSON {
	out := []diagJSON{}
	for _, e := range es {
		if only != "" && e.Pos.File != only {
			continue
		}
		sev := "error"
		if e.Warn {
			sev = "warning"
		}
		out = append(out, diagJSON{e.Pos.File, e.Pos.Line, e.Pos.Col, sev, e.Msg})
	}
	return out
}

func treeJSON(parent *scene.Node, prefix string) []nodeJSON {
	var out []nodeJSON
	for _, n := range parent.Nodes() {
		if n.Key == "" {
			continue
		}
		p := prefix + n.Key
		out = append(out, nodeJSON{Path: p, Kind: n.Kind.String(), Line: n.Pos.Line, Children: treeJSON(n, p+".")})
	}
	return out
}

// state lists every file with the problems found in it (a problem of an imported file shows under
// that file, which is loaded on its own too).
func (s *Server) state() (string, error) {
	files, err := s.files()
	if err != nil {
		return "", err
	}
	out := struct {
		Files []fileJSON `json:"files"`
	}{Files: []fileJSON{}}
	for _, rel := range files {
		fj := fileJSON{File: rel, Tree: []nodeJSON{}}
		es, tree, err := s.diagnose(rel)
		if err != nil {
			return "", err
		}
		fj.Diagnostics = diagsJSON(es, rel)
		if f := fileRoot(tree, rel); f != nil {
			fj.Tree = treeJSON(f, "")
		}
		out.Files = append(out.Files, fj)
	}
	b, err := json.MarshalIndent(out, "", " ")
	return string(b), err
}

func (s *Server) read(file, path string) (string, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(raw)
	total := strings.Count(text, "\n") + 1
	if path == "" {
		return fmt.Sprintf("%s (%d lines)\n%s", rel, total, numbered(text, 1, total)), nil
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
	return fmt.Sprintf("%d files checked\n%s", len(files), summary(all, false)), nil
}
