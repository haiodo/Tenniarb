package servicewire

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// ErrRunning means another service holds the lock of the directory.
var ErrRunning = errors.New("another tenniarb-service runs in this directory")

// Listener is the service end: it holds the directory lock and the endpoint file for as long as it
// is open, so one service listens per directory.
type Listener struct {
	net.Listener
	Token string
	dir   string
	lock  *os.File
	once  sync.Once
}

// Listen takes the lock of dir, listens and publishes the endpoint with a fresh token. version is
// written to the endpoint for diagnostics; the check is the service's own.
func Listen(dir, version string) (*Listener, error) {
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	lock, err := lockFile(filepath.Join(dir, lockName))
	if err != nil {
		return nil, err
	}
	var raw [16]byte
	rand.Read(raw[:])
	l := &Listener{Token: hex.EncodeToString(raw[:]), dir: dir, lock: lock}
	nl, network, addr, err := listen(dir)
	if err != nil {
		unlockFile(lock)
		return nil, err
	}
	l.Listener = nl
	e := Endpoint{Network: network, Address: addr, Token: l.Token, Version: version, PID: os.Getpid()}
	if err := writeEndpoint(dir, e); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

// Close stops listening, removes the endpoint and the socket and releases the lock; a new service
// can start in the directory while connections already accepted go on.
func (l *Listener) Close() error {
	var err error
	l.once.Do(func() {
		err = l.Listener.Close()
		os.Remove(filepath.Join(l.dir, endpointName))
		cleanupSocket(l.dir)
		unlockFile(l.lock)
	})
	return err
}
