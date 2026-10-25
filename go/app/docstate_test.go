package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPushRecent(t *testing.T) {
	if got := PushRecent([]string{"/a", "/b"}, "/b"); !slices.Equal(got, []string{"/b", "/a"}) {
		t.Fatalf("move to front: %v", got)
	}
	var many []string
	for i := 0; i < RecentLimit; i++ {
		many = append(many, fmt.Sprintf("/f%d", i))
	}
	next := PushRecent(many, "/new")
	if len(next) != RecentLimit || next[0] != "/new" || slices.Contains(next, "/f9") {
		t.Fatalf("limit: %v", next)
	}
	if many[0] != "/f0" {
		t.Fatal("PushRecent changed its argument")
	}
}

func TestRecentFile(t *testing.T) {
	dir := t.TempDir()
	if got := LoadRecent(dir); len(got) != 0 {
		t.Fatalf("empty dir: %v", got)
	}
	if _, err := AddRecent(dir, "/a.tenn"); err != nil {
		t.Fatal(err)
	}
	if got, _ := AddRecent(dir, "/b.tenn"); !slices.Equal(got, []string{"/b.tenn", "/a.tenn"}) {
		t.Fatalf("add: %v", got)
	}
	if got := LoadRecent(dir); !slices.Equal(got, []string{"/b.tenn", "/a.tenn"}) {
		t.Fatalf("reload: %v", got)
	}
	os.WriteFile(filepath.Join(dir, recentFile), []byte("{broken"), 0o644)
	if got := LoadRecent(dir); len(got) != 0 {
		t.Fatalf("malformed file: %v", got)
	}
}

func TestFrames(t *testing.T) {
	good := `{"/a":{"x":-10,"y":20.5,"width":944,"height":764},"/bad":{"x":1,"y":2,"width":0,"height":5},"/str":{"x":"1","y":2,"width":3,"height":4},"/null":null}`
	got := ParseFrames([]byte(good))
	if len(got) != 1 || got["/a"] != (Frame{-10, 20.5, 944, 764}) {
		t.Fatalf("parse: %v", got)
	}
	if len(ParseFrames([]byte("nope"))) != 0 {
		t.Fatal("malformed json")
	}

	dir := t.TempDir()
	if _, ok := LoadFrame(dir, "/a"); ok {
		t.Fatal("frame in an empty dir")
	}
	if err := SaveFrame(dir, "/a", Frame{1, 2, 300, 200}); err != nil {
		t.Fatal(err)
	}
	if err := SaveFrame(dir, "/b", Frame{5, 6, 700, 500}); err != nil {
		t.Fatal(err)
	}
	if f, ok := LoadFrame(dir, "/a"); !ok || f != (Frame{1, 2, 300, 200}) {
		t.Fatalf("a: %v %v", f, ok)
	}
	if f, ok := LoadFrame(dir, "/b"); !ok || f != (Frame{5, 6, 700, 500}) {
		t.Fatalf("b: %v %v", f, ok)
	}
}

func TestFrameOnScreen(t *testing.T) {
	if !(Frame{100, 100, 800, 600}).OnScreen(1920, 1080) {
		t.Fatal("inside")
	}
	if (Frame{3000, 100, 800, 600}).OnScreen(1920, 1080) {
		t.Fatal("a monitor that is gone")
	}
	if (Frame{100, -400, 800, 600}).OnScreen(1920, 1080) {
		t.Fatal("title bar above the screen")
	}
}

func TestUntitledStash(t *testing.T) {
	dir := t.TempDir()
	if ids := StashedUntitled(dir); len(ids) != 0 {
		t.Fatalf("empty: %v", ids)
	}
	if err := StashUntitled(dir, "untitled-1", "element \"A\" {\n}\n"); err != nil {
		t.Fatal(err)
	}
	StashUntitled(dir, "untitled-2", "x")
	if ids := StashedUntitled(dir); !slices.Equal(ids, []string{"untitled-1", "untitled-2"}) {
		t.Fatalf("list: %v", ids)
	}
	if text, err := ReadUntitled(dir, "untitled-1"); err != nil || text != "element \"A\" {\n}\n" {
		t.Fatalf("read: %q %v", text, err)
	}
	DropUntitled(dir, "untitled-1")
	DropUntitled(dir, "untitled-1") // twice is fine
	if ids := StashedUntitled(dir); !slices.Equal(ids, []string{"untitled-2"}) {
		t.Fatalf("after drop: %v", ids)
	}
	if ParseText(NewDocumentText).Model == nil {
		t.Fatal("NewDocumentText does not parse")
	}
}

func TestDecideClose(t *testing.T) {
	cases := []struct {
		path            string
		dirty, quitting bool
		want            CloseAction
	}{
		{"", false, false, CloseNow},
		{"/a.tenn", false, true, CloseNow},
		{"/a.tenn", true, false, CloseSave},
		{"/a.tenn", true, true, CloseSave},
		{"", true, false, CloseAsk},
		{"", true, true, CloseStash},
	}
	for _, c := range cases {
		if got := DecideClose(c.path, c.dirty, c.quitting); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
