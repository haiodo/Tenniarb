package mcpserver

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Clock is the time source of the idle timer; tests pass a fake one.
type Clock interface {
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Service hosts MCP sessions for many proxies and the documents of connected apps in one process.
type Service struct {
	Host *Host
	// Version is the BinaryVersion of this binary. A proxy that wants another one makes the service
	// stop listening; its sessions go on until their clients leave. Empty accepts any.
	Version string
	// Idle is how long the service stays without MCP sessions before it exits; 0 never exits.
	// Connected apps do not keep it alive.
	Idle  time.Duration
	Clock Clock
	Log   *log.Logger
	// PresenceTTL is how long an agent counts as working on a document after its last action in the
	// app's presence indicator; 0 means servicewire.PresenceTTL.
	PresenceTTL time.Duration

	mu        sync.Mutex
	ln        *servicewire.Listener
	sessions  int
	stopped   bool
	restart   bool
	wake      chan struct{}
	providers map[*provider]bool
	conns     map[net.Conn]bool
	wg        sync.WaitGroup
}

func (sv *Service) logf(format string, args ...any) {
	if sv.Log != nil {
		sv.Log.Printf(format, args...)
	}
}

// Serve accepts connections on ln until the service stops (idle, restart or ctx), then waits for
// the sessions in progress to end. ln is closed on return.
func (sv *Service) Serve(ctx context.Context, ln *servicewire.Listener) error {
	if sv.Host == nil {
		sv.Host = NewHost()
	}
	if sv.Clock == nil {
		sv.Clock = realClock{}
	}
	sv.mu.Lock()
	sv.ln = ln
	sv.wake = make(chan struct{}, 1)
	sv.providers = map[*provider]bool{}
	sv.conns = map[net.Conn]bool{}
	sv.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		sv.stop(false)
		sv.mu.Lock()
		for c := range sv.conns {
			c.Close()
		}
		sv.mu.Unlock()
	}()
	if sv.Idle > 0 {
		go sv.idleLoop()
	}
	if sv.PresenceTTL > 0 {
		sv.Host.presence.ttl = sv.PresenceTTL
	}
	go sv.Host.presence.run(ctx, min(max(sv.Host.presence.ttl/10, time.Second), 30*time.Second))
	for {
		c, err := ln.Accept()
		if err != nil {
			break
		}
		sv.wg.Add(1)
		go func() {
			defer sv.wg.Done()
			sv.track(c, true)
			defer sv.track(c, false)
			sv.handle(ctx, c)
		}()
	}
	sv.stop(false)
	ln.Close() // waits for a Close in progress elsewhere: the endpoint is gone on return
	sv.wg.Wait()
	return nil
}

func (sv *Service) track(c net.Conn, on bool) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	if on {
		sv.conns[c] = true
	} else {
		delete(sv.conns, c)
		c.Close()
	}
}

// stop closes the listener once, so a new service can start in the directory. After a restart the
// apps' documents stay as tombstones: old sessions must not touch files with unsaved app changes.
func (sv *Service) stop(restart bool) {
	sv.mu.Lock()
	if sv.stopped {
		sv.mu.Unlock()
		return
	}
	sv.stopped, sv.restart = true, restart
	ln := sv.ln
	var provs []*provider
	for p := range sv.providers {
		provs = append(provs, p)
	}
	sv.mu.Unlock()
	ln.Close()
	for _, p := range provs {
		p.peer.Notify(servicewire.MethodReconnect, struct{}{})
		sv.Host.live.drop(p, restart)
		p.peer.Close()
	}
	sv.signal()
}

func (sv *Service) signal() {
	select {
	case sv.wake <- struct{}{}:
	default:
	}
}

func (sv *Service) idleLoop() {
	for {
		sv.mu.Lock()
		n, stopped := sv.sessions, sv.stopped
		sv.mu.Unlock()
		if stopped {
			return
		}
		if n > 0 {
			<-sv.wake
			continue
		}
		select {
		case <-sv.Clock.After(sv.Idle):
			sv.mu.Lock()
			idle := sv.sessions == 0
			sv.mu.Unlock()
			if idle {
				sv.logf("idle for %v, exiting", sv.Idle)
				sv.stop(false)
				return
			}
		case <-sv.wake:
		}
	}
}

