// tenn-render draws one element of a .tenn file as SVG on stdout.
//
//	tenn-render -root DIR FILE ELEMENT > out.svg
//
// FILE is relative to DIR (the workspace root, "." by default). ELEMENT is a dotted path of element
// names; "" or "." is the content of the file itself. Validation errors go to stderr with the exit
// code 1; warnings go to stderr and do not stop the render.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
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
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, path, scene.Options{FontName: "Helvetica, Arial, sans-serif"})
	})
	if err != nil {
		return err
	}
	for _, w := range screen.Warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	var buf bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&buf, sc, screen.Root, gfx.Rect{}, scale); err != nil {
		return err
	}
	_, err = os.Stdout.Write(preserveSpace(buf.Bytes()))
	return err
}

// preserveSpace keeps leading, trailing and repeated spaces of a title: gfx writes plain <text> and
// a viewer collapses whitespace in it unless the document says otherwise.
func preserveSpace(svg []byte) []byte {
	return bytes.Replace(svg, []byte("<svg "), []byte(`<svg xml:space="preserve" `), 1)
}
