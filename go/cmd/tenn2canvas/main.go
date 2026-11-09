// tenn2canvas converts a .tenn file to JSON Canvas.
//
//	tenn2canvas [-root DIR] [-o OUT] file.tenn
//
// root is the workspace root the file belongs to, as with tenn-render. Without -o the canvas goes
// to stdout. What JSON Canvas cannot hold is reported on stderr; an error exits with code 1.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/haiodo/tenniarb/go/jsoncanvas"
	"github.com/haiodo/tenniarb/go/scene"
)

func main() {
	root := flag.String("root", ".", "workspace root: mounted files may not leave it")
	out := flag.String("o", "", "output .canvas file, stdout when empty")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tenn2canvas [-root DIR] [-o OUT] file.tenn")
		flag.PrintDefaults()
	}
	os.Args = append(os.Args[:1], reorder(os.Args[1:])...)
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*root, flag.Arg(0), *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// reorder moves flags before the file name, so "tenn2canvas file.tenn -o out.canvas" works as
// written: the flag package stops at the first other argument.
func reorder(args []string) []string {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") || args[i] == "-" {
			rest = append(rest, args[i])
			continue
		}
		flags = append(flags, args[i])
		if !strings.Contains(args[i], "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, rest...)
}

// fromTree exports a file of one element as that element's content, without a wrapper group.
func fromTree(tree *scene.Tree) (*jsoncanvas.Canvas, []string, error) {
	if tree != nil {
		files := tree.Files()
		if len(files) > 0 {
			if main := tree.File(files[0]); main != nil && len(main.Elems) == 1 && len(main.Content) == 0 {
				return jsoncanvas.FromElement(tree, main.Elems[0])
			}
		}
	}
	return jsoncanvas.FromTree(tree)
}

func run(root, file, out string) error {
	tree, err := scene.LoadAuto(root, file, nil)
	if err != nil {
		return err
	}
	c, warnings, err := fromTree(tree)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	data, err := jsoncanvas.Marshal(c)
	if err != nil {
		return err
	}
	if out == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(out, data, 0o644)
}
