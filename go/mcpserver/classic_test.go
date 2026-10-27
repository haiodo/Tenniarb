package mcpserver

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const brainMap = `element "Map" {
    styles {
        item {
            shadow -5 -5 5
        }
    }
    item "Idea" {
        pos 0 0
    }
    item "Next step" {
        pos 160 40
        color red
    }
    link "Idea" "Next step" {
        display arrow
    }
}
element "Screen" {
    item "Card" {
        auto-layout column
        item "Title" { title "Hi" }
    }
}
element "Flat" {
    item "Row" {
        auto-layout row
        width 40
        height 20
        color green
    }
}
`

func memSession(t *testing.T, root string) *mcp.ClientSession {
	t.Helper()
	srv, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.MCP().Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestClassicDiagram(t *testing.T) {
	root := tempRoot(t, map[string]string{"map.tenn": brainMap})
	cs := memSession(t, root)
	state := mustOK(t, cs, "get_state", nil)
	for _, want := range []string{"  Map element 1 diagram\n", "    Map.Next_step item 10\n", "  Screen element 18 scene\n", "  Flat element 24 scene\n"} {
		if !strings.Contains(state, want) {
			t.Errorf("state lacks %s:\n%s", want, state)
		}
	}
	if got := mustOK(t, cs, "read", map[string]any{"file": "map.tenn", "path": "Map.Next_step"}); !strings.Contains(got, "color red") {
		t.Errorf("read node: %s", got)
	}
	mustOK(t, cs, "edit", withFile(edits("color red", "color blue"), "map.tenn"))
	if got := mustOK(t, cs, "validate", map[string]any{"file": "map.tenn"}); !strings.Contains(got, "0 errors") {
		t.Errorf("validate: %s", got)
	}
	mustFail(t, cs, "edit", withFile(edits(`link "Idea" "Next step"`, `link "Idea" "Nobody"`), "map.tenn"), "adds errors")

	if runtime.GOOS != "darwin" {
		t.Skip("render checked on macOS only")
	}
	res, text := call(t, cs, "render", map[string]any{"file": "map.tenn", "element": "Map", "scale": 2})
	if res.IsError || !strings.Contains(text, "as a diagram") {
		t.Fatalf("render Map: %s", text)
	}
	img := res.Content[0].(*mcp.ImageContent)
	cfg, err := png.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil || cfg.Width < 200 || cfg.Height < 50 {
		t.Fatalf("diagram PNG: %v %+v", err, cfg)
	}
	if _, text := call(t, cs, "render", map[string]any{"file": "map.tenn", "element": "Screen"}); !strings.Contains(text, "as a scene") {
		t.Errorf("render Screen: %s", text)
	}
	if _, text := call(t, cs, "render", map[string]any{"file": "map.tenn", "element": "Flat"}); !strings.Contains(text, "as a scene") {
		t.Errorf("auto-layout without children must draw as a scene: %s", text)
	}
	mustFail(t, cs, "render", map[string]any{"file": "map.tenn", "element": "Map.Idea"}, "not an element")
}

// TestCacheSeesExternalWrites: a tree is reused only while no file it read has changed.
func TestCacheSeesExternalWrites(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"main.tenn": "import \"lib.tenn\" as lib\nelement \"S\" {\n    use lib.K.B as b\n}\n",
		"lib.tenn":  "element \"K\" {\n    item \"B\" { title \"b\" }\n}\n",
	})
	cs := memSession(t, root)
	if got := mustOK(t, cs, "validate", map[string]any{"file": "main.tenn"}); !strings.Contains(got, "0 errors") {
		t.Fatalf("validate: %s", got)
	}
	// Same length, other name: only the modification time tells.
	if err := os.WriteFile(filepath.Join(root, "lib.tenn"), []byte("element \"K\" {\n    item \"C\" { title \"b\" }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mustOK(t, cs, "validate", map[string]any{"file": "main.tenn"}); strings.Contains(got, "0 errors") {
		t.Errorf("a change of an imported file was not seen: %s", got)
	}
}

// TestRendererRule: renderers follow model.SceneElement, the rule the app draws by. go/scene reads
// slot only inside an item, so a slot written directly in an element leaves it a diagram.
func TestRendererRule(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"r.tenn":    "element \"Slotted\" {\n    slot \"x\"\n    item \"A\" { pos 0 0 }\n}\nelement \"Comp\" {\n    item \"W\" {\n        slot \"x\"\n    }\n}\n",
		"root.tenn": "item \"A\" { title x }\n",
	})
	cs := memSession(t, root)
	state := mustOK(t, cs, "get_state", nil)
	for _, want := range []string{"\n  Slotted element 1 diagram\n", "\n  Comp element 5 scene\n", "\nroot.tenn diagram: "} {
		if !strings.Contains(state, want) {
			t.Errorf("state lacks %q:\n%s", want, state)
		}
	}
	if runtime.GOOS != "darwin" {
		t.Skip("render checked on macOS only")
	}
	if _, text := call(t, cs, "render", map[string]any{"file": "r.tenn", "element": "Slotted"}); !strings.Contains(text, "as a diagram") {
		t.Errorf("render Slotted: %s", text)
	}
	if _, text := call(t, cs, "render", map[string]any{"file": "r.tenn", "element": "Comp"}); !strings.Contains(text, "as a scene") {
		t.Errorf("render Comp: %s", text)
	}
}
