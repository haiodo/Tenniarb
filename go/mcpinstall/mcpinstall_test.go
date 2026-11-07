package mcpinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCommand: every agent gets the server under the same name, its log and the profiler address, and
// claude is cleared of an old entry first.
func TestCommand(t *testing.T) {
	const bin, log, pprof = "/go/bin/tenniarb-mcp", "/tmp/calls.log", "127.0.0.1:6061"
	want := map[Agent][]string{
		Opencode: {"opencode", "mcp", "add", "--global", "tenniarb",
			"--env", "TENNIARB_MCP_LOG=" + log, "--env", "TENNIARB_PPROF=" + pprof, "--", bin},
		Codex: {"codex", "mcp", "add", "tenniarb",
			"--env", "TENNIARB_MCP_LOG=" + log, "--env", "TENNIARB_PPROF=" + pprof, "--", bin},
		Claude: {"claude", "mcp", "add", "tenniarb", "-s", "user",
			"-e", "TENNIARB_MCP_LOG=" + log, "-e", "TENNIARB_PPROF=" + pprof, "--", bin},
	}
	for agent, args := range want {
		calls, err := Command(agent, bin, log, pprof)
		if err != nil {
			t.Fatalf("%s: %v", agent, err)
		}
		got := calls[len(calls)-1]
		if strings.Join(got, " ") != strings.Join(args, " ") {
			t.Errorf("%s: %q, want %q", agent, got, args)
		}
	}
	if calls, _ := Command(Claude, bin, log, pprof); len(calls) != 2 || strings.Join(calls[0], " ") != "claude mcp remove tenniarb -s user" {
		t.Errorf("claude does not clear the old entry first: %q", calls)
	}
	if calls, _ := Command(Codex, bin, log, "off"); strings.Contains(strings.Join(calls[0], " "), "PPROF") {
		t.Errorf("pprof off is still passed: %q", calls[0])
	}
	if _, err := Command("nope", bin, log, pprof); err == nil {
		t.Error("no error for an unknown agent")
	}
}

// fakeCLI puts a script named name on PATH that appends its arguments to out.
func fakeCLI(t *testing.T, name, out string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	dir := filepath.Dir(out)
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestRegister: the agent CLI is called with the arguments Command builds, and an agent without a CLI is
// Missing rather than a failure.
func TestRegister(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "calls")
	fakeCLI(t, "codex", out)
	bin, log := filepath.Join(dir, "tenniarb-mcp"), filepath.Join(dir, "calls.log")
	if err := Register(Codex, bin, log, "127.0.0.1:6061"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(b))
	for _, want := range []string{"mcp add tenniarb", "TENNIARB_MCP_LOG=" + log, "TENNIARB_PPROF=127.0.0.1:6061", bin} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q lacks %q", got, want)
		}
	}

	t.Setenv("PATH", dir)
	if err := Register(Claude, bin, log, ""); err == nil {
		t.Fatal("no error without a claude CLI")
	} else if _, ok := err.(Missing); !ok {
		t.Errorf("an absent CLI is %T, want Missing", err)
	}
}
