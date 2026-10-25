// tenniarb-render draws one element of a classic .tenn diagram as SVG or PNG.
//
//	tenniarb-render [-element NAME] [-scale N] [-bg COLOR] [-dark] [-no-eval] [-font FAMILY] -o out.svg|out.png in.tenn
//
// NAME is an element name or an "A/B" path; the first element with items by default. Exit codes: 1 usage,
// 2 parse errors, 3 render failure.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/tenn"
)

// exportMargin is the margin of Swift's ExportManager: the image is the scene grown by 30 on every side.
const exportMargin = 30

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("tenniarb-render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	element := fs.String("element", "", "element name or A/B path (default: the first element with items)")
	out := fs.String("o", "", "output file, .svg or .png")
	scale := fs.Float64("scale", 2, "device pixels per unit for png, size factor for svg")
	bg := fs.String("bg", "", "background colour name or hex (transparent by default)")
	dark := fs.Bool("dark", false, "dark mode")
	noEval := fs.Bool("no-eval", false, "render ${...} expressions as written instead of evaluating them")
	family := fs.String("font", "Helvetica, Arial, sans-serif", "font family")
	margin := fs.Float64("margin", exportMargin, "margin around the diagram")
	usage := func() int {
		fmt.Fprintln(stderr, "usage: tenniarb-render [-element NAME] [-scale N] [-bg COLOR] [-dark] [-no-eval] [-font FAMILY] -o out.svg|out.png in.tenn")
		fs.PrintDefaults()
		return 1
	}
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 || *out == "" || !(*scale > 0) {
		return usage()
	}
	isSVG := strings.HasSuffix(strings.ToLower(*out), ".svg")
	if !isSVG && !strings.HasSuffix(strings.ToLower(*out), ".png") {
		fmt.Fprintln(stderr, "output extension must be .svg or .png")
		return 1
	}
	text, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "cannot read input:", err)
		return 1
	}

	tree, parser := tenn.Parse(strings.TrimPrefix(string(text), "\xef\xbb\xbf"))
	if parser.Errors.HasErrors() {
		for _, e := range parser.Errors.Errors {
			fmt.Fprintf(stderr, "%s:%d:%d: %s\n", fs.Arg(0), e.Line, e.Col, e.Message)
		}
		return 2
	}
	root := model.ParseTenn(tree)

	var el *model.Element
	if *element != "" {
		if el = render.FindElement(&root.Element, *element); el == nil {
			fmt.Fprintln(stderr, "render failed: element not found:", *element)
			return 3
		}
	} else {
		for _, e := range render.AllElements(&root.Element) {
			if len(e.Items) > 0 {
				el = e
				break
			}
		}
		if el == nil {
			fmt.Fprintln(stderr, "render failed: no elements to render")
			return 3
		}
	}

	var bgColor gfx.Color
	if *bg != "" {
		c, ok := render.ParseColor(*bg)
		if !ok {
			fmt.Fprintln(stderr, "bad colour:", *bg)
			return 1
		}
		bgColor = c
	}

	opts := render.Options{DarkMode: *dark, NoEvaluate: *noEval, FontFamily: *family, Padding: margin}
	if err := write(*out, isSVG, el, opts, *scale, bgColor); err != nil {
		fmt.Fprintln(stderr, "render failed:", err)
		return 3
	}
	return 0
}

func write(path string, isSVG bool, el *model.Element, opts render.Options, scale float64, bg gfx.Color) error {
	if !isSVG {
		data, err := render.PNG(el, opts, scale, bg)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	}
	var buf bytes.Buffer
	if err := render.SVGBackground(&buf, el, opts, scale, bg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
