package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIsScheme(t *testing.T) {
	for s, want := range map[string]bool{
		"tenniarb-library://c4":        true,
		"tenniarb-library://haiodo/c4": true,
		"tenniarb-library:c4":          false,
		"library://c4":                 false,
		"c4":                           false,
		"":                             false,
	} {
		if got := IsScheme(s); got != want {
			t.Errorf("IsScheme(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Path
	}{
		{"tenniarb-library://c4", Path{Name: "c4"}},
		{"tenniarb-library://c4.tenn", Path{Name: "c4"}},
		{"tenniarb-library://haiodo/c4", Path{Owner: "haiodo", Name: "c4"}},
		{"tenniarb-library://haiodo/c4.tenn", Path{Owner: "haiodo", Name: "c4"}},
		{"tenniarb-library://Haiodo/C4", Path{Owner: "haiodo", Name: "c4"}},
	} {
		got, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
		if got.String() != tc.want.String() {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.in, got.String(), tc.want.String())
		}
	}
}

func TestParseBadName(t *testing.T) {
	for _, in := range []string{"c4", "library://c4", "tenniarb-library:c4"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) has no error", in)
		}
	}
	for _, in := range []string{
		"tenniarb-library://",
		"tenniarb-library://haiodo/",
		"tenniarb-library:///c4",
		"tenniarb-library:///tmp/c4",
		"tenniarb-library://haiodo/c4/extra",
		"tenniarb-library://haiodo/c 4",
		"tenniarb-library://..",
		"tenniarb-library://haiodo/..",
		"tenniarb-library://../c4",
		"tenniarb-library://haiodo/.htaccess",
		"tenniarb-library://haiodo/c4\\x",
		"tenniarb-library://haiodo/C_4",
		"tenniarb-library://haiodo/-c4",
	} {
		_, err := Parse(in)
		if err == nil {
			t.Errorf("Parse(%q) has no error", in)
			continue
		}
		if !strings.Contains(err.Error(), nameRe.String()) {
			t.Errorf("Parse(%q) error %q does not list the rule %s", in, err, nameRe)
		}
	}
}

func TestPathText(t *testing.T) {
	for _, tc := range []struct {
		p        Path
		str, rel string
	}{
		{Path{Name: "c4"}, "tenniarb-library://c4", "c4.tenn"},
		{Path{Owner: "haiodo", Name: "c4"}, "tenniarb-library://haiodo/c4", "haiodo/c4.tenn"},
	} {
		if got := tc.p.String(); got != tc.str {
			t.Errorf("Path%+v.String() = %q, want %q", tc.p, got, tc.str)
		}
		if got := tc.p.Rel(); got != tc.rel {
			t.Errorf("Path%+v.Rel() = %q, want %q", tc.p, got, tc.rel)
		}
	}
}

func TestResolve(t *testing.T) {
	installed, shipped := t.TempDir(), t.TempDir()
	touch := func(root, rel string) string {
		file := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	shippedC4 := touch(shipped, "c4.tenn")
	installedC4 := touch(installed, "haiodo/c4.tenn")
	touch(shipped, "haiodo/c4.tenn") // the installed copy shadows this one
	shippedBase := touch(shipped, "haiodo/base.tenn")
	roots := Roots{Installed: installed, Shipped: shipped}

	for _, tc := range []struct {
		p    Path
		want string
	}{
		{Path{Name: "c4"}, shippedC4},
		{Path{Owner: "haiodo", Name: "c4"}, installedC4},
		{Path{Owner: "haiodo", Name: "base"}, shippedBase}, // an owner is not installed-only
		{Path{Owner: "haiodo", Name: "nope"}, ""},
		{Path{Name: "base"}, ""},
	} {
		got, err := roots.Resolve(tc.p)
		if tc.want == "" {
			if err == nil {
				t.Errorf("Resolve(%+v) = %q, want an error", tc.p, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Resolve(%+v): %v", tc.p, err)
		} else if got != tc.want {
			t.Errorf("Resolve(%+v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestResolveUnknownNamesBothRoots(t *testing.T) {
	installed, shipped := t.TempDir(), t.TempDir()
	_, err := Roots{Installed: installed, Shipped: shipped}.Resolve(Path{Owner: "haiodo", Name: "c4"})
	if err == nil {
		t.Fatal("Resolve of a missing library has no error")
	}
	for _, want := range []string{installed, shipped, "haiodo/c4.tenn"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// A Path built by hand must not reach a file above the root.
func TestResolveHandBuiltTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "secret.tenn")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	roots := Roots{Installed: root, Shipped: root}
	for _, p := range []Path{
		{Name: ".."},
		{Owner: "..", Name: ".."},
		{Owner: "haiodo", Name: "../secret"},
		{Name: ""},
		{Owner: "haiodo", Name: ""},
		{Owner: "/abs", Name: "c4"},
	} {
		got, err := roots.Resolve(p)
		if err == nil {
			t.Errorf("Resolve(%+v) = %q, want an error", p, got)
		}
	}
}

func TestLoadIndexMissing(t *testing.T) {
	list, err := LoadIndex(filepath.Join(t.TempDir(), "nowhere"))
	if err != nil || list != nil {
		t.Fatalf("LoadIndex of a missing file = %v, %v; want nil, nil", list, err)
	}
}

func TestLoadIndexMalformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, IndexName()), []byte(`{"owner":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIndex(dir); err == nil {
		t.Fatal("LoadIndex of malformed JSON has no error")
	}
}

func TestSaveIndex(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config", "nested")
	in := []Entry{
		{Owner: "zeta", Name: "b", URL: "https://example.com/zeta/b.git", Ref: "main"},
		{Owner: "haiodo", Name: "c4", URL: "https://example.com/haiodo/c4.git"},
		{Owner: "haiodo", Name: "base", URL: "https://example.com/haiodo/base.git"},
	}
	if err := SaveIndex(dir, in); err != nil {
		t.Fatal(err)
	}
	list, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{in[2], in[1], in[0]}
	if !slices.Equal(list, want) {
		t.Errorf("LoadIndex = %+v, want %+v", list, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != IndexName() {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("index directory holds %v, want only %s", names, IndexName())
	}
	if list[0].URL != in[2].URL || list[0].Ref != in[2].Ref {
		t.Errorf("fields lost in the round trip: %+v", list[0])
	}
	if err := SaveIndex(dir, in[:1]); err != nil {
		t.Fatal(err)
	}
	if again, err := LoadIndex(dir); err != nil || !slices.Equal(again, in[:1]) {
		t.Errorf("second save = %+v, %v; want %+v", again, err, in[:1])
	}
}
