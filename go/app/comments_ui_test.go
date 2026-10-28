//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// With no service the app writes the files itself, in its own config folder.
func TestCommentsWithoutService(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{dir: dir}
	doc := filepath.Join(t.TempDir(), "a.tenn")
	send := func(op servicewire.CommentsOp) servicewire.CommentsUpdate {
		t.Helper()
		op.Path = doc
		up, err := sh.sendComment(op)
		if err != nil {
			t.Fatalf("%s: %v", op.Op, err)
		}
		return up
	}
	up := send(servicewire.CommentsOp{Op: servicewire.CommentAdd, Text: "tight", Anchor: &comments.Anchor{Element: []string{"A"}, Node: "x"}})
	if up.Rev != 1 || len(up.Comments) != 1 || up.Comments[0].Author != comments.User {
		t.Fatalf("add: %+v", up)
	}
	id := up.Comments[0].ID
	send(servicewire.CommentsOp{Op: servicewire.CommentReply, ID: id, Text: "more"})
	up = send(servicewire.CommentsOp{Op: servicewire.CommentResolve, ID: id})
	if c := up.Comments[0]; c.State != comments.Resolved || len(c.Replies) != 1 || up.Rev != 3 {
		t.Fatalf("resolve: %+v", up)
	}
	// A second store, as after a restart, reads the same.
	if up2 := send(servicewire.CommentsOp{Op: servicewire.CommentList}); up2.Rev != 3 || len(up2.Comments) != 1 {
		t.Fatalf("list: %+v", up2)
	}
	if ents, _ := os.ReadDir(filepath.Join(dir, "comments")); len(ents) != 1 {
		t.Fatalf("comment files: %v", ents)
	}
	if _, err := sh.sendComment(servicewire.CommentsOp{Path: doc, Op: servicewire.CommentDelete, ID: "nope"}); err == nil {
		t.Fatal("deleting a missing comment succeeded")
	}
}

func TestOnCommentsDropsOlderRevision(t *testing.T) {
	sh := &Shell{}
	v := newDoc(t, sh, "", screenText)
	v.onComments(servicewire.CommentsUpdate{Rev: 5, Comments: []comments.Comment{{ID: "a"}, {ID: "b"}}})
	v.onComments(servicewire.CommentsUpdate{Rev: 4, Comments: []comments.Comment{{ID: "old"}}})
	if v.cm.rev != 5 || len(v.cm.list) != 2 {
		t.Fatalf("an older update replaced the list: %d %+v", v.cm.rev, v.cm.list)
	}
	v.onComments(servicewire.CommentsUpdate{Rev: 5, Comments: []comments.Comment{{ID: "c"}}})
	if len(v.cm.list) != 1 {
		t.Fatalf("an update of the same revision is dropped: %+v", v.cm.list)
	}
}
