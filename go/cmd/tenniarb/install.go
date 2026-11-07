package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/mcpinstall"
	"github.com/haiodo/tenniarb/go/skill"
)

const installUsage = `usage: tenniarb install [-dir DIR] [-mcp] [-codex|-claude|-opencode|-agents]
                         [-log PATH] [-pprof ADDR] [-bin PATH]`

// install writes the design skill where the agents read it - every directory of skill.Places, or the one
// DIR given - and, with -mcp, registers the tenniarb MCP server in the config of the selected agents. It
// returns the exit code of the command: 0 done, 1 something could not be written or registered, 2 usage.
func install(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "install only into DIR, instead of the directories of the agents")
	mcp := fs.Bool("mcp", false, "also register the tenniarb MCP server in the agent configs")
	codex := fs.Bool("codex", false, "codex only")
	claude := fs.Bool("claude", false, "Claude Code only")
	opencode := fs.Bool("opencode", false, "opencode only")
	agents := fs.Bool("agents", false, "every agent (the default)")
	logPath := fs.String("log", mcpinstall.DefaultLog(), "where the MCP proxy records the calls of the agents")
	pprof := fs.String("pprof", mcpinstall.DefaultPprof, "loopback address of the service profiler (\"off\" for none)")
	bin := fs.String("bin", "", "path of tenniarb-mcp; the default is the one next to this binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// An old -dir install is a skill copy somewhere else: no agent is selected for it.
	if *dir != "" && *mcp {
		fmt.Fprintln(stderr, installUsage)
		fmt.Fprintln(stderr, "tenniarb install: -dir takes no -mcp: the agent configs would name a server, not a directory")
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

	selected := selectAgents(*codex, *claude, *opencode, *agents)
	done, errs := skill.InstallFor(selected)
	if len(done) == 0 && len(errs) == 0 {
		fmt.Fprintln(stderr, "tenniarb install: no home directory to install the skill into")
		return 1
	}
	for _, d := range done {
		fmt.Fprintf(stdout, "installed %s to %s\n", strings.Join(skill.Names(), ", "), d)
	}
	code := 0
	for _, err := range errs {
		fmt.Fprintf(stderr, "tenniarb install: %v\n", err)
		code = 1
	}
	if !*mcp {
		return code
	}
	binPath, err := mcpBin(*bin)
	if err != nil {
		fmt.Fprintf(stderr, "tenniarb install: %v\n", err)
		return 1
	}
	if lp := *logPath; lp != "" && lp != "off" {
		if err := os.MkdirAll(filepath.Dir(lp), 0o755); err != nil {
			fmt.Fprintf(stderr, "tenniarb install: %v\n", err)
			return 1
		}
	}
	for _, name := range selected {
		agent, err := mcpinstall.ParseAgent(name)
		if err != nil {
			fmt.Fprintf(stderr, "tenniarb install: %v\n", err)
			code = 1
			continue
		}
		if err := mcpinstall.Register(agent, binPath, *logPath, *pprof); err != nil {
			if _, missing := err.(mcpinstall.Missing); missing {
				fmt.Fprintf(stdout, "%v\n", err) // the skill is installed; a missing CLI is a note, not a failure
				continue
			}
			fmt.Fprintf(stderr, "tenniarb install: %v\n", err)
			code = 1
			continue
		}
		fmt.Fprintf(stdout, "registered %s in %s\n", binPath, agent)
	}
	return code
}

// selectAgents is the agents the flags name: the ones given, or every one of them.
func selectAgents(codex, claude, opencode, agents bool) []string {
	var out []string
	if opencode {
		out = append(out, string(mcpinstall.Opencode))
	}
	if codex {
		out = append(out, string(mcpinstall.Codex))
	}
	if claude {
		out = append(out, string(mcpinstall.Claude))
	}
	if len(out) == 0 || agents {
		out = out[:0]
		for _, a := range mcpinstall.All() {
			out = append(out, string(a))
		}
	}
	return out
}

// mcpBin is the proxy the agent will run: the path given, or tenniarb-mcp next to this binary.
func mcpBin(given string) (string, error) {
	if given != "" {
		if _, err := os.Stat(given); err != nil {
			return "", err
		}
		return given, nil
	}
	return mcpinstall.Bin()
}
