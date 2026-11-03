//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	g "github.com/haiodo/gowt"
)

// mcpProxyName is the stdio MCP server the app ships next to itself: the agent starts it, it connects to
// the running app's service, and the tools live in the app.
const mcpProxyName = "tenniarb-mcp"

// clientPathDirs are where the agent CLIs live when the app is started from Finder, which does not read
// the shell PATH.
var clientPathDirs = []string{"/opt/homebrew/bin", "/usr/local/bin", ".local/bin", ".opencode/bin", ".claude/local", ".bun/bin"}

// installMCP registers the shipped MCP server in the agent CLIs on this machine and says what happened.
func (v *Viewer) installMCP() {
	proxy, err := mcpProxy()
	if err != nil {
		v.win.MessageBox(g.Message{Title: "MCP server not installed", Text: err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
		return
	}
	done, failed := registerMCP(proxy)
	title, icon := "MCP server installed", g.IconInfo
	text := strings.Join(done, "\n") + "\n\n" + proxy
	switch {
	case len(done) == 0:
		title, icon = "MCP server not installed", g.IconWarning
		text = fmt.Sprintf("No agent CLI found (opencode, claude).\n\nAdd this to your MCP configuration by hand:\n%s", proxy)
		text += "\n\n" + strings.Join(failed, "\n")
	case len(failed) > 0:
		title, icon = "MCP server installed in part", g.IconWarning
		text += "\n\nFailed:\n" + strings.Join(failed, "\n")
	}
	v.win.MessageBox(g.Message{Title: title, Text: text, Icon: icon, Buttons: g.ButtonsOK})
}

// mcpProxy is the MCP server next to the running executable: the one the package ships.
func mcpProxy() (string, error) {
	name := mcpProxyName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var tried []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		p := filepath.Join(filepath.Dir(exe), name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		tried = append(tried, p)
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if p, err := lookPathWithDirs(name); err == nil {
		return p, nil
	}
	tried = append(tried, name+" on PATH")
	return "", fmt.Errorf("%s not found; looked for:\n%s", name, strings.Join(tried, "\n"))
}

// lookPathWithDirs is LookPath plus the directories Finder-started apps do not have.
func lookPathWithDirs(name string) (string, error) {
	home, _ := os.UserHomeDir()
	for _, d := range clientPathDirs {
		if !filepath.IsAbs(d) {
			if home == "" {
				continue
			}
			d = filepath.Join(home, d)
		}
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", exec.ErrNotFound
}

// registration is one MCP client: the command that registers the proxy in it, and how it is called.
type registration struct {
	client string
	args   []string
}

// mcpRegistrations are the client CLIs that know how to register an MCP server themselves.
func mcpRegistrations(proxy string) []registration {
	return []registration{
		{client: "opencode", args: []string{"mcp", "add", "--global", "tenniarb", "--", proxy}},
		{client: "claude", args: []string{"mcp", "add", "--scope", "user", "tenniarb", "--", proxy}},
	}
}

// registerMCP runs the registration of every client CLI on this machine: what worked, and what did not.
func registerMCP(proxy string) (done, failed []string) {
	for _, r := range mcpRegistrations(proxy) {
		path, err := lookPathWithDirs(r.client)
		if err != nil {
			continue
		}
		out, err := exec.Command(path, r.args...).CombinedOutput()
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v: %s", r.client, err, strings.TrimSpace(string(out))))
			continue
		}
		done = append(done, r.client+": tenniarb registered")
	}
	return done, failed
}
