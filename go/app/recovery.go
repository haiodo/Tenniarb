package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// A run that is told to stop keeps its unsaved work here, one folder per run: a make install, a
// restart or a kill must not lose what the user or an agent has not saved yet. The next start offers
// it back (recovery.go reads it, the window shows it against the file).

// recoveryEntry is one piece of unsaved text kept from a stopped run.
type recoveryEntry struct {
	Path string    `json:"path"` // where the text belongs, absolute
	File string    `json:"file"` // the copy inside the folder, relative to it
	At   time.Time `json:"at"`
}

// recoveryManifest is what a recovery folder holds.
type recoveryManifest struct {
	Run     string          `json:"run"`
	At      time.Time       `json:"at"`
	Entries []recoveryEntry `json:"entries"`
}

// recoveryRoot is the folder the runs of this configuration write into.
func recoveryRoot(dir string) string { return filepath.Join(dir, "recovery") }

// WriteRecovery writes the texts, by absolute path, into a folder of their own under dir and returns
// it. Nothing is written when there is nothing to keep.
func WriteRecovery(dir string, texts map[string]string, now time.Time) (string, error) {
	if len(texts) == 0 {
		return "", nil
	}
	run := now.Format("20060102-150405")
	at := filepath.Join(recoveryRoot(dir), run)
	if err := os.MkdirAll(at, 0o755); err != nil {
		return "", err
	}
	paths := make([]string, 0, len(texts))
	for p := range texts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	m := recoveryManifest{Run: run, At: now}
	for i, p := range paths {
		name := fmt.Sprintf("%04d-%s", i+1, filepath.Base(p))
		if err := os.WriteFile(filepath.Join(at, name), []byte(texts[p]), 0o644); err != nil {
			return "", err
		}
		m.Entries = append(m.Entries, recoveryEntry{Path: p, File: name, At: now})
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	// The manifest goes last: a folder without one is a run that died mid-write and is not offered.
	if err := os.WriteFile(filepath.Join(at, "manifest.json"), b, 0o644); err != nil {
		return "", err
	}
	return at, nil
}

// LoadRecovery returns the newest folder under dir and what it holds; "" when a run kept nothing.
func LoadRecovery(dir string) (string, recoveryManifest, error) {
	root := recoveryRoot(dir)
	runs, err := os.ReadDir(root)
	if err != nil {
		return "", recoveryManifest{}, nil
	}
	var names []string
	for _, r := range runs {
		if r.IsDir() {
			names = append(names, r.Name())
		}
	}
	if len(names) == 0 {
		return "", recoveryManifest{}, nil
	}
	sort.Strings(names)
	var errs []error
	for i := len(names) - 1; i >= 0; i-- {
		at := filepath.Join(root, names[i])
		b, err := os.ReadFile(filepath.Join(at, "manifest.json"))
		if err != nil {
			continue
		}
		var m recoveryManifest
		if err := json.Unmarshal(b, &m); err != nil {
			errs = append(errs, err)
			continue
		}
		if len(m.Entries) != 0 {
			return at, m, nil
		}
	}
	return "", recoveryManifest{}, errors.Join(errs...)
}

// RecoveryText is the kept text of one entry.
func RecoveryText(dir, entryFile string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, entryFile))
	return string(b), err
}

// DropRecovery deletes a recovered folder, once its work is saved or thrown away.
func DropRecovery(dir string) error { return os.RemoveAll(dir) }

// saveRecovery keeps the unsaved work of every window: the document itself and the files it mounts
// that it holds unsaved. It returns the folder, "" when there was nothing to keep.
func (sh *Shell) saveRecovery() string {
	texts := map[string]string{}
	for _, v := range sh.viewers {
		if v.state.Doc.Path == "" {
			continue // an untitled document is stashed by the quit itself
		}
		if v.isDirty() {
			texts[realPath(v.state.Doc.Path)] = v.text
		}
		for file, ld := range v.docCore.libDocs {
			if ld.tracker != nil && ld.tracker.Dirty() {
				texts[v.docCore.libPath(file)] = ld.text
			}
		}
	}
	at, err := WriteRecovery(sh.dir, texts, time.Now())
	if err != nil {
		return ""
	}
	return at
}

// watchTerminate keeps the unsaved work when the process is told to stop: a make install, a restart or
// a kill cannot ask the windows to save, but the text is still not lost.
func (sh *Shell) watchTerminate() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, os.Interrupt)
	<-ch
	done := make(chan struct{})
	go func() {
		sh.ui(func() {
			sh.saveRecovery()
			close(done)
		})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	os.Exit(0)
}

// offerRecovery puts the work of a stopped run back into the windows: each kept text opens as unsaved
// changes of its file, so the user sees it as an ordinary draft, compares it and saves or drops it.
// One question, once per launch; without a window to ask, the files are left for the next launch.
func (sh *Shell) offerRecovery() {
	folder, m, err := LoadRecovery(sh.dir)
	if err != nil || folder == "" || len(m.Entries) == 0 {
		return
	}
	ask := sh.active
	if ask == nil {
		return
	}
	when := m.At.Format("15:04")
	if !ask.win.Confirm("Unsaved changes from a stopped run",
		fmt.Sprintf("Tenniarb was stopped at %s with %d unsaved file(s). Open them as unsaved changes?", when, len(m.Entries))) {
		return
	}
	if err := sh.restoreRecovery(folder, m); err != nil {
		ask.setBanner("Could not restore unsaved changes: " + err.Error())
	}
}

func (sh *Shell) restoreRecovery(folder string, m recoveryManifest) error {
	var errs []error
	for _, e := range m.Entries {
		text, err := RecoveryText(folder, e.File)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Path, err))
			continue
		}
		v, err := sh.viewer(e.Path)
		if err != nil {
			sh.openPath(e.Path)
			v, err = sh.viewer(e.Path)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Path, err))
			continue
		}
		if v.session != nil && v.text == text {
			continue
		}
		if err := v.applyText(0, len(v.text), text); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Path, err))
			continue
		}
		v.markEdited(0)
	}
	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	return DropRecovery(folder)
}
