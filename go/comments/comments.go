// Package comments is the model and the storage of the comments a user leaves on a design for an
// agent to answer. They live in the user's config folder, one JSON file per document, never in the
// repository. The package uses the standard library only: the servicewire messages carry its types.
package comments

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Authors and states.
const (
	// User is the author of the comments and replies written in the app.
	User     = "user"
	Open     = "open"
	Resolved = "resolved"
)

// Kinds of the events agents wait for: what the user did.
const (
	EventComment = "comment"
	EventReply   = "reply"
	EventReopen  = "reopen"
)

// MaxText caps the text of one comment or reply, in characters.
const MaxText = 10000

// Point is a place in the coordinates of its anchor: the offset from the top left corner of the node
// box (y down, scene units) or, without a node, from the origin of the element content.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Anchor says what a comment is about. Element holds the names from the root down as the app's PathOf
// writes them ("#n" for a repeat); Node is the selection path in that element (the app's NodePath, "a/b"
// reaches into an instance) or the name of a diagram item. Both may be empty.
type Anchor struct {
	Element []string `json:"element,omitempty"`
	Node    string   `json:"node,omitempty"`
	Point   *Point   `json:"point,omitempty"`
}

// Reply is one answer in a thread; threads are one level deep.
type Reply struct {
	Author  string    `json:"author"`
	Text    string    `json:"text"`
	Created time.Time `json:"created"`
}

// Comment is a thread: the text of its author and the replies to it.
type Comment struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	Author  string    `json:"author"`
	Text    string    `json:"text"`
	Anchor  Anchor    `json:"anchor"`
	State   string    `json:"state"`
	Replies []Reply   `json:"replies,omitempty"`
}

// Doc is the comments of one document.
type Doc struct {
	V int `json:"v"`
	// Key is the identity of the document (link.Repos.DocKey).
	Key string `json:"key"`
	// Rev grows with every write; a reader drops an update older than the one it holds.
	Rev      int64     `json:"rev"`
	Comments []Comment `json:"comments"`
}

// Version is the version of the file format. A field added later keeps it.
const Version = 1

// Errors of the operations; the service shows them to the agent and the app as they are.
var (
	ErrNotFound = errors.New("no such comment")
	ErrEmpty    = errors.New("the text is empty")
	ErrTooLong  = fmt.Errorf("the text is longer than %d characters", MaxText)
	ErrNotYours = errors.New("only the author can delete a comment")
)

func checkText(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", ErrEmpty
	case utf8.RuneCountInString(text) > MaxText:
		return "", ErrTooLong
	}
	return text, nil
}

func newID(taken func(string) bool) string {
	for {
		var b [4]byte
		rand.Read(b[:])
		if id := hex.EncodeToString(b[:]); !taken(id) {
			return id
		}
	}
}

// Find returns the comment with id.
func (d *Doc) Find(id string) *Comment {
	for i := range d.Comments {
		if d.Comments[i].ID == id {
			return &d.Comments[i]
		}
	}
	return nil
}

// Add appends an open comment of author.
func (d *Doc) Add(author, text string, a Anchor, now time.Time) (*Comment, error) {
	text, err := checkText(text)
	if err != nil {
		return nil, err
	}
	if author == "" {
		author = User
	}
	d.Comments = append(d.Comments, Comment{
		ID: newID(func(id string) bool { return d.Find(id) != nil }), Created: now, Author: author, Text: text, Anchor: a, State: Open,
	})
	return &d.Comments[len(d.Comments)-1], nil
}

// Reply adds a reply to the thread id.
func (d *Doc) Reply(id, author, text string, now time.Time) (*Comment, error) {
	c := d.Find(id)
	if c == nil {
		return nil, ErrNotFound
	}
	text, err := checkText(text)
	if err != nil {
		return nil, err
	}
	c.Replies = append(c.Replies, Reply{Author: author, Text: text, Created: now})
	return c, nil
}

// SetState resolves or reopens the thread id. A resolve may bring a closing reply; its text may be empty.
func (d *Doc) SetState(id, state, author, text string, now time.Time) (*Comment, error) {
	c := d.Find(id)
	if c == nil {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(text) != "" {
		if _, err := d.Reply(id, author, text, now); err != nil {
			return nil, err
		}
	}
	c.State = state
	return c, nil
}

// Delete removes the thread id; only its author may.
func (d *Doc) Delete(id, author string) error {
	for i := range d.Comments {
		if d.Comments[i].ID != id {
			continue
		}
		if d.Comments[i].Author != author {
			return ErrNotYours
		}
		d.Comments = append(d.Comments[:i], d.Comments[i+1:]...)
		return nil
	}
	return ErrNotFound
}

// OpenOf lists the comments in state open.
func OpenOf(cs []Comment) []Comment {
	var out []Comment
	for _, c := range cs {
		if c.State == Open {
			out = append(out, c)
		}
	}
	return out
}
