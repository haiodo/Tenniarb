package servicewire

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// message is one line of a provider connection: a request (ID and Method), a notification
// (Method, no ID) or the response to a request (ID with Result or Error).
type message struct {
	ID     int64           `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// maxMessage caps one line: the text of a document travels in one message.
const maxMessage = 256 << 20

// Handler answers the requests and notifications a Peer receives. A notification's result is dropped.
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// Peer is one end of a provider connection; both ends call and answer.
type Peer struct {
	rw      io.ReadWriteCloser
	handle  Handler
	wmu     sync.Mutex
	w       *bufio.Writer
	mu      sync.Mutex
	next    int64
	pending map[int64]chan message
	done    chan struct{}
	err     error
}

// NewPeer wraps rw; Serve must run for calls to get answers.
func NewPeer(rw io.ReadWriteCloser, handle Handler) *Peer {
	return &Peer{rw: rw, handle: handle, w: bufio.NewWriter(rw), pending: map[int64]chan message{}, done: make(chan struct{})}
}

// Done is closed when the connection ends.
func (p *Peer) Done() <-chan struct{} { return p.done }

// Close ends the connection; pending calls fail.
func (p *Peer) Close() error { return p.rw.Close() }

func (p *Peer) send(m message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	p.wmu.Lock()
	defer p.wmu.Unlock()
	p.w.Write(b)
	p.w.WriteByte('\n')
	return p.w.Flush()
}

// ErrClosed is returned by calls on a connection that has ended.
var ErrClosed = errors.New("connection closed")

// Call sends a request and decodes the result into result (unless nil).
func (p *Peer) Call(ctx context.Context, method string, params, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	ch := make(chan message, 1)
	p.mu.Lock()
	if p.pending == nil {
		p.mu.Unlock()
		return ErrClosed
	}
	p.next++
	id := p.next
	p.pending[id] = ch
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
	}()
	if err := p.send(message{ID: id, Method: method, Params: raw}); err != nil {
		return ErrClosed
	}
	select {
	case m := <-ch:
		if m.Error != "" {
			return errors.New(m.Error)
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(m.Result, result)
	case <-p.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Notify sends a notification.
func (p *Peer) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return p.send(message{Method: method, Params: raw})
}

// Serve reads until the connection ends. Requests run concurrently, each in its own goroutine.
func (p *Peer) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sc := bufio.NewScanner(p.rw)
	sc.Buffer(make([]byte, 64<<10), maxMessage)
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		if m.Method == "" {
			p.mu.Lock()
			ch := p.pending[m.ID]
			p.mu.Unlock()
			if ch != nil {
				ch <- m
			}
			continue
		}
		go p.answer(ctx, m)
	}
	p.mu.Lock()
	p.pending = nil
	p.err = sc.Err()
	p.mu.Unlock()
	close(p.done)
	p.rw.Close()
	return p.err
}

func (p *Peer) answer(ctx context.Context, m message) {
	res, err := p.handle(ctx, m.Method, m.Params)
	if m.ID == 0 {
		return
	}
	out := message{ID: m.ID}
	if err != nil {
		out.Error = err.Error()
		if out.Error == "" {
			out.Error = "error"
		}
	} else if out.Result, err = json.Marshal(res); err != nil {
		out.Result, out.Error = nil, err.Error()
	}
	p.send(out)
}
