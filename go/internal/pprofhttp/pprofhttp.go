// Package pprofhttp starts the Go profiler endpoints on an address a flag or an environment variable
// names. Both binaries take it as TENNIARB_PPROF, so a memory or CPU question about a running process
// is answered with `go tool pprof http://<addr>/debug/pprof/heap` instead of a rebuild.
package pprofhttp

import (
	"net"
	"net/http"
	_ "net/http/pprof"
)

// Start listens on addr and serves /debug/pprof/ in a goroutine. Callers report the address; a taken
// port is their error to show, not a reason to stop.
func Start(addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() { _ = http.Serve(listener, nil) }()
	return listener, nil
}
