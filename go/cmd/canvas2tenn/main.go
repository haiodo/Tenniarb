// canvas2tenn converts a JSON Canvas file (.canvas) to the .tenn scene format.
//
//	canvas2tenn [-o OUT] file.canvas
//
// Without -o the text goes to stdout. Warnings about what the format cannot hold go to stderr;
// an error exits with code 1.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/jsoncanvas"
)

func main() {
	out := flag.String("o", "", "output .tenn file, stdout when empty")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: canvas2tenn [-o OUT] file.canvas")
		flag.PrintDefaults()
	}
	os.Args = append(os.Args[:1], reorder(os.Args[1:])...)
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// reorder moves flags before the file name, so "canvas2tenn file.canvas -o out.tenn" works as
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

func run(file, out string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	c, err := jsoncanvas.Parse(data)
	if err != nil {
		return err
	}
	text, warnings, err := jsoncanvas.ToTenn(c, strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if out == "" {
		_, err = os.Stdout.WriteString(text)
		return err
	}
	return os.WriteFile(out, []byte(text), 0o644)
}
