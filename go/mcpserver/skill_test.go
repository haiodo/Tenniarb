package mcpserver

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

var (
	skillFileLine = regexp.MustCompile(`^// file: (\S+)\n`)
	skillRef      = regexp.MustCompile("`([a-z]+\\.md)`")
)

// TestSkillExamples loads and renders every tenn block of the skill, so the skill cannot drift from
// the format. A block whose first line is "// file: <path>" is written there for others to import.
func TestSkillExamples(t *testing.T) {
	for _, name := range skillFiles() {
		text, err := readSkill(name)
		if err != nil {
			t.Fatal(err)
		}
		blocks := tennBlock.FindAllStringSubmatch(text, -1)
		if len(blocks) == 0 {
			continue
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var written []string
			for i, b := range blocks {
				rel := fmt.Sprintf("%s-%d.tenn", strings.TrimSuffix(name, ".md"), i+1)
				if m := skillFileLine.FindStringSubmatch(b[1]); m != nil {
					rel = m[1]
				}
				p := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(b[1]), 0o644); err != nil {
					t.Fatal(err)
				}
				written = append(written, rel)
			}
			srv, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, rel := range written {
				checkSkillFile(t, srv, rel)
			}
		})
	}
}

// checkSkillFile fails on any problem of rel and renders each element of it that has content.
func checkSkillFile(t *testing.T, srv *Server, rel string) {
	t.Helper()
	es, tree, err := srv.diagnose(rel)
	if err != nil || tree == nil {
		t.Fatalf("%s does not load: %v %v", rel, err, es)
	}
	if len(es) > 0 {
		t.Errorf("%s:\n%s", rel, summary(es, false, ""))
		return
	}
	if runtime.GOOS != "darwin" {
		return
	}
	for _, el := range skillElements(tree.File(rel), "") {
		r, err := srv.render(rel, el, 1, false)
		if err != nil {
			t.Errorf("render %s %q: %v", rel, el, err)
			continue
		}
		if _, err := png.Decode(bytes.NewReader(r.png)); err != nil {
			t.Errorf("render %s %q: %v", rel, el, err)
		}
		if !strings.HasSuffix(r.text, "\n0 warnings") {
			t.Errorf("render %s %q: %s", rel, el, r.text)
		}
		if dir := os.Getenv("SKILL_PNG_DIR"); dir != "" {
			out := strings.NewReplacer("/", "_", " ", "_").Replace(rel + "-" + el + ".png")
			os.WriteFile(filepath.Join(dir, out), r.png, 0o644)
		}
	}
}

// skillElements lists the dotted paths of the elements under n that have content.
func skillElements(n *scene.Node, prefix string) []string {
	var out []string
	for _, e := range n.Elems {
		p := prefix + e.Key
		if hasContent(e) {
			out = append(out, p)
		}
		out = append(out, skillElements(e, p+".")...)
	}
	return out
}

func TestReadSkill(t *testing.T) {
	cs, _ := workspace(t)
	main := mustOK(t, cs, "read_skill", nil)
	if !strings.HasPrefix(main, "---\nname: tenniarb-design") {
		t.Fatalf("read_skill without path: %q", main[:min(len(main), 80)])
	}
	for _, m := range skillRef.FindAllStringSubmatch(main, -1) {
		if got := mustOK(t, cs, "read_skill", map[string]any{"path": m[1]}); !strings.HasPrefix(got, "# ") {
			t.Errorf("%s: %q", m[1], got[:min(len(got), 80)])
		}
	}
	mustFail(t, cs, "read_skill", map[string]any{"path": "../server.go"}, "files: SKILL.md")
	mustFail(t, cs, "read_skill", map[string]any{"path": "nothing.md"}, "layout.md")
	if g := mustOK(t, cs, "guide", map[string]any{"topic": "examples"}); !strings.Contains(g, "read_skill") {
		t.Error("guide does not point at read_skill")
	}
}
