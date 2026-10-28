package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/haiodo/tenniarb/go/render"
)

// Environment variables that point the application at another config directory. The QA driver
// sets GOWT_QA_DATA, so a test run never touches the user's files.
const (
	EnvConfigDir = "TENNIARB_CONFIG_DIR"
	envQAData    = "GOWT_QA_DATA"
)

// ConfigDir is where settings, the recent list, window frames and unsaved untitled documents live.
func ConfigDir() string {
	for _, env := range []string{EnvConfigDir, envQAData} {
		if d := os.Getenv(env); d != "" {
			return d
		}
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "Tenniarb")
	}
	return filepath.Join(os.TempDir(), "Tenniarb")
}

// readJSON reads name from dir into v; a missing or malformed file leaves v as it was.
func readJSON(dir, name string, v any) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, v)
}

// writeJSON writes v to name in dir atomically, creating dir.
func writeJSON(dir, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(dir, name), data)
}

// Prefs are the settings of Swift's PreferencesController that have a consumer in the Go application,
// with the names and defaults of the Tauri version (web/packages/editor settings).
type Prefs struct {
	// AutoExpand and ExpandLevel: the outline opens the top level and ExpandLevel more.
	AutoExpand  bool `json:"autoExpand"`
	ExpandLevel int  `json:"expandLevel"`
	// Background and BackgroundDark are "#rrggbb[aa]": the canvas, and the PNG export fill.
	Background     string `json:"background"`
	BackgroundDark string `json:"backgroundDark"`
	// QuickPanel shows the quick style panel after a click on an item.
	QuickPanel bool `json:"quickPanel"`
	// ExportBackground fills the PNG export with Background; ExportNativeScale renders it at the screen scale.
	ExportBackground  bool `json:"exportBackground"`
	ExportNativeScale bool `json:"exportNativeScale"`
	// Appearance forces the colour scheme: "system" (follow the OS), "light" or "dark".
	Appearance string `json:"appearance"`
}

// DefaultPrefs are the defaults of Swift's checkDefaults.
func DefaultPrefs() Prefs {
	return Prefs{
		AutoExpand: true, ExpandLevel: 2,
		Background: "#e7e9ebff", BackgroundDark: "#2e2e2eff",
		QuickPanel: true, ExportBackground: true, ExportNativeScale: true,
		Appearance: AppearanceSystem,
	}
}

// Values of Prefs.Appearance.
const (
	AppearanceSystem = "system"
	AppearanceLight  = "light"
	AppearanceDark   = "dark"
)

const prefsFile = "settings.json"

// LoadPrefs reads settings.json over the defaults. A value out of range falls back to its default.
func LoadPrefs(dir string) Prefs {
	p := DefaultPrefs()
	readJSON(dir, prefsFile, &p)
	return p.sane()
}

// Save writes settings.json.
func (p Prefs) Save(dir string) error { return writeJSON(dir, prefsFile, p.sane()) }

func (p Prefs) sane() Prefs {
	d := DefaultPrefs()
	if p.ExpandLevel < 0 || p.ExpandLevel > 20 {
		p.ExpandLevel = d.ExpandLevel
	}
	if _, ok := render.ParseColor(p.Background); !ok {
		p.Background = d.Background
	}
	if _, ok := render.ParseColor(p.BackgroundDark); !ok {
		p.BackgroundDark = d.BackgroundDark
	}
	if p.Appearance != AppearanceLight && p.Appearance != AppearanceDark {
		p.Appearance = AppearanceSystem
	}
	return p
}
