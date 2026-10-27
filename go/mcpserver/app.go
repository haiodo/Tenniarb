package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/haiodo/tenniarb/go/servicewire"
)

var errNoApp = errors.New("the Tenniarb app is not connected to tenniarb-service: open the file in Tenniarb, or work on the files without it")

// openDoc resolves file to a document the app has open.
func (s *Server) openDoc(file string) (*liveDoc, string, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return nil, "", err
	}
	d := s.host.live.lookup(abs)
	if d == nil {
		if len(s.host.live.all()) == 0 {
			return nil, "", errNoApp
		}
		return nil, "", fmt.Errorf("%s is not open in Tenniarb", rel)
	}
	if d.moved {
		return nil, "", errMoved
	}
	return d, rel, nil
}

func (s *Server) renderView(abs, rel string, scale float64) (*rendered, error) {
	d, _, err := s.openDoc(rel)
	if err != nil {
		return nil, err
	}
	var res servicewire.RenderResult
	if err := d.prov.call(servicewire.MethodRender, servicewire.RenderArgs{Path: d.path, Scale: scale}, &res); err != nil {
		return nil, fmt.Errorf("Tenniarb did not render %s: %v", rel, err)
	}
	return &rendered{png: res.PNG, text: fmt.Sprintf("rendered the Tenniarb window of %s, scale %v", rel, scale)}, nil
}

func (s *Server) selectNodes(file string, nodes []string, reveal bool) (string, error) {
	d, rel, err := s.openDoc(file)
	if err != nil {
		return "", err
	}
	if len(nodes) > 0 {
		_, tree, err := s.diagnose(rel)
		if err != nil || tree == nil {
			return "", fmt.Errorf("cannot load %s: %v", rel, err)
		}
		for _, p := range nodes {
			if _, err := nodeAt(tree, rel, p); err != nil {
				return "", fmt.Errorf("%s: %v", p, err)
			}
		}
	}
	if err := d.prov.call(servicewire.MethodSelect, servicewire.SelectArgs{Path: d.path, Nodes: nodes, Reveal: reveal}, nil); err != nil {
		return "", fmt.Errorf("Tenniarb did not select: %v", err)
	}
	return fmt.Sprintf("ok: %d nodes selected in %s", len(nodes), rel), nil
}

type openJSON struct {
	File    string `json:"file"`
	Unsaved bool   `json:"unsaved"`
}

type selectionJSON struct {
	Open      []openJSON `json:"open"`
	File      string     `json:"file,omitempty"`
	Element   string     `json:"element,omitempty"`
	Nodes     []string   `json:"nodes,omitempty"`
	Elsewhere bool       `json:"outside_workspace,omitempty"`
}

func (s *Server) selection() (string, error) {
	provs := s.host.live.all()
	if len(provs) == 0 {
		return "", errNoApp
	}
	out := selectionJSON{Open: []openJSON{}}
	for abs, d := range s.host.live.within(s.root) {
		r, _ := filepath.Rel(s.root, abs)
		out.Open = append(out.Open, openJSON{File: filepath.ToSlash(r), Unsaved: d.doc.Dirty})
	}
	sort.Slice(out.Open, func(i, j int) bool { return out.Open[i].File < out.Open[j].File })
	// Several apps can be connected; the front window is the one with a selection.
	for _, p := range provs {
		var sel servicewire.Selection
		if err := p.call(servicewire.MethodSelection, struct{}{}, &sel); err != nil || sel.Path == "" {
			continue
		}
		abs := realPath(sel.Path)
		if !within(s.root, abs) {
			out.Elsewhere = true
			continue
		}
		r, _ := filepath.Rel(s.root, abs)
		out.File, out.Element, out.Nodes, out.Elsewhere = filepath.ToSlash(r), sel.Element, sel.Nodes, false
		break
	}
	b, err := json.MarshalIndent(out, "", " ")
	return string(b), err
}
