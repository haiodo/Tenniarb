// tenn-render draws one element of a .tenn file as SVG on stdout.
//
//	tenn-render -root DIR FILE ELEMENT > out.svg
//
// FILE is relative to DIR (the workspace root, "." by default). ELEMENT is a dotted path of element
// names, a name with punctuation in double quotes (S."2 Пространства - пусто"); "" or "." is the
// content of the file itself. Validation errors go to stderr with the exit code 1; warnings go to
// stderr and do not stop the render.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

func main() {
	root := flag.String("root", ".", "workspace root: mounted files may not leave it")
	scale := flag.Float64("scale", 1, "SVG scale")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tenn-render [-root DIR] [-scale N] FILE ELEMENT > out.svg")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}
	path := ""
	if flag.NArg() == 2 && flag.Arg(1) != "." {
		path = flag.Arg(1)
	}
	if err := run(*root, flag.Arg(0), path, *scale); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root, file, path string, scale float64) error {
	tree, err := scene.Load(root, file)
	if err != nil {
		return err
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	classic := classicHook(tree, root, file, path)
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, path, scene.Options{Measurer: textMeasurer(), FontName: "Helvetica, Arial, sans-serif",
			EvalVars: exec.SceneEvalVars, Classic: classic})
	})
	if err != nil {
		return err
	}
	for _, w := range screen.Warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	var buf bytes.Buffer
	if err := screen.WriteSVG(&buf, sc, gfx.Rect{}, scale); err != nil {
		return err
	}
	_, err = os.Stdout.Write(preserveSpace(buf.Bytes()))
	return err
}

// classicHook is scene.Options.Classic for path when it names an element of the element model, so a
// classic diagram reads the values go/render computes. nil for a file root, an element the file does
// not declare or one whose content is in another file: those keep their own evaluator, or ignore the
// expressions as before.
func classicHook(tree *scene.Tree, root, file, path string) func(scene.ClassicRef, string) ([]string, bool) {
	if path == "" {
		return nil
	}
	src, ok := fileSource(tree, root, file)
	if !ok {
		return nil
	}
	parsed, parser := tenn.Parse(strings.TrimPrefix(src, "\xef\xbb\xbf"))
	if parser.Errors.HasErrors() {
		return nil
	}
	el := findElement(&model.ParseTenn(parsed).Element, path)
	if el == nil || el.FromFile != "" {
		return nil
	}
	var vals func(scene.ClassicRef, string) ([]string, bool)
	return func(at scene.ClassicRef, prop string) ([]string, bool) {
		if vals == nil {
			o := render.Options{Measurer: textMeasurer(), FontFamily: "Helvetica, Arial, sans-serif"}
			vals = render.ClassicValues(el, render.NewExecContext(el, o))
		}
		return vals(at, prop)
	}
}

// fileSource is the text of file as scene.Load keyed it: relative to the workspace root, with slashes.
func fileSource(tree *scene.Tree, root, file string) (string, bool) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(absRoot, file)
	}
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil {
		return "", false
	}
	return tree.Source(filepath.ToSlash(rel))
}

// findElement follows a dotted scene path (spec 4.2) through the elements of the model: a quoted
// segment is a name as written, a bare one its key.
func findElement(top *model.Element, path string) *model.Element {
	segs, ok := scene.SplitPath(path)
	if !ok {
		return nil
	}
	el := top
	for _, seg := range segs {
		var next *model.Element
		for _, c := range el.Elements {
			if seg.IsName(c.Name) {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		el = next
	}
	return el
}

// preserveSpace keeps leading, trailing and repeated spaces of a title: gfx writes plain <text> and
// a viewer collapses whitespace in it unless the document says otherwise.
func preserveSpace(svg []byte) []byte {
	return bytes.Replace(svg, []byte("<svg "), []byte(`<svg xml:space="preserve" `), 1)
}
