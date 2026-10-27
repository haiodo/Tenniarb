//go:build windows

package servicewire

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"syscall"
)

const errorSharingViolation = 32

func lockFile(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// Share mode 0: a second service gets a sharing violation until this handle closes.
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errors.Is(err, syscall.Errno(errorSharingViolation)) {
			return nil, ErrRunning
		}
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

func unlockFile(f *os.File) { f.Close() }

// listen uses loopback TCP: the token of the endpoint file, which sits in the user's profile, is
// what keeps other local users out.
func listen(string) (net.Listener, string, string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", "", err
	}
	return l, "tcp", l.Addr().String(), nil
}

func cleanupSocket(string) {}

func startDetached(bin string, args []string, log *os.File) (*os.Process, error) {
	const detachedProcess = 0x00000008
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
