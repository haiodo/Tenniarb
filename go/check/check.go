// Package check is the tenniarb check command: it validates .tenn files with the same scene.Diagnose
// the MCP validate tool uses, headless, for agents that edit files without MCP.
package check

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
)

// Exit codes of Run.
const (
	ExitOK     = 0
	ExitErrors = 1
	ExitUsage  = 2
)

const usage = "usage: tenniarb check [-json] [-root DIR] FILE..."

// Diag is one diagnostic in the -json output; File is relative to the working directory.
type Diag struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Run checks the files named in args and returns the exit code: 0 clean or warnings only, 1 errors,
// 2 usage or a file that could not be read.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print the diagnostics as a JSON array")
	root := fs.String("root", "", "workspace root for --from-file (default: the working directory, or the directory of a file outside it)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, usage)
		return ExitUsage
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "tenniarb check:", err)
		return ExitUsage
	}
	diags := []Diag{}
	seen := map[scene.Error]bool{}
	var nerr, nwarn int
	failed := false
	for _, f := range fs.Args() {
		abs, err := filepath.Abs(f)
		if err != nil {
			fmt.Fprintln(stderr, "tenniarb check:", err)
			failed = true
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			fmt.Fprintln(stderr, "tenniarb check:", err)
			failed = true
			continue
		}
		r := rootOf(*root, cwd, abs)
		es, _, err := scene.Diagnose(r, abs, nil)
		if err != nil {
			fmt.Fprintln(stderr, "tenniarb check:", err)
			failed = true
			continue
		}
		for _, e := range es {
			if seen[e] {
				continue
			}
			seen[e] = true
			d := diagOf(e, r, cwd)
			if e.Warn {
				nwarn++
			} else {
				nerr++
			}
			diags = append(diags, d)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(diags)
	} else {
		for _, d := range diags {
			fmt.Fprintf(stdout, "%s:%d:%d: %s: %s\n", d.File, d.Line, d.Col, d.Severity, d.Message)
		}
		fmt.Fprintf(stdout, "%d files checked, %d errors, %d warnings\n", fs.NArg(), nerr, nwarn)
	}
	switch {
	case failed:
		return ExitUsage
	case nerr > 0:
		return ExitErrors
	}
	return ExitOK
}

// rootOf is the workspace root of file abs: -root, else the working directory as tenniarb-mcp
// does, else the directory of the file when it lies outside the working directory.
func rootOf(flagRoot, cwd, abs string) string {
	if flagRoot != "" {
		r, err := filepath.Abs(flagRoot)
		if err == nil {
			return r
		}
	}
	if rel, err := filepath.Rel(cwd, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return cwd
	}
	return filepath.Dir(abs)
}

func diagOf(e scene.Error, root, cwd string) Diag {
	file := filepath.Join(root, filepath.FromSlash(e.Pos.File))
	if rel, err := filepath.Rel(cwd, file); err == nil {
		file = rel
	}
	sev := "error"
	if e.Warn {
		sev = "warning"
	}
	return Diag{File: filepath.ToSlash(file), Line: e.Pos.Line, Col: e.Pos.Col, Severity: sev, Message: e.Msg}
}
