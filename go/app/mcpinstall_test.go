//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMCPRegistrations pins the command line every client CLI gets: the proxy last, after "--", named
// tenniarb.
func TestMCPRegistrations(t *testing.T) {
	proxy := "/opt/Tenniarb.app/Contents/MacOS/tenniarb-mcp"
	regs := mcpRegistrations(proxy)
	if len(regs) != 2 || regs[0].client != "opencode" || regs[1].client != "claude" {
		t.Fatalf("clients: %+v", regs)
	}
	for _, r := range regs {
		if n := len(r.args); n < 2 || r.args[n-1] != proxy || r.args[n-2] != "--" {
			t.Fatalf("%s: args %v, want the proxy after --", r.client, r.args)
		}
		if i := indexOf(r.args, "tenniarb"); i < 0 {
			t.Fatalf("%s: args %v, no server name", r.client, r.args)
		}
	}
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// TestFindClientOutsidePath covers the Finder case: the app starts without the shell PATH, and the
// client CLIs live in the home directories installers use.
func TestFindClientOutsidePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	name := "tenniarb-test-client"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, name)
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := lookPathWithDirs(name)
	if err != nil {
		t.Fatalf("lookPathWithDirs: %v", err)
	}
	if got != want {
		t.Fatalf("found %q, want %q", got, want)
	}
	if _, err := lookPathWithDirs("tenniarb-not-there"); err == nil {
		t.Fatal("a missing client was found")
	}
}
