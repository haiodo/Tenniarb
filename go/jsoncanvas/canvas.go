// Package jsoncanvas reads and writes JSON Canvas 1.0 (https://github.com/obsidianmd/jsoncanvas)
// and converts it to and from the .tenn scene format.
package jsoncanvas

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Canvas is one .canvas file: a flat list of nodes and the edges between them.
type Canvas struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node is a canvas node. The fields of one type are empty for the others.
type Node struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"` // text, file, link or group
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Color  string  `json:"color,omitempty"` // "1".."6"
	Text   string  `json:"text,omitempty"`
	File   string  `json:"file,omitempty"`
	// Subpath is the heading or block a file node points at.
	Subpath string `json:"subpath,omitempty"`
	URL     string `json:"url,omitempty"`
	// Label is the title of a group.
	Label           string `json:"label,omitempty"`
	Background      string `json:"background,omitempty"`
	BackgroundStyle string `json:"backgroundStyle,omitempty"`
}

// Edge is an edge between two nodes. Sides and ends are optional in the format.
type Edge struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	FromSide string `json:"fromSide,omitempty"`
	FromEnd  string `json:"fromEnd,omitempty"`
	ToNode   string `json:"toNode"`
	ToSide   string `json:"toSide,omitempty"`
	ToEnd    string `json:"toEnd,omitempty"`
	Color    string `json:"color,omitempty"`
	Label    string `json:"label,omitempty"`
}

// Parse reads a .canvas file. Unknown fields are ignored; a broken structure names the node or edge
// it is in.
func Parse(data []byte) (*Canvas, error) {
	var c Canvas
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("json canvas: %w", err)
	}
	ids := make(map[string]bool, len(c.Nodes))
	for i := range c.Nodes {
		n := &c.Nodes[i]
		if n.ID == "" {
			return nil, fmt.Errorf("json canvas: node %d has no id", i+1)
		}
		if ids[n.ID] {
			return nil, fmt.Errorf("json canvas: node %q is repeated", n.ID)
		}
		ids[n.ID] = true
		switch n.Type {
		case "text", "group":
		case "file":
			if n.File == "" {
				return nil, fmt.Errorf("json canvas: file node %q has no file", n.ID)
			}
		case "link":
			if n.URL == "" {
				return nil, fmt.Errorf("json canvas: link node %q has no url", n.ID)
			}
		default:
			return nil, fmt.Errorf("json canvas: node %q has unknown type %q", n.ID, n.Type)
		}
	}
	edges := make(map[string]bool, len(c.Edges))
	for i := range c.Edges {
		e := &c.Edges[i]
		if e.ID == "" {
			return nil, fmt.Errorf("json canvas: edge %d has no id", i+1)
		}
		if edges[e.ID] {
			return nil, fmt.Errorf("json canvas: edge %q is repeated", e.ID)
		}
		edges[e.ID] = true
		if !ids[e.FromNode] {
			return nil, fmt.Errorf("json canvas: edge %q: fromNode %q is not a node", e.ID, e.FromNode)
		}
		if !ids[e.ToNode] {
			return nil, fmt.Errorf("json canvas: edge %q: toNode %q is not a node", e.ID, e.ToNode)
		}
	}
	return &c, nil
}

// Marshal writes a .canvas file: two spaces of indentation, empty arrays for missing lists.
func Marshal(c *Canvas) ([]byte, error) {
	if c == nil {
		return nil, errors.New("json canvas: nil canvas")
	}
	out := *c
	if out.Nodes == nil {
		out.Nodes = []Node{}
	}
	if out.Edges == nil {
		out.Edges = []Edge{}
	}
	b, err := json.MarshalIndent(&out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json canvas: %w", err)
	}
	return append(b, '\n'), nil
}
