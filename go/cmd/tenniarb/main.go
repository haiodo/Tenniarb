// tenniarb is the Tenniarb editor.
//
//	tenniarb [file.tenn ...]
//
// Each file opens in its own window; without files the untitled documents of the last launch come
// back, or a new document opens. Settings, the recent list and window frames live in the OS config
// directory (TENNIARB_CONFIG_DIR overrides it).
package main

import (
	"fmt"
	"os"

	"github.com/haiodo/tenniarb/go/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tenniarb:", err)
		os.Exit(1)
	}
}
