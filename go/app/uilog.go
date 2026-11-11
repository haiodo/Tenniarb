//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// uiLogPath is where the app writes what a report needs: a button whose click does nothing cannot be
// seen from outside. TENNIARB_UI_LOG points it elsewhere.
func uiLogPath() string {
	if p := os.Getenv("TENNIARB_UI_LOG"); p != "" {
		if p == "off" {
			return ""
		}
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tenniarb-ui.log")
	}
	return filepath.Join(home, "Library", "Logs", "Tenniarb", "ui.log")
}

// uilog appends one line to uiLogPath, with the time to the millisecond: the order of a click and what it
// did is the whole point.
func uilog(format string, a ...any) {
	path := uiLogPath()
	if path == "" {
		return
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > 1<<20 {
		os.Rename(path, path+".1")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, a...))
}
