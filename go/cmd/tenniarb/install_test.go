package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstall: the skill files land in the directory given, and the output says where.
func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tenniarb-design")
	var out, errOut bytes.Buffer
	if code := install([]string{"-dir", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md: %v", err)
	}
	if !strings.Contains(out.String(), dir) {
		t.Errorf("output does not name the directory: %q", out.String())
	}
}
