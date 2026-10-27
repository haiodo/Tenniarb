//go:build unix

package servicewire

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// maxSocketPath is sun_path of macOS less the terminating zero; Linux allows 107.
const maxSocketPath = 103

func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, ErrRunning
		}
		return nil, err
	}
	return f, nil
}

func unlockFile(f *os.File) {
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}

// listen opens the unix socket of dir. The lock is held, so a socket file left there belongs to a
// service that died. The 0700 directory guards the moment before the chmod.
func listen(dir string) (net.Listener, string, string, error) {
	sock := filepath.Join(dir, socketName)
	if len(sock) > maxSocketPath {
		return nil, "", "", fmt.Errorf("socket path %s is longer than %d bytes: set %s to a shorter directory", sock, maxSocketPath, EnvDir)
	}
	os.Remove(sock)
	l, err := net.Listen("unix", sock)
	if err != nil {
		return nil, "", "", err
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := os.Chmod(sock, 0o600); err != nil {
		l.Close()
		return nil, "", "", err
	}
	return l, "unix", sock, nil
}

func cleanupSocket(dir string) { os.Remove(filepath.Join(dir, socketName)) }

func startDetached(bin string, args []string, log *os.File) (*os.Process, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
