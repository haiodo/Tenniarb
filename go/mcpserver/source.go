package mcpserver

import (
	"fmt"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// parseError returns the first syntax error of text, if any.
func parseError(text string) (tenn.Error, bool) {
	_, p := tenn.Parse(text)
	if !p.Errors.HasErrors() {
		return tenn.Error{}, false
	}
	return p.Errors.Errors[0], true
}

// nodeAt resolves a dotted path (spec 4.2) from the root of file rel. The node can sit in a
// mounted file: its Pos.File says which.
func nodeAt(tree *scene.Tree, rel, path string) (*scene.Node, error) {
	root := tree.File(rel)
	if root == nil {
		return nil, fmt.Errorf("%s is not loaded", rel)
	}
	n, err := tree.Resolve(root, path)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// numbered formats lines first..last (1-based) of text as "  N| line".
func numbered(text string, first, last int) string {
	var b strings.Builder
	for i, ln := range strings.Split(text, "\n") {
		n := i + 1
		if n < first || n > last {
			continue
		}
		fmt.Fprintf(&b, "%5d| %s\n", n, ln)
	}
	return b.String()
}
