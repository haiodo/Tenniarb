package servicewire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Methods of a provider connection. The service calls the provider with MethodText..MethodRender;
// the provider calls MethodDocuments; the service sends MethodReconnect and MethodPresence as
// notifications.
const (
	MethodDocuments = "documents"
	MethodText      = "text"
	MethodApply     = "apply"
	MethodSelection = "selection"
	MethodSelect    = "select"
	MethodRender    = "render"
	MethodReconnect = "reconnect"
	MethodPresence  = "presence"
)

// PresenceTTL is how long an agent counts as working on a document after its last action. The
// service drops it then, and the app does the same with a presence the service never cleared.
const PresenceTTL = 10 * time.Minute

// PresenceVersion is the version of the Presence message. New optional fields keep it; a change that
// an older reader would misread raises it, and a reader drops messages of a version it does not know.
// The hello protocol does not change with it: a peer that does not know MethodPresence ignores it.
const PresenceVersion = 1

// Doc is a document the app has open. Version must change on every change of the text (edit, undo,
// redo, reload); Dirty says the text differs from the file on disk.
type Doc struct {
	Path    string `json:"path"` // absolute
	Version int64  `json:"version"`
	Dirty   bool   `json:"dirty"`
}

// Documents is the full set of open documents; each MethodDocuments replaces the previous set.
type Documents struct {
	Docs []Doc `json:"docs"`
}

// Presence says which agents work on one open document through MCP. The service sends the current
// state of a document whenever it changes: an agent acts, goes quiet for too long or leaves. Agents
// 0 clears it.
type Presence struct {
	V      int    `json:"v"`
	Path   string `json:"path"` // as the app sent it in Doc
	Agents int    `json:"agents"`
	// Seq orders messages of one service, which are handled concurrently: a message older than the
	// last one seen is dropped. It grows with the clock, so it also grows across service restarts.
	Seq int64 `json:"seq"`
	// The rest describes the latest action among the agents.
	Name    string `json:"name,omitempty"`    // client name from the MCP initialize, may be empty
	Action  string `json:"action,omitempty"`  // tool name: read, edit, select ...
	At      int64  `json:"at,omitempty"`      // Unix milliseconds; each action has its own, growing value
	Element string `json:"element,omitempty"` // dotted path (spec 4.2) of the element the action touched
	Node    string `json:"node,omitempty"`    // dotted path of the innermost node the action touched
	Line    int    `json:"line,omitempty"`    // 1-based lines of the touched text, when known
	EndLine int    `json:"endLine,omitempty"`
}

// PresenceReceiver is implemented by a Provider that wants Presence notifications. It runs on a
// goroutine of the client, concurrently with other calls.
type PresenceReceiver interface {
	Presence(Presence)
}

// TextArgs asks for the current text of a document.
type TextArgs struct {
	Path string `json:"path"`
}

// TextResult is the text of a document as the app holds it, unsaved edits included.
type TextResult struct {
	Text    string `json:"text"`
	Version int64  `json:"version"`
	Dirty   bool   `json:"dirty"`
}

// ApplyArgs replaces the text of a document as one undo step. Base is the version the text was
// computed from: the app refuses when its document is at another version.
type ApplyArgs struct {
	Path string `json:"path"`
	Base int64  `json:"base"`
	Text string `json:"text"`
}

// ApplyResult is the version after the change.
type ApplyResult struct {
	Version int64 `json:"version"`
}

// Selection is what the user looks at: the document of the key window, the element on screen and
// the selected nodes, as dotted paths (spec 4.2) from the root of that file.
type Selection struct {
	Path    string   `json:"path,omitempty"`
	Element string   `json:"element,omitempty"`
	Nodes   []string `json:"nodes,omitempty"`
}

// SelectArgs selects nodes of a document (dotted paths); Reveal also brings the window to front and
// scrolls the first node into view.
type SelectArgs struct {
	Path   string   `json:"path"`
	Nodes  []string `json:"nodes"`
	Reveal bool     `json:"reveal,omitempty"`
}

// RenderArgs asks for a picture of the current view of a document at scale device pixels per point.
type RenderArgs struct {
	Path  string  `json:"path"`
	Scale float64 `json:"scale,omitempty"`
}

// RenderResult is a PNG image.
type RenderResult struct {
	PNG []byte `json:"png"`
}

// Provider is what the app implements. The service calls it from its own goroutines, several at
// once: an implementation hands the work to the UI thread itself.
type Provider interface {
	// Text returns the current text of an open document.
	Text(path string) (TextResult, error)
	// Apply replaces the text as one undo step if the document is still at base.
	Apply(path string, base int64, text string) (version int64, err error)
	Selection() (Selection, error)
	Select(path string, nodes []string, reveal bool) error
	// Render draws what the window of the document shows.
	Render(path string, scale float64) ([]byte, error)
}

