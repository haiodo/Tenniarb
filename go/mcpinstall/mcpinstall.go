// Package mcpinstall registers the tenniarb MCP server in the config of an agent by running the CLI of
// that agent: every one of them owns its format and its scope, and a config written by hand goes stale.
package mcpinstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Agent is one agent whose config can name the tenniarb MCP server.
type Agent string

const (
	Codex    Agent = "codex"
	Claude   Agent = "claude"
	Opencode Agent = "opencode"
)

// All is every agent Register knows, in the order the installer reports them.
func All() []Agent { return []Agent{Opencode, Codex, Claude} }

// ParseAgent reads an agent name as the install flags write it.
func ParseAgent(s string) (Agent, error) {
	for _, a := range All() {
		if string(a) == s {
			return a, nil
		}
	}
	return "", fmt.Errorf("unknown agent %q: codex, claude or opencode", s)
}

// Command is the CLI calls that make agent a know the server bin with env: a server under the same name
// is replaced. claude needs two calls, the rest one.
func Command(a Agent, bin, log, pprof string) ([][]string, error) {
	vars := [][2]string{{"TENNIARB_MCP_LOG", log}}
	if pprof != "" && pprof != "off" {
		vars = append(vars, [2]string{"TENNIARB_PPROF", pprof})
	}
	switch a {
	case Opencode:
		return [][]string{append([]string{"opencode", "mcp", "add", "--global", "tenniarb"}, append(flagPairs("--env", vars), "--", bin)...)}, nil
	case Codex:
		return [][]string{append([]string{"codex", "mcp", "add", "tenniarb"}, append(flagPairs("--env", vars), "--", bin)...)}, nil
	case Claude:
		// Claude Code refuses a name it already has, so the old entry goes first.
		return [][]string{
			{"claude", "mcp", "remove", "tenniarb", "-s", "user"},
			append([]string{"claude", "mcp", "add", "tenniarb", "-s", "user"}, append(flagPairs("-e", vars), "--", bin)...),
		}, nil
	}
	return nil, fmt.Errorf("unknown agent %q", a)
}

// flagPairs is flag KEY=VALUE for every variable, in the order given.
func flagPairs(flag string, vars [][2]string) []string {
	out := make([]string, 0, len(vars)*2)
	for _, v := range vars {
		out = append(out, flag, v[0]+"="+v[1])
	}
	return out
}

// Missing is the error of an agent whose CLI is not on PATH: the skill is still worth writing, so the
// caller reports it and goes on.
type Missing struct{ Agent Agent }

func (m Missing) Error() string {
	return fmt.Sprintf("%s is not on PATH: its MCP server is not registered", m.Agent)
}

// Register runs the calls of Command. A missing CLI is Missing; the first other failure stops the agent.
// The remove of an old claude entry is allowed to fail: there may be nothing to remove.
func Register(a Agent, bin, log, pprof string) error {
	calls, err := Command(a, bin, log, pprof)
	if err != nil {
		return err
	}
	name := calls[0][0]
	if _, err := exec.LookPath(name); err != nil {
		return Missing{Agent: a}
	}
	for i, call := range calls {
		cmd := exec.Command(call[0], call[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if i == 0 && len(calls) > 1 {
				continue // nothing to remove
			}
			return fmt.Errorf("%s: %v: %s", name, err, firstLine(string(out)))
		}
	}
	return nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// Bin is the path of the MCP proxy: tenniarb-mcp next to the running tenniarb, so a GOBIN that holds
// both by install writes the same pair the agent will run.
func Bin() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	name := "tenniarb-mcp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(filepath.Dir(self), name)
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("%s: %v (install it with the CLI: make go-mcp-install)", bin, err)
	}
	return bin, nil
}

// DefaultLog is where the calls of the agents are recorded: the log a developer looks for, not one under
// the temporary directory.
func DefaultLog() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Logs", "Tenniarb", "mcp-calls.log")
	}
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "tenniarb", "mcp-calls.log")
	}
	return filepath.Join(home, ".local", "state", "tenniarb", "mcp-calls.log")
}

// DefaultPprof is the loopback address of the service profiler, off unless asked for by address.
const DefaultPprof = "127.0.0.1:6061"
