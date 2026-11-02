package mcpserver

import (
	"fmt"
	"strings"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/optimize"
	"github.com/haiodo/tenniarb/go/scene"
)

// genTool is the input of the gen tool.
type genToolIn struct {
	File    string            `json:"file" jsonschema:"a .tenn file of the workspace"`
	Element string            `json:"element" jsonschema:"the element, item or use whose template to render, by dotted path (Button, Screen.save)"`
	Set     map[string]string `json:"set,omitempty" jsonschema:"parameter values by name, as tenniarb gen -set does"`
}

// gen writes the text the template of a component generates, with the parameters set by name.
func (s *Server) gen(in genToolIn) (string, error) {
	if in.File == "" || in.Element == "" {
		return "", fmt.Errorf("gen needs file and element")
	}
	_, rel, err := s.resolve(in.File)
	if err != nil {
		return "", err
	}
	_, tree, err := s.diagnose(rel)
	if err != nil {
		return "", err
	}
	if tree == nil {
		return "", fmt.Errorf("cannot load %s", rel)
	}
	root := tree.File(rel)
	if root == nil {
		return "", fmt.Errorf("%s is not in the workspace", rel)
	}
	node, err := tree.Resolve(root, in.Element)
	if err != nil {
		return "", err
	}
	text, err := tree.Render(node, in.Set, scene.Options{EvalVars: exec.SceneEvalVars})
	if err != nil {
		return "", err
	}
	return text, nil
}

// suggestTool is the input of the suggest tool.
type suggestToolIn struct {
	File string `json:"file" jsonschema:"a .tenn file of the workspace; empty for every file of the workspace"`
}

// suggest reports what could be tidied in a file, as tenniarb suggest does, and changes nothing.
func (s *Server) suggest(in suggestToolIn) (string, error) {
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
	n := 0
	for _, rel := range files {
		_, tree, err := s.diagnose(rel)
		if err != nil || tree == nil {
			continue
		}
		for _, f := range optimize.Analyze(tree) {
			b.WriteString(f.Text() + "\n")
			n++
		}
	}
	if n == 0 {
		return fmt.Sprintf("%d files, 0 findings", len(files)), nil
	}
	fmt.Fprintf(&b, "%d files, %d findings", len(files), n)
	return b.String(), nil
}
