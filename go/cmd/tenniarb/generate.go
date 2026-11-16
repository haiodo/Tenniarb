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

const generateUsage = "usage: tenniarb generate [-root DIR] [-set name=value]... [-template TEXT|@FILE] FILE ELEMENT"

// generate prints the code an element records in its templates (spec 5c): the template of every use below
// it, each with the values of the use and the content of its slots, and the element's own template or the
// one given around them. A plain item prints nothing, so only the components of the tree make the code.
// ELEMENT is a path of names: an element or a use of a component. Exit 0 done, 1 errors in the file or a
// template, 2 usage or an unreadable file.
func generate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("root", "", "workspace root for --from-file (default: the working directory, or the directory of a file outside it)")
	template := fs.String("template", "", "code around the uses, its <slot/> takes their code; @FILE reads it from the file (default: the template of ELEMENT)")
	values := setFlag{}
	fs.Var(values, "set", "value of a parameter: name=value, repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, generateUsage)
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		return 2
	}
	abs, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		return 2
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		return 2
	}
	// The entry context matters: a screen names the components of a library it mounts, and only the tree
	// of the entry knows them.
	tree, err := scene.LoadAuto(genRoot(*rootFlag, cwd, abs), abs, nil)
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		if tree == nil {
			return 2
		}
		return 1
	}
	node, err := tree.Resolve(tree.File(tree.Files()[0]), fs.Arg(1))
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		return 1
	}
	tpl, err := templateOf(*template, node.Pos)
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		return 2
	}
	var text string
	o := scene.Options{EvalVars: exec.SceneEvalVars}
	if node.Kind == scene.KUse {
		text, err = tree.Render(node, values, o)
	} else {
		text, err = tree.RenderElement(node, tpl, values, o)
	}
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb generate:", err)
		return 1
	}
	fmt.Fprint(stdout, text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprintln(stdout)
	}
	return 0
}

// templateOf reads the template of -template: the text itself, or the whole file after @.
func templateOf(flag string, pos scene.Pos) (*scene.Template, error) {
	if flag == "" {
		return nil, nil
	}
	if path, ok := strings.CutPrefix(flag, "@"); ok {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return &scene.Template{Text: string(b), Pos: pos}, nil
	}
	return &scene.Template{Text: flag, Pos: pos}, nil
}
