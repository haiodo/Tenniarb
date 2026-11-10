//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// A classic title whose block carries the newlines and the indentation of its edges is drawn without them,
// as go/render drew it: they were an empty line above the text and one below, and an item with a picture in
// its title came out a line taller than the old renderer made it.
func TestClassicTitleEdgesAreTrimmed(t *testing.T) {
	// a classic diagram is an element inside another: the top one of a file is a screen
	const text = "element \"Doc\" {\n  element \"E\" {\n    item A {\n      title %{\n    The words\n}\n    }\n  }\n}\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "e.tenn"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	m := model.ReadTenn(text)
	el := &m.Element
	for _, c := range m.Element.Elements {
		for _, d := range c.Elements {
			if d.Name == "E" {
				el = d
			}
		}
	}
	if el.Name != "E" {
		t.Fatal("no element E")
	}
	tree, err := scene.Load(dir, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	ctx := execContextOf(el)
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		screen, err := tree.Build(tx, nil, "Doc.E", scene.Options{
			Measurer: gfx.DefaultMeasurer, Classic: classic.Values(el, ctx)})
		if err != nil {
			t.Fatal(err)
		}
		var walk func(p *scene.Placed)
		walk = func(p *scene.Placed) {
			for _, k := range p.Kids {
				if k.Src != nil && k.Src.Kind == scene.KItem {
					lines := screen.TitleLines(k.Src, 0)
					if len(lines) != 1 {
						t.Fatalf("%d lines, want one: %+v", len(lines), lines)
					}
					if got := strings.Join(lines[0].Parts, ""); got != "The words" {
						t.Errorf("the line %q, want the words without the indentation", got)
					}
				}
				walk(k)
			}
		}
		walk(screen.Placed())
	})
}

// A picture in a title carries the advance of the `@` that names it, as go/render counted it: without that
// space every item with a picture in its title was five points narrower than the old renderer drew it.
const titlePNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg=="

func TestClassicPictureInTitleKeepsItsSpace(t *testing.T) {
	const text = "element \"Doc\" {\n  element \"E\" {\n    item A { title %{A@(pic.png|32)}; image \"pic.png\" @(" + titlePNG + ") }\n  }\n}\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "e.tenn"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	el := &model.ReadTenn(text).Element
	for _, c := range el.Elements {
		for _, d := range c.Elements {
			if d.Name == "E" {
				el = d
			}
		}
	}
	tree, err := scene.Load(dir, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	ctx := execContextOf(el)
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		screen, err := tree.Build(tx, nil, "Doc.E", scene.Options{
			Measurer: gfx.DefaultMeasurer, Classic: classic.Values(el, ctx)})
		if err != nil {
			t.Fatal(err)
		}
		var walk func(p *scene.Placed)
		walk = func(p *scene.Placed) {
			for _, k := range p.Kids {
				if k.Src != nil && k.Src.Kind == scene.KItem {
					// The title is one line of two pictures: the box is as wide as the wider of them plus
					// the padding of the item, and no wider - the blank edges of a block title are gone.
					lines := screen.TitleLines(k.Src, 0)
					if len(lines) != 1 {
						t.Errorf("%q: %d title lines, want one", k.Src.Name, len(lines))
						walk(k)
						continue
					}
					if lines[0].W > k.W {
						t.Errorf("%q: the title is %g wide, the item %g", k.Src.Name, lines[0].W, k.W)
					}
				}
				walk(k)
			}
		}
		walk(screen.Placed())
	})
}
