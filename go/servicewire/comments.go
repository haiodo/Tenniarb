package servicewire

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/haiodo/tenniarb/go/comments"
)

// The comments of a document are stored by the service, the only writer. The app changes them with
// MethodCommentsOp and hears of every change, its own included, in a MethodComments notification.
const (
	MethodCommentsOp = "comments.op" // app -> service, answered with CommentsUpdate
	MethodComments   = "comments"    // service -> app notification, CommentsUpdate
)

// CommentsVersion is the version of CommentsUpdate. New optional fields keep it; a reader drops a
// message of a version it does not know.
const CommentsVersion = 1

// Operations of CommentsOp.
const (
	CommentList    = "list"    // no change: the current comments
	CommentAdd     = "add"     // Text, Anchor
	CommentReply   = "reply"   // ID, Text
	CommentResolve = "resolve" // ID; Text is an optional closing reply
	CommentReopen  = "reopen"  // ID
	CommentDelete  = "delete"  // ID; only a comment of the user
)

// CommentsOp is a change by the user of the app; the service writes it as the author "user".
type CommentsOp struct {
	Path   string           `json:"path"` // absolute path of the document, as in Doc
	Op     string           `json:"op"`
	ID     string           `json:"id,omitempty"`
	Text   string           `json:"text,omitempty"`
	Anchor *comments.Anchor `json:"anchor,omitempty"`
}

// Apply makes the change of op to d as the user. It returns the id of the comment it touched and the
// kind of event an agent waiting for comments is woken with (comments.Event*), "" for none. The service
// and an app without a service run the same code.
func (op CommentsOp) Apply(d *comments.Doc, now time.Time) (id, kind string, err error) {
	switch op.Op {
	case CommentAdd:
		a := comments.Anchor{}
		if op.Anchor != nil {
			a = *op.Anchor
		}
		c, err := d.Add(comments.User, op.Text, a, now)
		if err != nil {
			return "", "", err
		}
		return c.ID, comments.EventComment, nil
	case CommentReply:
		_, err = d.Reply(op.ID, comments.User, op.Text, now)
		return op.ID, comments.EventReply, err
	case CommentResolve:
		_, err = d.SetState(op.ID, comments.Resolved, comments.User, op.Text, now)
		if strings.TrimSpace(op.Text) != "" {
			kind = comments.EventReply
		}
		return op.ID, kind, err
	case CommentReopen:
		_, err = d.SetState(op.ID, comments.Open, comments.User, "", now)
		return op.ID, comments.EventReopen, err
	case CommentDelete:
		return op.ID, "", d.Delete(op.ID, comments.User)
	}
	return "", "", fmt.Errorf("unknown comment operation %q", op.Op)
}

// CommentsUpdate is all the comments of one document. Messages are handled concurrently: a reader drops
// one whose Rev is lower than the one it holds.
type CommentsUpdate struct {
	V        int                `json:"v"`
	Path     string             `json:"path"` // as the app sent it in Doc
	Rev      int64              `json:"rev"`
	Comments []comments.Comment `json:"comments"`
}

// CommentsReceiver is implemented by a Provider that shows comments. It runs on a goroutine of the
// client, concurrently with other calls.
type CommentsReceiver interface {
	Comments(CommentsUpdate)
}

// commentsTimeout bounds one change: the service writes a small file.
const commentsTimeout = 10 * time.Second

// Comments sends op to the service and returns the comments of the document after it. It returns
// ErrNotConnected while no service runs.
func (c *ProviderClient) Comments(op CommentsOp) (CommentsUpdate, error) {
	c.mu.Lock()
	peer := c.peer
	c.mu.Unlock()
	if peer == nil {
		return CommentsUpdate{}, ErrNotConnected
	}
	ctx, cancel := context.WithTimeout(context.Background(), commentsTimeout)
	defer cancel()
	var up CommentsUpdate
	err := peer.Call(ctx, MethodCommentsOp, op, &up)
	return up, err
}
