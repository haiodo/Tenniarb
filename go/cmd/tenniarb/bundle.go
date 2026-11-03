package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/haiodo/tenniarb/go/bundle"
)

const bundleUsage = "usage: tenniarb bundle [-root DIR] [-o OUT] FILE"

// bundleCmd writes FILE with every "element <name> --from-file <path>" replaced by the block that holds
// the content of the mounted file, mounts of mounts included: one file the web renderer can draw,
// as it cannot read the neighbours. The text of FILE stays as written, comments and blank lines kept;
// the text of a mounted file moves to the indentation of its place.
func bundleCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("root", "", "workspace root for --from-file (default: the working directory, or the directory of a file outside it)")
	out := fs.String("o", "", "write to OUT instead of stdout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, bundleUsage)
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 2
	}
	abs, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 2
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 2
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil { // the cycle and root checks compare real paths
		abs = r
	}
	root, err := filepath.EvalSymlinks(genRoot(*rootFlag, cwd, abs))
	if err != nil {
		root = genRoot(*rootFlag, cwd, abs)
	}
	text, err := bundle.File(root, abs)
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 1
	}
	if *out == "" {
		io.WriteString(stdout, text)
		return 0
	}
	if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
		fmt.Fprintln(stderr, "tenniarb bundle:", err)
		return 1
	}
	return 0
}
