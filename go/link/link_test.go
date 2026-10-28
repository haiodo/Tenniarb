package link

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeRemote(t *testing.T) {
	want := "github.com/haiodo/foundation"
	for _, raw := range []string{
		"git@github.com:haiodo/foundation.git",
		"https://github.com/haiodo/foundation.git",
		"https://github.com/haiodo/foundation",
		"https://github.com/haiodo/foundation/",
		"https://user:token@github.com/haiodo/foundation.git",
		"ssh://git@github.com:22/haiodo/foundation.git",
		"git://github.com/haiodo/foundation.git\n",
		"GitHub.com:Haiodo/Foundation",
	} {
		if got := NormalizeRemote(raw); got != want {
			t.Errorf("%q: %q, want %q", raw, got, want)
		}
	}
	for _, raw := range []string{"", "foundation", "/srv/git/foundation.git", "https://github.com"} {
		if got := NormalizeRemote(raw); got != "" {
			t.Errorf("%q: %q, want empty", raw, got)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, l := range []Link{
		{Repo: "github.com/haiodo/foundation", Path: "design/PlatformDesign.tenn", Element: []string{"Tracker", "Boards"}, Node: "Kanban"},
		{Repo: "github.com/a/b", Path: "File.tenn"},
		{Repo: "github.com/a/b", Path: "File.tenn", Node: "x"},
		{Repo: "github.com/a/b", Path: "My docs/Дизайн.tenn", Element: []string{"Трекер досок", "Boards#1"}, Node: "Card#2"},
		{Repo: "github.com/a/b", Path: "a.tenn", Element: []string{"Screen", "v1.2 a/b 100%"}, Node: "login/Header.Title"},
		{Repo: "github.com/a/b", Path: "a.tenn", Element: []string{"A&B=C?"}, Node: "x y&z"},
		{Home: true, Path: "notes/a.tenn", Element: []string{"E"}},
	} {
		got, err := Parse(l.URL())
		if err != nil {
			t.Errorf("%s: %v", l.URL(), err)
			continue
		}
		if got.Repo != l.Repo || got.Home != l.Home || got.Path != l.Path || got.Node != l.Node || !slices.Equal(got.Element, l.Element) {
			t.Errorf("url %s: got %+v, want %+v", l.URL(), got, l)
		}
		plain, err := Parse(l.Text())
		if err != nil {
			t.Errorf("%s: %v", l.Text(), err)
			continue
		}
		if plain.Path != l.Path || plain.Node != l.Node || !slices.Equal(plain.Element, l.Element) || plain.Repo != "" {
			t.Errorf("plain %s: got %+v, want %+v", l.Text(), plain, l)
		}
	}
}

func TestURLShape(t *testing.T) {
	l := Link{Repo: "github.com/haiodo/foundation", Path: "a/B c.tenn", Element: []string{"Tracker", "Boards"}, Node: "Kanban"}
	want := "tenniarb://open?repo=github.com/haiodo/foundation&path=a/B%20c.tenn&el=Tracker.Boards&node=Kanban"
	if l.URL() != want {
		t.Fatalf("%s", l.URL())
	}
	if got := l.Text(); got != "a/B c.tenn#Tracker.Boards/Kanban" {
		t.Fatalf("%s", got)
	}
}

func TestParsePlain(t *testing.T) {
	l, err := Parse("  PlatformDesign.tenn#Tracker.Boards/Kanban\n")
	if err != nil || l.Path != "PlatformDesign.tenn" || !slices.Equal(l.Element, []string{"Tracker", "Boards"}) || l.Node != "Kanban" {
		t.Fatalf("%+v %v", l, err)
	}
	// The first "/" ends the element; the rest is the node, an instance path included.
	l, err = Parse("d/F.tenn#Screen/login/Header.Title")
	if err != nil || l.Node != "login/Header.Title" || !slices.Equal(l.Element, []string{"Screen"}) {
		t.Fatalf("%+v %v", l, err)
	}
	l, err = Parse("F.tenn#/Kanban")
	if err != nil || len(l.Element) != 0 || l.Node != "Kanban" {
		t.Fatalf("%+v %v", l, err)
	}
	l, err = Parse("F.tenn")
	if err != nil || l.Path != "F.tenn" || len(l.Element) != 0 {
		t.Fatalf("%+v %v", l, err)
	}
}

func TestParseRefuses(t *testing.T) {
	for _, s := range []string{
		"", "hello", "notes.txt#a", "foo#1",
		"tenniarb://open?path=../x.tenn",
		"tenniarb://open?path=/etc/x.tenn",
		"tenniarb://open?path=a/../../x.tenn",
		"tenniarb://open?path=x.txt",
		"tenniarb://open?repo=a/b",
		"tenniarb://close?path=x.tenn",
		"../x.tenn#a",
		"/abs/x.tenn#a",
		"x.tenn#a..b",
	} {
		if l, err := Parse(s); err == nil {
			t.Errorf("%q parsed as %+v", s, l)
		}
	}
}

func TestResolve(t *testing.T) {
	files := map[string]bool{
		"/work/foundation/design/P.tenn": true,
		"/work/other/design/P.tenn":      true,
		"/home/me/notes/n.tenn":          true,
	}
	exists := func(p string) bool { return files[filepath.ToSlash(p)] }
	roots := []Root{
		{"/work/other", "github.com/x/other"},
		{"/work/foundation", "github.com/haiodo/foundation"},
	}
	l := Link{Repo: "github.com/haiodo/foundation", Path: "design/P.tenn"}
	if p, ok := Resolve(l, roots, "/home/me", exists); !ok || p != "/work/foundation/design/P.tenn" {
		t.Fatalf("%q %v", p, ok)
	}
	l.Repo = "GitHub.com/Haiodo/Foundation"
	if _, ok := Resolve(l, roots, "/home/me", exists); !ok {
		t.Fatal("the remote is compared without case")
	}
	l.Repo = "github.com/x/unknown"
	if _, ok := Resolve(l, roots, "/home/me", exists); ok {
		t.Fatal("a clone of another repository")
	}
	l = Link{Repo: "github.com/haiodo/foundation", Path: "design/missing.tenn"}
	if _, ok := Resolve(l, roots, "/home/me", exists); ok {
		t.Fatal("the clone has no such file")
	}
	// A plain link takes the first clone that has the file.
	if p, ok := Resolve(Link{Path: "design/P.tenn"}, roots, "/home/me", exists); !ok || p != "/work/other/design/P.tenn" {
		t.Fatalf("%q %v", p, ok)
	}
	if p, ok := Resolve(Link{Home: true, Path: "notes/n.tenn"}, nil, "/home/me", exists); !ok || p != "/home/me/notes/n.tenn" {
		t.Fatalf("%q %v", p, ok)
	}
	if _, ok := Resolve(Link{Home: true, Path: "notes/n.tenn"}, nil, "", exists); ok {
		t.Fatal("no home folder")
	}
}

func TestOf(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	run := func(d string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	repo := filepath.Join(dir, "clone")
	bare := filepath.Join(dir, "norem")
	for _, d := range []string{repo, filepath.Join(repo, "design"), bare} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	run(repo, "init", "-q")
	run(repo, "remote", "add", "origin", "git@github.com:haiodo/foundation.git")
	run(bare, "init", "-q")
	file := filepath.Join(repo, "design", "P.tenn")
	var r Repos
	l, note, err := r.Of(file, dir, []string{"A"}, "n")
	if err != nil || note != "" || l.Repo != "github.com/haiodo/foundation" || l.Path != "design/P.tenn" || l.Home {
		t.Fatalf("%+v %q %v", l, note, err)
	}
	// A repository without a remote, and a folder without a repository, fall back to home.
	l, note, err = r.Of(filepath.Join(bare, "q.tenn"), dir, nil, "")
	if err != nil || !l.Home || l.Path != "norem/q.tenn" || !strings.Contains(note, "remote") {
		t.Fatalf("%+v %q %v", l, note, err)
	}
	if _, _, err = r.Of(filepath.Join(os.TempDir(), "elsewhere", "q.tenn"), dir, nil, ""); err != ErrNoBase {
		t.Fatalf("%v", err)
	}
}

// Two clones of one repository name a document alike; another remote or no repository does not.
func TestDocKeyAcrossClones(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	clone := func(name, remote string) string {
		d := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Join(d, "design"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", remote}} {
			if out, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		return d
	}
	a := clone("a", "git@github.com:haiodo/foundation.git")
	b := clone("b", "https://github.com/haiodo/foundation")
	other := clone("c", "https://github.com/haiodo/other.git")
	var r Repos
	ka := r.DocKey(filepath.Join(a, "design", "P.tenn"))
	if ka != "repo:github.com/haiodo/foundation#design/P.tenn" {
		t.Fatalf("key %q", ka)
	}
	if kb := r.DocKey(filepath.Join(b, "design", "P.tenn")); kb != ka {
		t.Fatalf("clone b: %q, want %q", kb, ka)
	}
	if kc := r.DocKey(filepath.Join(other, "design", "P.tenn")); kc == ka {
		t.Fatalf("another repository has the key %q", kc)
	}
	if kp := r.DocKey(filepath.Join(a, "design", "Q.tenn")); kp == ka {
		t.Fatal("another file has the key of P.tenn")
	}
	loose := filepath.Join(dir, "loose", "P.tenn")
	if k := r.DocKey(loose); k != "path:"+filepath.ToSlash(loose) {
		t.Fatalf("outside a repository: %q", k)
	}
}
