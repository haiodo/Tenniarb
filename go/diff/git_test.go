package diff_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/diff"
)

// repo makes a repository with one committed file and returns its folder.
func repo(t *testing.T, name, text string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", name)
	run("commit", "-m", "the first version")
	return dir
}

func TestGitRepoAndShow(t *testing.T) {
	const text = "element \"Card\" {\n  width 240\n}\n"
	dir := repo(t, "Card.tenn", text)

	root, ok := diff.Repo(dir)
	if !ok {
		t.Fatalf("Repo(%q) found nothing", dir)
	}
	got, err := diff.Show(root, diff.RevHead, "Card.tenn")
	if err != nil {
		t.Fatal(err)
	}
	if got != text {
		t.Fatalf("git show gives %q, want %q", got, text)
	}
	// the working copy is the new text; the comparison is against the commit
	edit := strings.Replace(text, "240", "200", 1)
	if err := os.WriteFile(filepath.Join(dir, "Card.tenn"), []byte(edit), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := diff.Compare(root, "Card.tenn", got, edit)
	if err != nil {
		t.Fatal(err)
	}
	if p := rep.Node("Card").Props; len(p) != 1 || p[0].Old != "240" || p[0].New != "200" {
		t.Fatalf("props %+v", p)
	}
}

func TestRepoOutside(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	if _, ok := diff.Repo(t.TempDir()); ok {
		t.Fatal("a folder that is no repository is reported as one")
	}
	if _, err := diff.Show(t.TempDir(), diff.RevHead, "x.tenn"); err == nil {
		t.Fatal("Show outside a repository gives no error")
	}
}

func TestCompareAddDoesNotMoveOthers(t *testing.T) {
	oldText := `element "A" {
  item One { title "1" }
  item Two { title "2" }
}
`
	newText := `element "A" {
  item Zero { title "0" }
  item One { title "1" }
  item Two { title "2" }
}
`
	r := compare(t, oldText, newText)
	got := statuses(r)
	if got["A.Zero"] != diff.Added || len(got) != 1 {
		t.Fatalf("adding a node moved the others: %v", got)
	}
}

func TestRevertTopLevelElement(t *testing.T) {
	oldText := `element "A" {
}
element "B" {
}
`
	newText := `element "A" {
}
`
	r := compare(t, oldText, newText)
	e, ok := r.Revert("B")
	if !ok {
		t.Fatal("no revert of a removed top-level element")
	}
	if got := apply(newText, e); got != oldText {
		t.Fatalf("revert gives %q, want %q", got, oldText)
	}
}

func TestCompareAddedElementHoldsItsNodes(t *testing.T) {
	oldText := `element "A" {
}
`
	newText := `element "A" {
}
element "B" {
  item X { title "x" }
}
`
	r := compare(t, oldText, newText)
	got := statuses(r)
	if got["B"] != diff.Added || got["B.X"] != diff.Added {
		t.Fatalf("the nodes of an added element are not listed as added: %v", got)
	}
	r = compare(t, newText, oldText)
	got = statuses(r)
	if got["B"] != diff.Removed || got["B.X"] != diff.Removed {
		t.Fatalf("the nodes of a removed element are not listed as removed: %v", got)
	}
}
