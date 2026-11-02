//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
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

// TestCommentInAMountedFile: a comment on an element the document shows from a file belongs to that file,
// its anchor a path inside it; the window still shows it at the path of the canvas.
func TestCommentInAMountedFile(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{dir: t.TempDir()}
	lib := filepath.Join(dir, "lib.tenn")
	libText2 := "element \"Panel\" {\n  item Screen { auto-layout column; width 100; height 100 }\n}\n"
	if err := os.WriteFile(lib, []byte(libText2), 0o644); err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, sh, filepath.Join(dir, "screen.tenn"), "element lib --from-file \"lib.tenn\"\n")
	panel := v.session.Root().Elements[0].Elements[0]
	if panel == nil || panel.Name != "Panel" {
		t.Fatalf("the mount has no element Panel: %+v", v.session.Root().Elements[0].Elements)
	}
	v.state.Current = panel
	anchor := comments.Anchor{Element: PathOf(panel), Point: &comments.Point{X: 3, Y: 4}}
	file, a, ok := v.anchorInFile(anchor)
	if !ok || realPath(file) != realPath(lib) || !slices.Equal(a.Element, []string{"Panel"}) {
		t.Fatalf("anchor in file: ok %v file %q element %v", ok, file, a.Element)
	}
	up, err := sh.sendComment(servicewire.CommentsOp{Path: file, Op: servicewire.CommentAdd, Text: "here", Anchor: &a})
	if err != nil {
		t.Fatal(err)
	}
	v.commentsArrived(up)
	if len(v.cm.list) != 1 || !slices.Equal(v.cm.list[0].Anchor.Element, []string{"lib", "Panel"}) {
		t.Fatalf("the window does not show the comment of the file: %+v", v.cm.list)
	}
	if got := v.commentFile(v.cm.list[0].ID); got != realPath(file) {
		t.Fatalf("reply would go to %q", got)
	}
	// the file itself lists it, as an agent working on it asks
	if up, err := sh.localComment(servicewire.CommentsOp{Path: file, Op: servicewire.CommentList}); err != nil || len(up.Comments) != 1 {
		t.Fatalf("stored under the file: %+v %v", up, err)
	}
}
