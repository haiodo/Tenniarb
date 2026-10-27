package mcpserver

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serviceDir is a private temp directory short enough for a unix socket; never the user's.
func serviceDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "tms")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

type running struct {
	sv   *Service
	done chan error
	over bool
}

// startService runs a Service on dir in this process until the test ends.
func startService(t *testing.T, dir, version string, clock Clock, idle time.Duration) *running {
	t.Helper()
	ln, err := servicewire.Listen(dir, version)
	if err != nil {
		t.Fatal(err)
	}
	r := &running{sv: &Service{Version: version, Idle: idle, Clock: clock}, done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { r.done <- r.sv.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		if !r.stopped(5 * time.Second) {
			t.Error("service did not stop")
		}
	})
	return r
}

func (r *running) stopped(within time.Duration) bool {
	if r.over {
		return true
	}
	select {
	case <-r.done:
		r.over = true
		return true
	case <-time.After(within):
		return false
	}
}

// session connects an MCP client to the service of dir through the wire, as the proxy does.
func session(t *testing.T, dir, root, version string, start func() error) *mcp.ClientSession {
	t.Helper()
	conn, err := servicewire.Connect(dir, servicewire.Hello{Role: servicewire.RoleMCP, Version: version, Root: root}, start, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(context.Background(), &mcp.IOTransport{Reader: conn, Writer: conn}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func tempRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestServiceConcurrentSessions(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	roots := []string{
		tempRoot(t, map[string]string{"a.tenn": "element \"A\" {\n    item \"x\"\n}\n"}),
		tempRoot(t, map[string]string{"b.tenn": "element \"B\" {\n    item \"y\"\n}\n"}),
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			root := roots[i%2]
			mine, other := "a.tenn", "b.tenn"
			if i%2 == 1 {
				mine, other = other, mine
			}
			cs := session(t, dir, root, "v1", nil)
			state := mustOK(t, cs, "get_state", nil)
			if !strings.Contains(state, mine) || strings.Contains(state, other) {
				t.Errorf("session %d at %s sees: %s", i, root, state)
			}
			mustFail(t, cs, "read", map[string]any{"file": other}, "no such file")
			mustFail(t, cs, "read", map[string]any{"file": "../" + filepath.Base(roots[(i+1)%2]) + "/" + other}, "leaves the workspace root")
			// Every session of a root adds its own item; the shared edit lock keeps them all.
			mustOK(t, cs, "edit", map[string]any{"file": mine, "edits": []map[string]any{{"find": "}\n", "replace": "    item \"s" + string(rune('0'+i)) + "\"\n}\n"}}})
		}()
	}
	wg.Wait()
	for i := 0; i < 6; i++ {
		name := []string{"a.tenn", "b.tenn"}[i%2]
		if got := fileText(t, roots[i%2], name); !strings.Contains(got, "item \"s"+string(rune('0'+i))+"\"") {
			t.Errorf("edit of session %d lost: %s", i, got)
		}
	}
}

func TestServiceBadHello(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	e, err := servicewire.ReadEndpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ line, want string }{
		{"tenniarb 1 mcp wrongtoken v1 /", "error token"},
		{"GET / HTTP/1.1", "error not a tenniarb hello"},
		{"tenniarb 1 mcp " + e.Token + " v1 relative/root", "error workspace root must be an absolute path"},
		{"tenniarb 1 spy " + e.Token + " - -", "error unknown role spy"},
	} {
		c, err := net.Dial(e.Network, e.Address)
		if err != nil {
			t.Fatal(err)
		}
		c.Write([]byte(tc.line + "\n"))
		got, _ := bufio.NewReader(c).ReadString('\n')
		c.Close()
		if strings.TrimSpace(got) != tc.want {
			t.Errorf("%q: got %q, want %q", tc.line, got, tc.want)
		}
	}
}

// TestServiceRestartOnVersion: a proxy for a newer binary makes the old service stop listening; the
// proxy starts the new one, and the old session keeps working until it closes.
func TestServiceRestartOnVersion(t *testing.T) {
	dir := serviceDir(t)
	root := tempRoot(t, map[string]string{"a.tenn": "element \"A\" {\n    item \"x\"\n}\n"})
	old := startService(t, dir, "v1", nil, 0)
	oldSession := session(t, dir, root, "v1", nil)
	mustOK(t, oldSession, "get_state", nil)

	var fresh *running
	start := func() error {
		fresh = startService(t, dir, "v2", nil, 0)
		return nil
	}
	newSession := session(t, dir, root, "v2", start)
	if fresh == nil {
		t.Fatal("the proxy did not start a new service")
	}
	mustOK(t, newSession, "get_state", nil)
	mustOK(t, oldSession, "read", map[string]any{"file": "a.tenn"})
	if old.stopped(100 * time.Millisecond) {
		t.Fatal("the old service quit while its session was open")
	}
	e, _ := servicewire.ReadEndpoint(dir)
	if e.Version != "v2" {
		t.Errorf("endpoint version %q, want v2", e.Version)
	}
	oldSession.Close()
	if !old.stopped(5 * time.Second) {
		t.Error("the old service did not exit after its last session")
	}
	if fresh.stopped(50 * time.Millisecond) {
		t.Error("the new service stopped")
	}
}

type fakeClock struct {
	mu      sync.Mutex
	now     time.Duration
	waiters []fakeWaiter
}

type fakeWaiter struct {
	at time.Duration
	ch chan time.Time
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	c.waiters = append(c.waiters, fakeWaiter{c.now + d, ch})
	return ch
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now += d
	keep := c.waiters[:0]
	for _, w := range c.waiters {
		if w.at <= c.now {
			w.ch <- time.Time{}
		} else {
			keep = append(keep, w)
		}
	}
	c.waiters = keep
}

// registered waits until After has been called n times in all.
func (c *fakeClock) registered(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < 500; i++ {
		c.mu.Lock()
		got := len(c.waiters)
		c.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("idle timer not armed %d times", n)
}

func TestServiceIdleExit(t *testing.T) {
	dir := serviceDir(t)
	clock := &fakeClock{}
	r := startService(t, dir, "v1", clock, 10*time.Minute)
	clock.registered(t, 1)
	cs := session(t, dir, tempRoot(t, map[string]string{"a.tenn": "item \"x\"\n"}), "v1", nil)
	mustOK(t, cs, "get_state", nil)
	clock.Advance(time.Hour) // the timer armed before the session fires and finds a client
	if r.stopped(50 * time.Millisecond) {
		t.Fatal("exited with a session open")
	}
	cs.Close()
	clock.registered(t, 1) // armed again once the session is gone
	clock.Advance(9 * time.Minute)
	if r.stopped(50 * time.Millisecond) {
		t.Fatal("exited before the idle time")
	}
	clock.Advance(time.Minute)
	if !r.stopped(5 * time.Second) {
		t.Fatal("did not exit after the idle time")
	}
	if _, err := os.Stat(filepath.Join(dir, "endpoint")); !os.IsNotExist(err) {
		ents, _ := os.ReadDir(dir)
		t.Errorf("endpoint left after exit: %v %v", err, ents)
	}
}
