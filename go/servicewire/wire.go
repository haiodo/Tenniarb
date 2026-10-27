// Package servicewire is the wire between tenniarb-service and its clients: the tenniarb-mcp proxy
// (role mcp) and the desktop app (role provider). It uses the standard library only, so the proxy
// that links it stays small.
//
// A client reads the endpoint file in the service directory, dials the address written there and
// sends one hello line:
//
//	tenniarb 1 <role> <token> <version> <root>\n
//
// The service answers "ok", "restart" (another binary version is wanted: this service stops
// listening, start a new one) or "error <message>". After "ok" a mcp connection carries MCP
// newline-delimited JSON-RPC unchanged; a provider connection carries the messages of provider.go.
package servicewire

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	protocol     = "1"
	endpointName = "endpoint"
	lockName     = "service.lock"
	socketName   = "service.sock"
	logName      = "service.log"

	// EnvDir overrides the service directory; tests and several parallel installs use it.
	EnvDir = "TENNIARB_SERVICE_DIR"
	// EnvBin overrides the path of the tenniarb-service binary a client starts.
	EnvBin = "TENNIARB_SERVICE_BIN"

	RoleMCP      = "mcp"
	RoleProvider = "provider"
	// AnyVersion in a hello skips the version check: the app connects to whatever service runs.
	AnyVersion = "-"
)

// Dir is the service directory: the endpoint, lock, socket and log live here.
func Dir() (string, error) {
	if d := os.Getenv(EnvDir); d != "" {
		return d, nil
	}
	if runtime.GOOS == "linux" {
		if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
			return filepath.Join(d, "tenniarb"), nil
		}
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "Tenniarb", "service"), nil
}

// ServiceBin finds tenniarb-service: $TENNIARB_SERVICE_BIN, next to the running executable,
// or on PATH.
func ServiceBin() (string, error) {
	if b := os.Getenv(EnvBin); b != "" {
		return b, nil
	}
	name := "tenniarb-service"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return exec.LookPath(name)
}

// BinaryVersion identifies a binary file by size and modification time: a rebuilt or reinstalled
// service gets a new version without hashing tens of megabytes on every proxy start.
func BinaryVersion(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(st.Size(), 16) + "-" + strconv.FormatInt(st.ModTime().UnixNano(), 16), nil
}

// Endpoint is the content of the endpoint file.
type Endpoint struct {
	Network, Address, Token, Version string
	PID                              int
}

func readEndpoint(dir string) (Endpoint, error) {
	b, err := os.ReadFile(filepath.Join(dir, endpointName))
	if err != nil {
		return Endpoint{}, err
	}
	var e Endpoint
	for _, ln := range strings.Split(string(b), "\n") {
		k, v, _ := strings.Cut(ln, " ")
		switch k {
		case "network":
			e.Network = v
		case "address":
			e.Address = v
		case "token":
			e.Token = v
		case "version":
			e.Version = v
		case "pid":
			e.PID, _ = strconv.Atoi(v)
		}
	}
	if e.Network == "" || e.Address == "" || e.Token == "" {
		return Endpoint{}, fmt.Errorf("endpoint file in %s is incomplete", dir)
	}
	return e, nil
}

// ReadEndpoint returns the endpoint of the service running for dir.
func ReadEndpoint(dir string) (Endpoint, error) { return readEndpoint(dir) }

func writeEndpoint(dir string, e Endpoint) error {
	text := "tenniarb-endpoint " + protocol + "\nnetwork " + e.Network + "\naddress " + e.Address +
		"\ntoken " + e.Token + "\nversion " + e.Version + "\npid " + strconv.Itoa(e.PID) + "\n"
	tmp := filepath.Join(dir, endpointName+".tmp")
	if err := os.WriteFile(tmp, []byte(text), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, endpointName))
}

// Hello is the first line a client sends.
type Hello struct {
	Role    string
	Token   string
	Version string // AnyVersion or the BinaryVersion of the service the client wants
	Root    string // workspace root of a mcp session; may contain spaces, not newlines
}

func (h Hello) line() string {
	root := h.Root
	if root == "" {
		root = "-"
	}
	return "tenniarb " + protocol + " " + h.Role + " " + h.Token + " " + h.Version + " " + root + "\n"
}

