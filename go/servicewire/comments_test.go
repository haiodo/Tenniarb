package servicewire

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/comments"
)

type commentsProvider struct {
	Provider
	got []CommentsUpdate
}

func (p *commentsProvider) Comments(u CommentsUpdate) { p.got = append(p.got, u) }

func TestCommentsRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	in := CommentsUpdate{V: CommentsVersion, Path: "/a.tenn", Rev: 3, Comments: []comments.Comment{{
		ID: "ab12", Created: at, Author: comments.User, Text: "tight", State: comments.Open,
		Anchor:  comments.Anchor{Element: []string{"A", "B#1"}, Node: "card/Title", Point: &comments.Point{X: 1.5, Y: 2}},
		Replies: []comments.Reply{{Author: "claude-code", Text: "fixed", Created: at.Add(time.Minute)}},
	}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	pp := &commentsProvider{}
	c := &ProviderClient{p: pp}
	if _, err := c.handle(context.Background(), MethodComments, raw); err != nil {
		t.Fatal(err)
	}
	if len(pp.got) != 1 || !reflect.DeepEqual(pp.got[0], in) {
		t.Fatalf("got %+v, want %+v", pp.got, in)
	}

	// A version this reader does not know is dropped, and a provider without the interface is fine.
	newer, _ := json.Marshal(CommentsUpdate{V: CommentsVersion + 1, Path: "/a.tenn", Rev: 9})
	if _, err := c.handle(context.Background(), MethodComments, newer); err != nil || len(pp.got) != 1 {
		t.Errorf("newer version: err %v, got %d", err, len(pp.got))
	}
	if _, err := (&ProviderClient{}).handle(context.Background(), MethodComments, raw); err != nil {
		t.Errorf("provider without CommentsReceiver: %v", err)
	}

	op := CommentsOp{Path: "/a.tenn", Op: CommentAdd, Text: "x", Anchor: &in.Comments[0].Anchor}
	raw, _ = json.Marshal(op)
	var back CommentsOp
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, op) {
		t.Errorf("op: %+v %v", back, err)
	}
}

func TestCommentsNotConnected(t *testing.T) {
	c := &ProviderClient{}
	if _, err := c.Comments(CommentsOp{Path: "/a.tenn", Op: CommentList}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("error %v", err)
	}
}
