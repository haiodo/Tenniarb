// tenniarb is the Tenniarb editor.
//
//	tenniarb [file.tenn ...]
//	tenniarb check [-json] [-root DIR] FILE...
//
// Each file opens in its own window; without files the untitled documents of the last launch come
// back, or a new document opens. Settings, the recent list and window frames live in the OS config
// directory (TENNIARB_CONFIG_DIR overrides it). check validates files headless: exit 0 clean, 1 errors, 2 usage.
package main

import (
	"fmt"
	"os"

	"github.com/haiodo/tenniarb/go/app"
	"github.com/haiodo/tenniarb/go/check"
)

func main() {
	// check runs before any GUI init: no window, no Dock icon.
	if len(os.Args) > 1 && os.Args[1] == "check" {
		os.Exit(check.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tenniarb:", err)
		os.Exit(1)
	}
}
