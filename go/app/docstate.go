package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// NewDocumentText is a fresh document: one empty diagram, as ElementModelFactory in Swift.
const NewDocumentText = "element \"Unnamed diagram\" {\n}\n"

// RecentLimit is the length of Open Recent.
const RecentLimit = 10

const (
	recentFile  = "recent.json"
	framesFile  = "windows.json"
	untitledDir = "untitled"
)

// PushRecent puts path first: most recent first, no duplicates, at most RecentLimit.
func PushRecent(list []string, path string) []string {
	out := append([]string{path}, slices.DeleteFunc(slices.Clone(list), func(p string) bool { return p == path })...)
	if len(out) > RecentLimit {
		out = out[:RecentLimit]
	}
	return out
}

// LoadRecent reads recent.json; it is empty when the file is missing or malformed.
func LoadRecent(dir string) []string {
	var list []string
	readJSON(dir, recentFile, &list)
	return list
}

// SaveRecent writes recent.json.
func SaveRecent(dir string, list []string) error { return writeJSON(dir, recentFile, list) }

// AddRecent is a read-modify-write of recent.json; two instances adding at once can lose one entry.
func AddRecent(dir, path string) ([]string, error) {
	list := PushRecent(LoadRecent(dir), path)
	return list, SaveRecent(dir, list)
}

// Frame is a window position and size in points.
type Frame struct {
	X, Y, Width, Height float64
}

// ParseFrames reads windows.json: a frame per document path (Swift: window.pos.<uri>). Malformed
// entries are dropped.
func ParseFrames(data []byte) map[string]Frame {
	out := map[string]Frame{}
	var raw map[string]map[string]any
	if json.Unmarshal(data, &raw) != nil {
		return out
	}
	for path, f := range raw {
		var n [4]float64
		ok := true
		for i, k := range []string{"x", "y", "width", "height"} {
			v, isNum := f[k].(float64)
			if !isNum || math.IsNaN(v) || math.IsInf(v, 0) {
				ok = false
			}
			n[i] = v
		}
		if ok && n[2] > 0 && n[3] > 0 {
			out[path] = Frame{n[0], n[1], n[2], n[3]}
		}
	}
	return out
}

type frameJSON struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// LoadFrame is the remembered frame of the document at path.
func LoadFrame(dir, path string) (Frame, bool) {
	data, err := os.ReadFile(filepath.Join(dir, framesFile))
	if err != nil {
		return Frame{}, false
	}
	f, ok := ParseFrames(data)[path]
	return f, ok
}

// SaveFrame remembers the frame of the document at path; the other entries stay.
func SaveFrame(dir, path string, f Frame) error {
	data, _ := os.ReadFile(filepath.Join(dir, framesFile))
	all := map[string]frameJSON{}
	for p, old := range ParseFrames(data) {
		all[p] = frameJSON(old)
	}
	all[path] = frameJSON(f)
	return writeJSON(dir, framesFile, all)
}

// OnScreen reports whether f overlaps the screen of w x h points enough to grab its title bar.
func (f Frame) OnScreen(w, h int) bool {
	return f.X+f.Width > 40 && f.X < float64(w)-40 && f.Y > -20 && f.Y < float64(h)-40
}

// Untitled documents with edits live in <dir>/untitled/<id>.tenn, a file per window, so windows never
// race on one file and the next launch finds them.

func untitledPath(dir, id string) string { return filepath.Join(dir, untitledDir, id+".tenn") }

// StashUntitled keeps the text of an edited untitled document.
func StashUntitled(dir, id, text string) error {
	if err := os.MkdirAll(filepath.Join(dir, untitledDir), 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(untitledPath(dir, id), []byte(text))
}

// DropUntitled removes the stash of a document that was saved or discarded.
func DropUntitled(dir, id string) { _ = os.Remove(untitledPath(dir, id)) }

// StashedUntitled lists the ids of the stashed documents, the oldest first.
func StashedUntitled(dir string) []string {
	ents, _ := os.ReadDir(filepath.Join(dir, untitledDir))
	var ids []string
	for _, e := range ents {
		if id, ok := strings.CutSuffix(e.Name(), ".tenn"); ok && !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			ids = append(ids, id)
		}
	}
	return ids
}

// ReadUntitled is the stashed text.
func ReadUntitled(dir, id string) (string, error) {
	data, err := os.ReadFile(untitledPath(dir, id))
	return string(data), err
}

// NewUntitledID is a name for the stash of a new window.
func NewUntitledID() string { return "untitled-" + time.Now().Format("20060102-150405.000000000") }

// CloseAction is what a closing window does with its document.
type CloseAction int

const (
	// CloseNow: nothing to keep.
	CloseNow CloseAction = iota
	// CloseSave: a file document is flushed first, as NSDocument autosavesInPlace does.
	CloseSave
	// CloseStash: an untitled document is kept for the next launch (quit only).
	CloseStash
	// CloseAsk: Save, Don't Save or Cancel.
	CloseAsk
)

// DecideClose combines the dirty mark with autosave: a saved file is written silently, an untitled
// document has nowhere to go and asks, except on quit where it is stashed.
func DecideClose(path string, dirty, quitting bool) CloseAction {
	switch {
	case !dirty:
		return CloseNow
	case path != "":
		return CloseSave
	case quitting:
		return CloseStash
	}
	return CloseAsk
}
