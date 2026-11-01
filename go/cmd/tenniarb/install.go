package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/haiodo/tenniarb/go/skill"
)

const installUsage = "usage: tenniarb install [-dir DIR]..."

// install writes the design skill where the agents read it: every directory of skill.Targets, or the one
// DIR given. It returns the exit code of the command: 0 done, 1 a directory could not be written,
// 2 usage.
func install(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "install only into DIR, instead of the directories of the agents")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir != "" {
		if err := skill.Write(*dir); err != nil {
			fmt.Fprintf(stderr, "tenniarb install: %s: %v\n", *dir, err)
			return 1
		}
		fmt.Fprintf(stdout, "installed %s to %s\n", strings.Join(skill.Names(), ", "), *dir)
		return 0
	}
	done, errs := skill.Install()
	if len(done) == 0 && len(errs) == 0 {
		fmt.Fprintln(stderr, "tenniarb install: no home directory to install the skill into")
		return 1
	}
	for _, t := range done {
		fmt.Fprintf(stdout, "installed %s to %s\n", strings.Join(skill.Names(), ", "), t)
	}
	for _, err := range errs {
		fmt.Fprintf(stderr, "tenniarb install: %v\n", err)
	}
	if len(errs) > 0 {
		return 1
	}
	return 0
}
