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
	targets := skill.Targets()
	if *dir != "" {
		targets = []string{*dir}
	}
	if len(targets) == 0 {
		fmt.Fprintln(stderr, "tenniarb install: no home directory to install the skill into")
		return 1
	}
	code := 0
	for _, t := range targets {
		if err := skill.Install(t); err != nil {
			fmt.Fprintf(stderr, "tenniarb install: %s: %v\n", t, err)
			code = 1
			continue
		}
		fmt.Fprintf(stdout, "installed %s to %s\n", strings.Join(skill.Names(), ", "), t)
	}
	return code
}
