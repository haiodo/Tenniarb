package servicewire

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type bufConn struct {
	*bufio.Reader
	net.Conn
}

func (c bufConn) Read(p []byte) (int, error) { return c.Reader.Read(p) }

// A SetDocuments made while the first set of a new connection is in flight must still reach the
// service.
func TestProviderResendsDocumentsChangedDuringRegister(t *testing.T) {
	dir := shortDir(t)
	l, err := Listen(dir, "v")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	inFlight, release := make(chan struct{}), make(chan struct{})
	got := make(chan []Doc, 4)
	calls := 0
	handle := func(_ context.Context, method string, params json.RawMessage) (any, error) {
		if method != MethodDocuments {
			return nil, errors.New("unexpected " + method)
		}
		var d Documents
		json.Unmarshal(params, &d)
		calls++
		if calls == 1 {
			close(inFlight)
			<-release
		}
		got <- d.Docs
		return nil, nil
	}
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		br := bufio.NewReader(c)
		line, _ := br.ReadString('\n')
		if h, _ := ParseHello(strings.TrimSuffix(line, "\n")); h.Token != l.Token {
			c.Close()
			return
		}
		c.Write([]byte("ok\n"))
		NewPeer(bufConn{br, c}, handle).Serve(context.Background())
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pc, err := RunProvider(ctx, dir, nil, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	<-inFlight
	if err := pc.SetDocuments([]Doc{{Path: "/b", Version: 1}}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("SetDocuments during register: %v, want ErrNotConnected", err)
	}
	close(release)
	if first := <-got; len(first) != 0 {
		t.Fatalf("first set %v, want empty", first)
	}
	select {
	case second := <-got:
		if len(second) != 1 || second[0].Path != "/b" {
			t.Fatalf("second set %v, want /b", second)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the set changed during register never reached the service")
	}
	wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
	defer wcancel()
	if err := pc.WaitConnected(wctx); err != nil {
		t.Fatal(err)
	}
}

type presenceProvider struct {
	Provider
	got []Presence
}

func (p *presenceProvider) Presence(pr Presence) { p.got = append(p.got, pr) }

func TestPresenceRoundTrip(t *testing.T) {
	in := Presence{V: PresenceVersion, Path: "/a.tenn", Agents: 2, Seq: 7, Name: "claude-code", Action: "edit",
		At: 5, Element: "A", Node: "A.x", Line: 2, EndLine: 3}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	pp := &presenceProvider{}
	c := &ProviderClient{p: pp}
	if _, err := c.handle(context.Background(), MethodPresence, raw); err != nil {
		t.Fatal(err)
	}
	if len(pp.got) != 1 || pp.got[0] != in {
		t.Errorf("got %+v, want %+v", pp.got, in)
	}

	// A version this reader does not know is dropped, and a provider without the interface is fine.
	newer, _ := json.Marshal(Presence{V: PresenceVersion + 1, Path: "/a.tenn", Agents: 1})
	if _, err := c.handle(context.Background(), MethodPresence, newer); err != nil || len(pp.got) != 1 {
		t.Errorf("newer version: err %v, got %d", err, len(pp.got))
	}
	plain := &ProviderClient{}
	if _, err := plain.handle(context.Background(), MethodPresence, raw); err != nil {
		t.Errorf("provider without PresenceReceiver: %v", err)
	}
}