func (sv *Service) sessionDelta(d int) {
	sv.mu.Lock()
	sv.sessions += d
	sv.mu.Unlock()
	sv.signal()
}

func reply(c net.Conn, line string) {
	c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	c.Write([]byte(line + "\n"))
	c.SetWriteDeadline(time.Time{})
}

func (sv *Service) handle(ctx context.Context, c net.Conn) {
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReaderSize(c, 8192)
	line, err := br.ReadSlice('\n')
	if err != nil {
		return
	}
	h, err := servicewire.ParseHello(string(line[:len(line)-1]))
	if err != nil {
		reply(c, "error "+err.Error())
		return
	}
	if subtle.ConstantTimeCompare([]byte(h.Token), []byte(sv.ln.Token)) != 1 {
		reply(c, "error token")
		return
	}
	c.SetReadDeadline(time.Time{})
	conn := &servicewire.Conn{Conn: c, Reader: br}
	sv.mu.Lock()
	stopped := sv.stopped
	sv.mu.Unlock()
	if stopped {
		reply(c, "restart")
		return
	}
	switch h.Role {
	case servicewire.RoleMCP:
		sv.serveMCP(ctx, conn, h)
	case servicewire.RoleProvider:
		sv.serveProvider(ctx, conn)
	default:
		reply(c, "error unknown role "+h.Role)
	}
}

func (sv *Service) serveMCP(ctx context.Context, conn *servicewire.Conn, h servicewire.Hello) {
	if h.Version != servicewire.AnyVersion && sv.Version != "" && h.Version != sv.Version {
		sv.logf("a client wants version %s, this is %s: restarting", h.Version, sv.Version)
		// Stop first: the client starts the new service as soon as it reads the reply.
		sv.stop(true)
		reply(conn, "restart")
		return
	}
	if !filepath.IsAbs(h.Root) {
		reply(conn, "error workspace root must be an absolute path")
		return
	}
	srv, err := sv.Host.Session(h.Root)
	if err != nil {
		reply(conn, "error "+err.Error())
		return
	}
	reply(conn, "ok")
	sv.sessionDelta(1)
	defer sv.sessionDelta(-1)
	defer srv.Close()
	sv.logf("session opened at %s", srv.root)
	err = srv.MCP().Run(ctx, &mcp.IOTransport{Reader: conn, Writer: conn})
	sv.logf("session at %s closed: %v", srv.root, err)
}

func (sv *Service) serveProvider(ctx context.Context, conn *servicewire.Conn) {
	reply(conn, "ok")
	p := &provider{}
	p.peer = servicewire.NewPeer(conn, func(_ context.Context, method string, params json.RawMessage) (any, error) {
		switch method {
		case servicewire.MethodDocuments:
		case servicewire.MethodCommentsOp:
			var op servicewire.CommentsOp
			if err := json.Unmarshal(params, &op); err != nil {
				return nil, err
			}
			return sv.Host.comments.fromApp(op)
		default:
			return nil, fmt.Errorf("unknown method %q", method)
		}
		var docs servicewire.Documents
		if err := json.Unmarshal(params, &docs); err != nil {
			return nil, err
		}
		for _, abs := range sv.Host.live.set(p, docs.Docs) {
			sv.Host.comments.pushDoc(abs)
		}
		paths := make([]string, len(docs.Docs))
		for i, d := range docs.Docs {
			paths[i] = realPath(d.Path)
		}
		sv.Host.presence.markTracked(paths)
		return struct{}{}, nil
	})
	sv.mu.Lock()
	sv.providers[p] = true
	sv.mu.Unlock()
	sv.logf("app connected")
	p.peer.Serve(ctx)
	sv.mu.Lock()
	delete(sv.providers, p)
	restart := sv.restart
	sv.mu.Unlock()
	sv.Host.live.drop(p, restart)
	sv.logf("app disconnected")
}
