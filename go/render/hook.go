package render

import (
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// ClassicHook is scene.Options.Classic for path when it names an element of the element model, so a
// classic diagram reads the values go/render computes. nil for a file root, an element the file does
// not declare or one whose content is in another file: those keep their own evaluator, or ignore the
// expressions as before.
func ClassicHook(tree *scene.Tree, root, file, path string) func(scene.ClassicRef, string) ([]string, bool) {
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
			o := Options{FontFamily: "Helvetica, Arial, sans-serif"}
			vals = ClassicValues(el, NewExecContext(el, o))
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
