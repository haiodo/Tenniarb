package comments

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)

func TestStoreRoundTrip(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "comments"))
	key := "repo:github.com/a/b#design/P.tenn"
	if d, err := s.Load(key); err != nil || len(d.Comments) != 0 || d.Rev != 0 {
		t.Fatalf("empty document: %+v %v", d, err)
	}
	anchor := Anchor{Element: []string{"Tracker", "Boards#1"}, Node: "card/Title", Point: &Point{X: 4.5, Y: 8}}
	var id string
	d, err := s.Update(key, func(d *Doc) error {
		c, err := d.Add(User, "  too tight  ", anchor, t0)
		id = c.ID
		return err
	})
	if err != nil || d.Rev != 1 {
		t.Fatalf("add: %+v %v", d, err)
	}
	if _, err := s.Update(key, func(d *Doc) error {
		_, err := d.Reply(id, "claude-code", "padding is 4 now", t0.Add(time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(key)
	if err != nil {
		t.Fatal(err)
	}
	want := Comment{ID: id, Created: t0, Author: User, Text: "too tight", Anchor: anchor, State: Open,
		Replies: []Reply{{Author: "claude-code", Text: "padding is 4 now", Created: t0.Add(time.Minute)}}}
	if got.Rev != 2 || got.Key != key || len(got.Comments) != 1 || !reflect.DeepEqual(got.Comments[0], want) {
		t.Fatalf("loaded %+v, want %+v", got, want)
	}
	// Another document has its own file.
	if d, _ := s.Load("path:/x.tenn"); len(d.Comments) != 0 {
		t.Fatalf("documents share a file: %+v", d)
	}
}

func TestStoreWriteIsAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "comments")
	s := NewStore(dir)
	key := "path:/a.tenn"
	add := func(d *Doc) error { _, err := d.Add(User, "x", Anchor{}, t0); return err }
	for i := 0; i < 3; i++ {
		if _, err := s.Update(key, add); err != nil {
			t.Fatal(err)
		}
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 || !strings.HasSuffix(ents[0].Name(), ".json") {
		t.Fatalf("a write left more than the file: %v", ents)
	}
	// A failed change writes nothing.
	before, _ := os.ReadFile(filepath.Join(dir, ents[0].Name()))
	if _, err := s.Update(key, func(d *Doc) error { _, err := d.Reply("nope", "a", "b", t0); return err }); err != ErrNotFound {
		t.Fatalf("error %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, ents[0].Name()))
	if string(before) != string(after) {
		t.Fatal("a refused change rewrote the file")
	}
	// A file that does not parse is kept: the next write must not destroy what the user wrote.
	path := filepath.Join(dir, ents[0].Name())
	os.WriteFile(path, []byte("{broken"), 0o600)
	if _, err := s.Update(key, add); err == nil {
		t.Fatal("a broken file was overwritten")
	}
	if b, _ := os.ReadFile(path); string(b) != "{broken" {
		t.Fatalf("the broken file changed: %s", b)
	}
}

func TestStoreConcurrentUpdates(t *testing.T) {
	s := NewStore(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Update("path:/a.tenn", func(d *Doc) error { _, err := d.Add(User, "x", Anchor{}, t0); return err }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if d, _ := s.Load("path:/a.tenn"); len(d.Comments) != 20 || d.Rev != 20 {
		t.Fatalf("%d comments, rev %d", len(d.Comments), d.Rev)
	}
}

func TestDocOperations(t *testing.T) {
	var d Doc
	c, err := d.Add("", "first", Anchor{}, t0)
	if err != nil || c.Author != User || c.State != Open {
		t.Fatalf("%+v %v", c, err)
	}
	id := c.ID
	if _, err := d.Add(User, "  ", Anchor{}, t0); err != ErrEmpty {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := d.Add(User, strings.Repeat("я", MaxText+1), Anchor{}, t0); err != ErrTooLong {
		t.Fatalf("long text: %v", err)
	}
	if _, err := d.SetState(id, Resolved, "agent", "done", t0); err != nil {
		t.Fatal(err)
	}
	if got := d.Find(id); got.State != Resolved || len(got.Replies) != 1 || got.Replies[0].Text != "done" {
		t.Fatalf("resolve with text: %+v", got)
	}
	if _, err := d.SetState(id, Open, User, "", t0); err != nil || d.Find(id).State != Open || len(d.Find(id).Replies) != 1 {
		t.Fatalf("reopen: %+v %v", d.Find(id), err)
	}
	if err := d.Delete(id, "agent"); err != ErrNotYours {
		t.Fatalf("delete by another author: %v", err)
	}
	if err := d.Delete(id, User); err != nil || len(d.Comments) != 0 {
		t.Fatalf("delete: %v %v", err, d.Comments)
	}
	if err := d.Delete(id, User); err != ErrNotFound {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestConfigDirOverride(t *testing.T) {
	t.Setenv(EnvConfigDir, "/cfg")
	if got := ConfigDir(); got != "/cfg" {
		t.Fatalf("%q", got)
	}
	if got := DirIn("/cfg"); got != filepath.Join("/cfg", "comments") {
		t.Fatalf("%q", got)
	}
}