// ParseHello parses a hello line without its newline.
func ParseHello(line string) (Hello, error) {
	f := strings.SplitN(strings.TrimRight(line, "\r"), " ", 6)
	if len(f) != 6 || f[0] != "tenniarb" {
		return Hello{}, errors.New("not a tenniarb hello")
	}
	if f[1] != protocol {
		return Hello{}, fmt.Errorf("protocol %s, this service speaks %s", f[1], protocol)
	}
	h := Hello{Role: f[2], Token: f[3], Version: f[4], Root: f[5]}
	if h.Root == "-" {
		h.Root = ""
	}
	return h, nil
}

var (
	// ErrNotRunning means no service answers in the directory.
	ErrNotRunning = errors.New("tenniarb-service is not running")
	// ErrRestart means the service runs another version and has stopped listening.
	ErrRestart = errors.New("tenniarb-service runs another version and is restarting")
	errToken   = errors.New("the service rejected the token")
)

// Conn is an accepted connection after the hello: Reader holds what the peer sent after it.
type Conn struct {
	net.Conn
	Reader *bufio.Reader
}

func (c *Conn) Read(p []byte) (int, error) { return c.Reader.Read(p) }

// CloseWrite half-closes the connection, so the peer reads EOF and can still answer.
func (c *Conn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Conn.Close()
}

// dial connects once and exchanges the hello; the token comes from the endpoint file.
func dial(dir string, h Hello) (*Conn, error) {
	if strings.ContainsAny(h.Root, "\r\n") {
		return nil, errors.New("workspace root contains a newline")
	}
	e, err := readEndpoint(dir)
	if err != nil {
		return nil, ErrNotRunning
	}
	nc, err := net.DialTimeout(e.Network, e.Address, 2*time.Second)
	if err != nil {
		return nil, ErrNotRunning
	}
	h.Token = e.Token
	nc.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := nc.Write([]byte(h.line())); err != nil {
		nc.Close()
		return nil, ErrNotRunning
	}
	br := bufio.NewReader(nc)
	resp, err := br.ReadString('\n')
	if err != nil {
		nc.Close()
		return nil, ErrNotRunning
	}
	nc.SetDeadline(time.Time{})
	resp = strings.TrimRight(resp, "\r\n")
	switch {
	case resp == "ok":
		return &Conn{Conn: nc, Reader: br}, nil
	case resp == "restart":
		nc.Close()
		return nil, ErrRestart
	case resp == "error token":
		nc.Close()
		return nil, errToken
	default:
		nc.Close()
		return nil, errors.New("tenniarb-service: " + strings.TrimPrefix(resp, "error "))
	}
}

// Dial connects to a running service without starting one.
func Dial(dir string, h Hello) (*Conn, error) { return dial(dir, h) }

// Connect dials the service of dir and starts it with start when it is not running or asks for a
// restart, retrying until timeout. start may be nil: then Connect only waits.
func Connect(dir string, h Hello, start func() error, timeout time.Duration) (*Conn, error) {
	deadline := time.Now().Add(timeout)
	wait := 20 * time.Millisecond
	starts := 0
	var lastStart time.Time
	var last error
	for {
		c, err := dial(dir, h)
		if err == nil {
			return c, nil
		}
		last = err
		switch {
		case errors.Is(err, ErrNotRunning) || errors.Is(err, ErrRestart):
			// A restart gets its own start; a start that lost the race for the lock gets another
			// after a second.
			if start != nil && starts < 3 && (starts == 0 || errors.Is(err, ErrRestart) || time.Since(lastStart) > time.Second) {
				starts++
				lastStart = time.Now()
				if serr := start(); serr != nil {
					return nil, fmt.Errorf("cannot start tenniarb-service: %w", serr)
				}
			}
		case errors.Is(err, errToken):
			// The endpoint file was replaced between reading it and dialing: read it again.
		default:
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, last
		}
		time.Sleep(wait)
		wait = min(wait*2, 400*time.Millisecond)
	}
}

// Start launches bin detached from the caller, serving dir; its output goes to service.log in dir.
func Start(bin, dir string) error {
	if err := ensureDir(dir); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(dir, logName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	p, err := startDetached(bin, []string{"-dir", dir}, log)
	if err != nil {
		return err
	}
	// Not waited for: the service outlives the caller.
	return p.Release()
}

// ensureDir creates dir for the service owner only and refuses one that others can enter.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if st.Mode().Perm()&0o077 != 0 {
		return os.Chmod(dir, 0o700)
	}
	return nil
}
