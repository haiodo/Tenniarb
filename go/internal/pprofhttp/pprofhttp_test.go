package pprofhttp

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestPprof(t *testing.T) {
	listener, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	response, err := http.Get("http://" + listener.Addr().String() + "/debug/pprof/goroutine?debug=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "goroutine profile") {
		t.Fatalf("status %s, body %q", response.Status, body)
	}

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if _, err := Start(busy.Addr().String()); err == nil {
		t.Fatal("expected an error for an occupied address")
	}
}
