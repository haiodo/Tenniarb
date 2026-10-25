package scene

import (
	"fmt"
	"strings"
)

// Pos is a source position, 1-based; File is relative to the workspace root, with slashes.
type Pos struct {
	File      string
	Line, Col int
}

func (p Pos) String() string { return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col) }

// Error is a validation problem. A warning does not stop a render.
type Error struct {
	Pos  Pos
	Msg  string
	Warn bool
}

func (e Error) Error() string {
	if e.Warn {
		return fmt.Sprintf("%s: warning: %s", e.Pos, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

// Errors is the list returned by Load and Build; it is an error when it holds a non-warning.
type Errors []Error

func (es Errors) Error() string {
	lines := make([]string, len(es))
	for i, e := range es {
		lines[i] = e.Error()
	}
	return strings.Join(lines, "\n")
}

// Fatal reports whether the list holds an error that is not a warning.
func (es Errors) Fatal() bool {
	for _, e := range es {
		if !e.Warn {
			return true
		}
	}
	return false
}

// asError returns es as an error, or nil when it holds only warnings.
func (es Errors) asError() error {
	if es.Fatal() {
		return es
	}
	return nil
}
