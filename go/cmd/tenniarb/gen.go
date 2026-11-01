package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

const genUsage = "usage: tenniarb gen [-root DIR] [-set name=value]... FILE ELEMENT"

// setFlag collects the -set name=value pairs: the values of the parameters of the component.
type setFlag map[string]string

func (s setFlag) String() string { return "" }

func (s setFlag) Set(pair string) error {
	name, value, ok := strings.Cut(pair, "=")
	if !ok || name == "" {
		return fmt.Errorf("want name=value, got %q", pair)
	}
	s[name] = value
	return nil
}

// gen prints the text the template of ELEMENT generates (spec 5c): its parameters filled from the block
// of the use, its slots from the nodes the use passes, -set over the top. ELEMENT is a path of names, a
// component element or a use of one. Exit 0 done, 1 errors in the file or the template, 2 usage or an
// unreadable file.
func gen(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("root", "", "workspace root for --from-file (default: the working directory, or the directory of a file outside it)")
	values := setFlag{}
	fs.Var(values, "set", "value of a parameter: name=value, repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, genUsage)
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb gen:", err)
		return 2
	}
	abs, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb gen:", err)
		return 2
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintln(stderr, "tenniarb gen:", err)
		return 2
	}
	tree, err := scene.Load(genRoot(*rootFlag, cwd, abs), abs)
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb gen:", err)
		if tree == nil {
			return 2
		}
		return 1
	}
	node, err := tree.Resolve(tree.File(tree.Files()[0]), fs.Arg(1))
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb gen:", err)
		return 1
	}
	text, err := tree.Render(node, values, scene.Options{EvalVars: exec.SceneEvalVars})
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb gen:", err)
		return 1
	}
	fmt.Fprint(stdout, text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprintln(stdout)
	}
	return 0
}

// genRoot is the workspace root of file abs, by the rule of tenniarb check: -root, else the working
// directory when the file lies inside it, else the directory of the file.
func genRoot(flagRoot, cwd, abs string) string {
	if flagRoot != "" {
		if r, err := filepath.Abs(flagRoot); err == nil {
			return r
		}
	}
	if rel, err := filepath.Rel(cwd, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return cwd
	}
	return filepath.Dir(abs)
}
