// tenniarb is the Tenniarb editor.
//
//	tenniarb [file.tenn | tenniarb://open?... ...]
//	tenniarb check [-json] [-root DIR] FILE...
//	tenniarb suggest [-json] [-root DIR] FILE...
//	tenniarb gen [-root DIR] [-set name=value]... FILE ELEMENT
//	tenniarb install [-dir DIR]
//	tenniarb help | -h | --help
//
// Each file opens in its own window; without files the untitled documents of the last launch come
// back, or a new document opens. Settings, the recent list and window frames live in the OS config
// directory (TENNIARB_CONFIG_DIR overrides it). check validates files headless: exit 0 clean, 1 errors, 2 usage.
// suggest reports what could be tidied in a file (repeats, styles, dead commands) and changes nothing.
// gen prints the code the template of an element generates, the parameters set by name: exit 0 done,
// 1 errors, 2 usage. Help prints the usage to stdout and exits 0; an unknown flag prints it to stderr
// and exits 2.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/haiodo/tenniarb/go/app"
	"github.com/haiodo/tenniarb/go/check"
	"github.com/haiodo/tenniarb/go/optimize"
)

const usage = `usage:
  tenniarb [FILE.tenn | LINK ...]               open the files or tenniarb:// links in the app; without files the last documents come back
  tenniarb check [-json] [-root DIR] FILE...    validate files without a window: exit 0 clean, 1 errors, 2 usage
  tenniarb suggest [-json] [-root DIR] FILE...  report what could be tidied in the files, changing nothing
  tenniarb gen [-root DIR] [-set name=value]... FILE ELEMENT
                                                print the text the template of ELEMENT generates
  tenniarb install [-dir DIR]                   write the agent skill where Claude Code and opencode read it
  tenniarb help | -h | --help                   print this text
`

// dispatch decides what the command line means before any GUI starts. launch is true when the app
// should open the returned files; otherwise code is the exit code.
func dispatch(args []string, stdout, stderr io.Writer) (launch bool, files []string, code int) {
	if len(args) == 0 {
		return true, nil, 0
	}
	switch args[0] {
	case "check":
		return false, nil, check.Run(args[1:], stdout, stderr)
	case "suggest":
		return false, nil, optimize.Run(args[1:], stdout, stderr)
	case "gen":
		return false, nil, gen(args[1:], stdout, stderr)
	case "install":
		return false, nil, install(args[1:], stdout, stderr)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return false, nil, 0
	}
	for _, a := range args {
		// AppKit adds -psn_*, -NS* and -Apple* to the command line; they are not ours to reject.
		if strings.HasPrefix(a, "-psn_") || strings.HasPrefix(a, "-NS") || strings.HasPrefix(a, "-Apple") {
			continue
		}
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "tenniarb: unknown flag %s\n%s", a, usage)
			return false, nil, 2
		}
		files = append(files, a)
	}
	return true, files, 0
}

func main() {
	launch, files, code := dispatch(os.Args[1:], os.Stdout, os.Stderr)
	if !launch {
		os.Exit(code)
	}
	// Documents are big trees that die on every edit: collect earlier, and hard above 1 GiB.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(1 << 30)
	}
	if err := app.Run(files); err != nil {
		fmt.Fprintln(os.Stderr, "tenniarb:", err)
		os.Exit(1)
	}
}
