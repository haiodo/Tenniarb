package render_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// TestClassicFilesDrawThroughScene is the acceptance of E4 for the old files: every classic diagram of
// the repository loads and lays out through go/scene, and every item it holds has a box. A file that
// go/render drew and the scene engine cannot place is the regression this guards.
//
// The error-*.tenn fixtures are broken on purpose, and docs-image-syntax.tenn is a grammar sketch, not
// a document: both are left out.
func TestClassicFilesDrawThroughScene(t *testing.T) {
	files := classicFiles(t)
	if len(files) < 20 {
		t.Fatalf("%d files, the fixture list is wrong", len(files))
	}
	for _, rel := range files {
		t.Run(rel, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("../..", rel))
			if err != nil {
				t.Fatal(err)
			}
			tree, err := scene.Load("../..", rel)
			if tree == nil || err != nil {
				t.Fatalf("load: %v", err)
			}
			root, _ := tenn.Parse(string(src))
			m := model.ParseTenn(root)
			var walk func(e *model.Element)
			walk = func(e *model.Element) {
				if len(e.Items) > 0 && !model.SceneElement(e) {
					checkClassicElement(t, rel, tree, e)
				}
				for _, c := range e.Elements {
					walk(c)
				}
			}
			walk(&m.Element)
		})
	}
}

// classicFiles lists the documents of the repository that hold a classic diagram.
func classicFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"../../examples", "../../docs", "../../web/fixtures"} {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name := e.Name()
			if !strings.HasSuffix(name, ".tenn") || strings.HasSuffix(name, ".saved.tenn") ||
				strings.HasPrefix(name, "error-") || name == "docs-image-syntax.tenn" {
				continue
			}
			out = append(out, strings.TrimPrefix(dir+"/"+name, "../../"))
		}
	}
	return out
}

func checkClassicElement(t *testing.T, rel string, tree *scene.Tree, e *model.Element) {
	t.Helper()
	node := elementNode(tree.File(rel), e.Name)
	if node == nil {
		return // the file declares it in a mount: not a document of its own
	}
	o := render.Options{Measurer: gfx.EstimateText{}}
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.BuildElement(tx, nil, node, scene.Options{Measurer: gfx.EstimateText{},
			EvalVars: exec.SceneEvalVars, Classic: render.ClassicValues(e, render.NewExecContext(e, o))})
	})
	if err != nil {
		t.Fatalf("%s: %v", e.Name, err)
	}
	var empty []string
	var walk func(p *scene.Placed)
	walk = func(p *scene.Placed) {
		if p.Parent != nil && (p.W <= 0 || p.H <= 0) {
			empty = append(empty, fmt.Sprintf("%q %gx%g", p.Src.Name, p.W, p.H))
		}
		for _, k := range p.Kids {
			walk(k)
		}
	}
	walk(screen.Placed())
	if len(empty) > 0 {
		t.Errorf("%s: %d items have no box: %s", e.Name, len(empty), strings.Join(limit(empty, 5), ", "))
	}
}

func limit(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// elementNode finds element name in the tree of file, nested elements included.
func elementNode(n *scene.Node, name string) *scene.Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Elems {
		if c.Name == name {
			return c
		}
		if f := elementNode(c, name); f != nil {
			return f
		}
	}
	return nil
}
