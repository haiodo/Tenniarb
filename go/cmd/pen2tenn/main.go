// pen2tenn converts pen.dev design files (.pen) to the .tenn scene format.
//
//	pen2tenn -o OUTDIR file.pen [file.pen ...]
//
// It writes one .tenn per .pen, libraries imported by the files included, plus a .map file with the
// pen id and the path of every node and a .vars file with the variables that were resolved to colors.
// The summary on stderr lists what could not be converted.
// With -check each output is also loaded by go/scene, and the layout of every screen is compared
// with the layout gfx/pen makes of the original.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"
	"github.com/haiodo/tenniarb/go/penimport"
)

func main() {
	out := flag.String("o", "", "output directory")
	check := flag.Bool("check", false, "load the output with go/scene and compare screen layouts with gfx/pen")
	all := flag.Bool("check-all", false, "with -check: compare every top level node, not only those of 400 x 300 and more")
	fonts := flag.String("fonts", "", "with -check: directory of fonts to measure text with, instead of the estimate")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: pen2tenn -o OUTDIR [-check] file.pen ...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*out, flag.Args(), *check, *all, *fonts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out string, paths []string, check, all bool, fonts string) error {
	res, err := penimport.Convert(paths, penimport.Options{})
	if err != nil {
		return err
	}
	if err := res.Write(out); err != nil {
		return err
	}
	for _, f := range res.Files {
		report(f, res.Tenn(f))
	}
	if !check {
		return nil
	}
	return runCheck(res, out, all, fonts)
}

func report(f *penimport.File, tenn string) {
	src, _ := os.ReadFile(f.Path)
	fmt.Fprintf(os.Stderr, "%s: pen %d bytes %d lines, tenn %d bytes %d lines, %d warnings\n", f.Name,
		len(src), bytes.Count(src, []byte("\n")), len(tenn), strings.Count(tenn, "\n"), len(f.Warnings))
	for _, w := range f.Warnings {
		fmt.Fprintln(os.Stderr, "  warning:", w)
	}
	keys := make([]string, 0, len(f.Unsupported))
	for k := range f.Unsupported {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  unsupported: %s (%d)\n", k, f.Unsupported[k])
	}
	if len(f.Variables) > 0 {
		n := 0
		for _, c := range f.Variables {
			n += c
		}
		fmt.Fprintf(os.Stderr, "  variables resolved to light: %d names, %d uses\n", len(f.Variables), n)
	}
}

func runCheck(res *penimport.Result, out string, all bool, fonts string) error {
	var m gfx.TextMeasurer = gfx.EstimateText{}
	if fonts != "" {
		if _, err := offscreen.RegisterFonts(fonts); err != nil {
			return err
		}
		r, err := offscreen.Default()
		if err != nil {
			return err
		}
		m = r.(gfx.TextMeasurer)
	}
	for _, f := range res.Files {
		r, err := penimport.CompareWith(f, out, all, m)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, r)
	}
	return nil
}
