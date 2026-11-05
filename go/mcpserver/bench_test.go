package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// callStat is one line of TENNIARB_MCP_LOG.
type callStat struct {
	Tool string `json:"tool"`
	In   int    `json:"in"`
	Out  int    `json:"out"`
	MS   int64  `json:"ms"`
}

// TestBenchWorkflows measures two ways of the same job by the bytes the tools return, which is what costs
// tokens: whole files and pictures, and the node tools. Skipped unless BENCHDST names a workspace with
// PlatformDesign.tenn.
func TestBenchWorkflows(t *testing.T) {
	src := os.Getenv("BENCHDST")
	if src == "" {
		t.Skip("BENCHDST not set: a workspace with PlatformDesign.tenn")
	}
	root := t.TempDir()
	if err := benchCopy(src, root); err != nil {
		t.Fatalf("copy: %v", err)
	}
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("TENNIARB_MCP_LOG", log)
	cs := memSession(t, root)

	measure := func(name string, run func()) {
		_ = os.Remove(log)
		run()
		b, _ := os.ReadFile(log)
		var in, out int
		var ms int64
		calls, failed := 0, 0
		names := map[string]int{}
		for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if ln == "" {
				continue
			}
			var c callStat
			if json.Unmarshal([]byte(ln), &c) != nil {
				continue
			}
			calls++
			in, out, ms = in+c.In, out+c.Out, ms+c.MS
			names[c.Tool]++
		}
		_ = failed
		t.Logf("%-6s calls=%d in=%dB out=%dB (~%d tokens) ms=%d %v", name, calls, in, out, out/4, ms, names)
	}

	const actions = "dev/design/tenn/lib/actions.tenn"
	const forms = "dev/design/tenn/lib/forms.tenn"

	measure("state", func() {
		call(t, cs, "get_state", map[string]any{})
	})

	measure("files", func() {
		for _, f := range []string{actions, forms} {
			call(t, cs, "read", map[string]any{"file": f})
		}
		for _, el := range []string{"Libraries.ui_kit.Actions", "Libraries.ui_kit.Forms"} {
			call(t, cs, "render", map[string]any{"file": "PlatformDesign.tenn", "element": el, "scale": 1})
		}
	})

	measure("nodes", func() {
		for _, el := range []string{"Libraries.ui_kit.Actions", "Libraries.ui_kit.Forms"} {
			call(t, cs, "layout", map[string]any{"file": "PlatformDesign.tenn", "element": el})
		}
		call(t, cs, "design", map[string]any{"file": actions, "ops": []map[string]any{
			{"op": "set", "node": "__ButtonCatalog", "name": "height", "value": "492"},
		}})
		call(t, cs, "layout", map[string]any{"file": "PlatformDesign.tenn", "element": "Libraries.ui_kit.Actions"})
	})
}

// benchCopy copies a workspace for the benchmark, so a run never touches the real files.
func benchCopy(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil // the design needs no links
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}
