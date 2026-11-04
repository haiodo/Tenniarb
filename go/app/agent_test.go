//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
)

func TestFollowTarget(t *testing.T) {
	for _, c := range []struct {
		name string
		p    servicewire.Presence
		want []string
	}{
		{"node wins", servicewire.Presence{Agents: 1, Node: "A.x", Element: "A"}, []string{"A.x"}},
		{"element only", servicewire.Presence{Agents: 1, Element: "A"}, []string{"A"}},
		{"no place", servicewire.Presence{Agents: 1, Action: "validate"}, nil},
		{"agent left", servicewire.Presence{Node: "A.x"}, nil},
	} {
		if got := followTarget(c.p); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAgentLabel(t *testing.T) {
	now := time.Now()
	at := func(ago time.Duration) int64 { return now.Add(-ago).UnixMilli() }
	for _, c := range []struct {
		p    servicewire.Presence
		want string
	}{
		{servicewire.Presence{}, ""},
		{servicewire.Presence{Agents: 1, At: at(time.Second)}, "● Agent"},
		{servicewire.Presence{Agents: 1, At: at(time.Minute), Name: "claude-code"}, "○ claude-code"},
		{servicewire.Presence{Agents: 2, At: at(0), Name: "claude-code"}, "● claude-code (2)"},
		{servicewire.Presence{Agents: 1, At: at(servicewire.PresenceTTL + time.Minute)}, ""},
	} {
		if got := agentLabel(c.p, now); got != c.want {
			t.Errorf("%+v: %q, want %q", c.p, got, c.want)
		}
	}
}

func TestPresenceFollow(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{}
	p := provider{sh}
	newDoc(t, sh, filepath.Join(dir, "lib.tenn"), libText)
	path := filepath.Join(dir, "screen.tenn")
	v := newDoc(t, sh, path, screenText)
	base := time.Now().UnixMilli()
	msg := func(seq int64, node string) servicewire.Presence {
		return servicewire.Presence{V: 1, Path: realPath(path), Agents: 1, Seq: seq, At: base + seq, Action: "read", Node: node}
	}

	p.Presence(msg(1, "Screen.ok"))
	if v.agent.statusText() != "● Agent" || v.selNodes != nil {
		t.Fatalf("follow off: label %q, selection %v", v.agent.statusText(), v.selNodes)
	}

	v.toggleFollow() // shows where the agent was last
	if !slices.Equal(v.selNodes, []string{"Screen.ok"}) {
		t.Fatalf("toggle on: %v", v.selNodes)
	}
	p.Presence(msg(2, "Screen"))
	if !slices.Equal(v.selNodes, []string{"Screen"}) {
		t.Fatalf("follow: %v", v.selNodes)
	}

	// An older message that arrives late changes nothing.
	p.Presence(msg(1, "Screen.ok"))
	if !slices.Equal(v.selNodes, []string{"Screen"}) || v.agent.p.Seq != 2 {
		t.Fatalf("stale message: %v seq %d", v.selNodes, v.agent.p.Seq)
	}

	// While the user types the place waits; the next poll shows it.
	v.props.changes++
	p.Presence(msg(3, "Screen.ok"))
	if !slices.Equal(v.selNodes, []string{"Screen"}) || !v.agent.pending {
		t.Fatalf("typing: %v pending %v", v.selNodes, v.agent.pending)
	}
	v.props.applied = v.props.changes
	v.agentTick()
	if !slices.Equal(v.selNodes, []string{"Screen.ok"}) || v.agent.pending {
		t.Fatalf("after typing: %v pending %v", v.selNodes, v.agent.pending)
	}

	// A node that is gone is not an error and not retried.
	p.Presence(msg(4, "Screen.gone"))
	if !slices.Equal(v.selNodes, []string{"Screen.ok"}) || v.agent.pending {
		t.Fatalf("missing node: %v pending %v", v.selNodes, v.agent.pending)
	}

	v.toggleFollow()
	p.Presence(msg(5, "Screen"))
	if !slices.Equal(v.selNodes, []string{"Screen.ok"}) {
		t.Fatalf("follow off again: %v", v.selNodes)
	}
	left := servicewire.Presence{V: 1, Path: realPath(path), Seq: 6}
	p.Presence(left)
	if v.agent.statusText() != "" {
		t.Errorf("the agent left, label %q", v.agent.statusText())
	}
}

// TestPresenceForAMountedFile: an agent working on a file the document shows as a mounted element lights
// the indicator of that window, and Follow does not jump there: the node path belongs to another file.
func TestPresenceForAMountedFile(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{}
	p := provider{sh}
	if err := os.WriteFile(filepath.Join(dir, "lib.tenn"), []byte(libText), 0o644); err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, sh, filepath.Join(dir, "screen.tenn"), screenText)
	lib := realPath(filepath.Join(dir, "lib.tenn"))
	if !v.showsFile(lib) {
		t.Fatalf("the document does not see %s as mounted: %d mounts", lib, len(v.mounts))
	}
	base := time.Now().UnixMilli()
	msg := func(seq int64) servicewire.Presence {
		return servicewire.Presence{V: 1, Path: lib, Agents: 1, Seq: seq, At: base + seq, Action: "read", Node: "ok"}
	}
	p.Presence(msg(1))
	if v.agent.statusText() != "● Agent" || v.agent.direct {
		t.Fatalf("indicator %q, direct %v", v.agent.statusText(), v.agent.direct)
	}
	v.toggleFollow()
	p.Presence(msg(2))
	if v.selNodes != nil || v.agent.pending {
		t.Fatalf("follow jumped to a path of the mounted file: %v pending %v", v.selNodes, v.agent.pending)
	}
}

// TestOpenAgentFile: only an edit opens a file no window shows, and only with the setting on.
func TestOpenAgentFile(t *testing.T) {
	for _, c := range []struct {
		name string
		p    Prefs
		pr   servicewire.Presence
	}{
		{"the setting is off", Prefs{}, servicewire.Presence{Agents: 1, Action: "edit", Path: "/tmp/a.tenn", At: 1}},
		{"a read", Prefs{OpenAgentEdits: true}, servicewire.Presence{Agents: 1, Action: "read", Path: "/tmp/a.tenn", At: 1}},
		{"the agent left", Prefs{OpenAgentEdits: true}, servicewire.Presence{Action: "edit", Path: "/tmp/a.tenn", At: 1}},
		{"not a .tenn file", Prefs{OpenAgentEdits: true}, servicewire.Presence{Agents: 1, Action: "edit", Path: "/tmp/a.txt", At: 1}},
	} {
		sh := &Shell{prefs: c.p}
		if v := sh.openAgentFile(c.pr); v != nil {
			t.Errorf("%s: opened a window", c.name)
		}
	}
}
