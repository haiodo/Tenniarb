//go:build darwin || linux || windows

package app

import (
	"fmt"
	"time"

	"github.com/haiodo/gowt/look"
	"github.com/haiodo/gowt/rx"

	"github.com/haiodo/tenniarb/go/servicewire"
)

// agentActive is how long after its last action an agent counts as working now.
const agentActive = 10 * time.Second

// agentUI is what a window shows of the agents that work on its document through MCP.
type agentUI struct {
	p    servicewire.Presence // the latest message
	seen bool                 // a message came; p.Seq is the order guard
	// bar says the toolbar holds the agent items now; they come and go with the agent.
	bar        bool
	follow     bool // the user asked to watch the agent; kept while the window lives
	pending    bool // a place to show waits for the user to stop typing
	followedAt int64
	status     *rx.State[string]
	followText *rx.State[string]
}

// ensure makes the texts of the toolbar items; a window made without newViewer (tests) has none.
func (a *agentUI) ensure() {
	if a.status == nil {
		a.status, a.followText = rx.New(""), rx.New("Follow")
	}
}

// agentLabel is the text of the indicator: a filled dot while the agent acted in the last seconds, an
// empty one after, and the count when several agents work here. "" when there is no agent.
func agentLabel(p servicewire.Presence, now time.Time) string {
	if !agentPresent(p, now) {
		return ""
	}
	dot := "○"
	if now.Sub(time.UnixMilli(p.At)) < agentActive {
		dot = "●"
	}
	name := p.Name
	if name == "" {
		name = "Agent"
	}
	if p.Agents > 1 {
		return fmt.Sprintf("%s %s (%d)", dot, name, p.Agents)
	}
	return dot + " " + name
}

// agentPresent: the service clears a presence itself; one older than the TTL is a service that went
// away before it could.
func agentPresent(p servicewire.Presence, now time.Time) bool {
	return p.Agents > 0 && now.Sub(time.UnixMilli(p.At)) < servicewire.PresenceTTL
}

// followTarget is what Follow shows for p: the node the agent touched, else its element; nil when
// the agent left or the action named no place (validate, a whole-file read).
func followTarget(p servicewire.Presence) []string {
	switch {
	case p.Agents == 0:
		return nil
	case p.Node != "":
		return []string{p.Node}
	case p.Element != "":
		return []string{p.Element}
	}
	return nil
}

// Presence is called by the service client on one of its goroutines.
func (p provider) Presence(pr servicewire.Presence) {
	p.sh.ui(func() {
		if v, err := p.sh.viewer(pr.Path); err == nil {
			v.onPresence(pr)
		}
	})
}

// onPresence takes a message from the service. Messages are handled concurrently, so an older one
// than the last is dropped.
func (v *Viewer) onPresence(p servicewire.Presence) {
	a := &v.agent
	if a.seen && p.Seq <= a.p.Seq {
		return
	}
	a.p, a.seen = p, true
	if a.follow && p.At > a.followedAt && followTarget(p) != nil {
		a.pending = true
	}
	v.agentTick()
}

// agentTick brings the indicator, the toolbar and a waiting Follow up to date; the poll calls it too,
// so the dot goes empty without a message.
func (v *Viewer) agentTick() {
	a := &v.agent
	a.ensure()
	now := time.Now()
	label := agentLabel(a.p, now)
	if a.status.Peek() != label {
		a.status.Set(label)
	}
	if want := label != ""; want != a.bar && v.agentBarFree() {
		a.bar = want
		if v.win != nil {
			v.toolbar()
		}
	}
	v.followFlush()
}

// agentBarFree: the toolbar is made again when the agent comes or goes, which empties the Goto Item
// field, so not while it holds a query or the user types somewhere.
func (v *Viewer) agentBarFree() bool { return v.search.last == "" && !v.userTyping() }

// userTyping: the user is in the middle of an edit that a selection change would interrupt.
func (v *Viewer) userTyping() bool {
	return v.inline != nil || v.activeText != nil || v.asking || v.props.changes != v.props.applied ||
		(v.props.text != nil && v.propsFocused())
}

// followFlush shows the place of a waiting message unless the user is typing; it then waits for a
// later poll.
func (v *Viewer) followFlush() {
	a := &v.agent
	if !a.pending || v.session == nil {
		return
	}
	if !a.follow {
		a.pending = false
		return
	}
	if v.userTyping() {
		return
	}
	a.pending = false
	a.followedAt = a.p.At
	if paths := followTarget(a.p); paths != nil {
		// A node the agent removed since is no reason to complain.
		_ = v.followPaths(paths)
	}
}

func (v *Viewer) toggleFollow() {
	a := &v.agent
	a.ensure()
	a.follow = !a.follow
	if a.follow {
		a.followText.Set("Following")
		a.pending = followTarget(a.p) != nil
	} else {
		a.followText.Set("Follow")
		a.pending = false
	}
	v.followFlush()
}

// agentItems are the toolbar items next to the title; there are none while no agent works here.
func (v *Viewer) agentItems() []look.ToolbarItem {
	if !v.agent.bar {
		return nil
	}
	v.agent.ensure()
	return []look.ToolbarItem{
		{Title: "An agent works on this file through MCP; click to follow it", Text: v.agent.status, Small: true, ID: "agent-status", OnClick: v.toggleFollow},
		{Title: "Show where the agent edits, reads and selects", Text: v.agent.followText, Small: true, ID: "agent-follow", OnClick: v.toggleFollow},
	}
}