// ProviderClient keeps the app connected to the service: it dials, sends the open documents and
// answers calls, and dials again when the service goes away (idle exit, restart, crash).
type ProviderClient struct {
	dir  string
	p    Provider
	poll time.Duration

	mu        sync.Mutex
	docs      []Doc
	gen       int // counts SetDocuments calls
	peer      *Peer
	connected chan struct{} // closed while a connection is up; replaced on loss
}

// RunProvider connects p to the service of dir (Dir() when empty) until ctx ends. It never starts
// the service: an agent's proxy does; the client polls every poll (2s when zero) until one runs.
func RunProvider(ctx context.Context, dir string, p Provider, poll time.Duration) (*ProviderClient, error) {
	if dir == "" {
		d, err := Dir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	if poll <= 0 {
		poll = 2 * time.Second
	}
	c := &ProviderClient{dir: dir, p: p, poll: poll, connected: make(chan struct{})}
	go c.loop(ctx)
	return c, nil
}

// SetDocuments sends the full set of open documents; call it on open, close, save and text change.
// The set is resent on every reconnect. Returns ErrNotConnected while no service runs.
func (c *ProviderClient) SetDocuments(docs []Doc) error {
	c.mu.Lock()
	c.docs = append([]Doc(nil), docs...)
	c.gen++
	peer := c.peer
	c.mu.Unlock()
	if peer == nil {
		return ErrNotConnected
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return peer.Call(ctx, MethodDocuments, Documents{Docs: docs}, nil)
}

// ErrNotConnected is returned by SetDocuments while no service runs.
var ErrNotConnected = errors.New("tenniarb-service is not connected")

// WaitConnected blocks until a connection is up and the documents are registered.
func (c *ProviderClient) WaitConnected(ctx context.Context) error {
	c.mu.Lock()
	ch := c.connected
	c.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *ProviderClient) loop(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := Dial(c.dir, Hello{Role: RoleProvider, Version: AnyVersion})
		if err != nil {
			select {
			case <-time.After(c.poll):
			case <-ctx.Done():
			}
			continue
		}
		peer := NewPeer(conn, c.handle)
		served := make(chan struct{})
		go func() { peer.Serve(ctx); close(served) }()
		c.register(ctx, peer)
		select {
		case <-served:
		case <-ctx.Done():
			peer.Close()
			<-served
		}
		c.mu.Lock()
		if c.peer == peer {
			c.peer = nil
			c.connected = make(chan struct{})
		}
		c.mu.Unlock()
	}
}

// register publishes peer once the documents it sent did not change during the call: a
// SetDocuments in flight saw no peer and returned ErrNotConnected.
func (c *ProviderClient) register(ctx context.Context, peer *Peer) {
	for {
		c.mu.Lock()
		docs, gen := c.docs, c.gen
		c.mu.Unlock()
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := peer.Call(rctx, MethodDocuments, Documents{Docs: docs}, nil)
		cancel()
		if err != nil {
			return
		}
		c.mu.Lock()
		if c.gen == gen {
			c.peer = peer
			close(c.connected)
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
}

func (c *ProviderClient) handle(_ context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case MethodText:
		var a TextArgs
		if err := json.Unmarshal(params, &a); err != nil {
			return nil, err
		}
		return c.p.Text(a.Path)
	case MethodApply:
		var a ApplyArgs
		if err := json.Unmarshal(params, &a); err != nil {
			return nil, err
		}
		v, err := c.p.Apply(a.Path, a.Base, a.Text)
		return ApplyResult{Version: v}, err
	case MethodSelection:
		return c.p.Selection()
	case MethodSelect:
		var a SelectArgs
		if err := json.Unmarshal(params, &a); err != nil {
			return nil, err
		}
		return struct{}{}, c.p.Select(a.Path, a.Nodes, a.Reveal)
	case MethodRender:
		var a RenderArgs
		if err := json.Unmarshal(params, &a); err != nil {
			return nil, err
		}
		png, err := c.p.Render(a.Path, a.Scale)
		return RenderResult{PNG: png}, err
	case MethodPresence:
		var pr Presence
		if err := json.Unmarshal(params, &pr); err != nil {
			return nil, err
		}
		if r, ok := c.p.(PresenceReceiver); ok && pr.V <= PresenceVersion {
			r.Presence(pr)
		}
		return nil, nil
	case MethodReconnect:
		// The service stops; Serve ends when it closes the connection and loop dials again.
		return nil, nil
	}
	return nil, fmt.Errorf("unknown method %q", method)
}
