package diff

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RevHead is the version a comparison starts from: the last commit.
const RevHead = "HEAD"

// Repo is the repository that holds dir: the top folder of the work tree, symlinks as git gives them.
// ok is false when dir is not in a repository, or git is not installed.
func Repo(dir string) (root string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	root = strings.TrimSpace(string(out))
	return root, root != ""
}

// Show reads file (a slash path relative to the repository root) as rev has it in the repository.
func Show(root, rev, file string) (string, error) {
	if rev == "" {
		rev = RevHead
	}
	out, err := exec.Command("git", "-C", root, "show", rev+":"+filepath.ToSlash(file)).Output()
	if err != nil {
		var stderr string
		if e, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(e.Stderr))
		}
		if stderr != "" {
			return "", fmt.Errorf("git show %s:%s: %s", rev, file, stderr)
		}
		return "", fmt.Errorf("git show %s:%s: %w", rev, file, err)
	}
	return string(out), nil
}
