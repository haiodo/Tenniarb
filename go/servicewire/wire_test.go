package servicewire

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// shortDir is a private temp directory with a path short enough for a unix socket.
func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "tsw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func TestHelloRoundTrip(t *testing.T) {
	h := Hello{Role: RoleMCP, Token: "abc", Version: "1-2", Root: "/a dir/with spaces"}
	line := h.line()
	got, err := ParseHello(strings.TrimSuffix(line, "\n"))
	if err != nil || got != h {
		t.Fatalf("got %+v, %v; want %+v", got, err, h)
	}
	if _, err := ParseHello("tenniarb 9 mcp t v /r"); err == nil {
		t.Error("another protocol version must be refused")
	}
	if _, err := dial(shortDir(t), Hello{Root: "/a\nb"}); err == nil || errors.Is(err, ErrNotRunning) {
		t.Errorf("a root with a newline must be refused before dialing, got %v", err)
	}
}

func TestListenSingleInstance(t *testing.T) {
	dir := shortDir(t)
	l, err := Listen(dir, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(dir, "v2"); !errors.Is(err, ErrRunning) {
		t.Fatalf("second Listen: %v, want ErrRunning", err)
	}
	e, err := ReadEndpoint(dir)
	if err != nil || e.Token != l.Token || e.Version != "v1" || e.PID != os.Getpid() {
		t.Fatalf("endpoint %+v, %v", e, err)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{endpointName, socketName} {
			st, err := os.Stat(filepath.Join(dir, name))
			if err != nil || st.Mode().Perm() != 0o600 {
				t.Errorf("%s: mode %v, %v; want 0600", name, st.Mode().Perm(), err)
			}
		}
	}
	l.Close()
	if _, err := os.Stat(filepath.Join(dir, endpointName)); !os.IsNotExist(err) {
		t.Error("Close must remove the endpoint")
	}
	l2, err := Listen(dir, "v2")
	if err != nil {
		t.Fatalf("Listen after Close: %v", err)
	}
	l2.Close()
}

func TestConnectNotRunning(t *testing.T) {
	start := time.Now()
	_, err := Connect(shortDir(t), Hello{Role: RoleMCP, Version: AnyVersion, Root: "/"}, nil, 100*time.Millisecond)
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("got %v, want ErrNotRunning", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("Connect overran its timeout")
	}
}

// TestConnectStarts checks that Connect calls start once and retries until the service listens.
func TestConnectStarts(t *testing.T) {
	dir := shortDir(t)
	starts := 0
	var l *Listener
	start := func() error {
		starts++
		go func() {
			time.Sleep(50 * time.Millisecond)
			var err error
			if l, err = Listen(dir, "v"); err != nil {
				t.Error(err)
				return
			}
			c, err := l.Accept()
			if err != nil {
				return
			}
			br := bufio.NewReader(c)
			line, _ := br.ReadString('\n')
			h, _ := ParseHello(strings.TrimSuffix(line, "\n"))
			if h.Token == l.Token && h.Root == "/w" {
				c.Write([]byte("ok\nhello"))
			}
		}()
		return nil
	}
	c, err := Connect(dir, Hello{Role: RoleMCP, Version: "v", Root: "/w"}, start, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	defer c.Close()
	buf := make([]byte, 5)
	if _, err := c.Read(buf); err != nil || string(buf) != "hello" {
		t.Fatalf("bytes after ok: %q, %v", buf, err)
	}
	if starts != 1 {
		t.Errorf("start called %d times", starts)
	}
}

func TestPeerCallsBothWays(t *testing.T) {
	a, b := net.Pipe()
	echo := func(_ context.Context, method string, params json.RawMessage) (any, error) {
		if method == "fail" {
			return nil, errors.New("boom")
		}
		var s string
		json.Unmarshal(params, &s)
		return method + ":" + s, nil
	}
	pa, pb := NewPeer(a, echo), NewPeer(b, echo)
	go pa.Serve(context.Background())
	go pb.Serve(context.Background())
	ctx := context.Background()
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			var got string
			if err := pa.Call(ctx, "x", "a", &got); err != nil || got != "x:a" {
				t.Errorf("a->b: %q %v", got, err)
			}
			if err := pb.Call(ctx, "y", "b", &got); err != nil || got != "y:b" {
				t.Errorf("b->a: %q %v", got, err)
			}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	if err := pa.Call(ctx, "fail", "", nil); err == nil || err.Error() != "boom" {
		t.Errorf("error result: %v", err)
	}
	pb.Close()
	<-pa.Done()
	if err := pa.Call(ctx, "x", "", nil); !errors.Is(err, ErrClosed) {
		t.Errorf("call after close: %v", err)
	}
}
