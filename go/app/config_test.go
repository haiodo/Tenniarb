package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirEnv(t *testing.T) {
	t.Setenv(EnvConfigDir, "")
	t.Setenv(envQAData, "")
	if d := ConfigDir(); filepath.Base(d) != "Tenniarb" {
		t.Fatalf("default: %s", d)
	}
	t.Setenv(envQAData, "/qa")
	if ConfigDir() != "/qa" {
		t.Fatal("QA data dir")
	}
	t.Setenv(EnvConfigDir, "/mine")
	if ConfigDir() != "/mine" {
		t.Fatal("explicit dir wins")
	}
}

func TestPrefs(t *testing.T) {
	dir := t.TempDir()
	if got := LoadPrefs(dir); got != DefaultPrefs() {
		t.Fatalf("defaults: %+v", got)
	}
	p := DefaultPrefs()
	p.ExpandLevel, p.QuickPanel, p.Background = 4, false, "#112233"
	if err := p.Save(dir); err != nil {
		t.Fatal(err)
	}
	if got := LoadPrefs(dir); got != p {
		t.Fatalf("round trip: %+v", got)
	}

	// a partial file keeps the defaults of what it lacks; bad values fall back
	os.WriteFile(filepath.Join(dir, prefsFile), []byte(`{"expandLevel":99,"background":"nonsense","autoExpand":false}`), 0o644)
	got := LoadPrefs(dir)
	d := DefaultPrefs()
	if got.AutoExpand || got.ExpandLevel != d.ExpandLevel || got.Background != d.Background || !got.ExportBackground {
		t.Fatalf("partial file: %+v", got)
	}
	os.WriteFile(filepath.Join(dir, prefsFile), []byte(`{"appearance":"purple"}`), 0o644)
	if LoadPrefs(dir).Appearance != AppearanceSystem {
		t.Fatal("unknown appearance")
	}
	p.Appearance = AppearanceDark
	p.Save(dir)
	if LoadPrefs(dir).Appearance != AppearanceDark {
		t.Fatal("appearance round trip")
	}
	os.WriteFile(filepath.Join(dir, prefsFile), []byte(`{broken`), 0o644)
	if LoadPrefs(dir) != d {
		t.Fatal("malformed file")
	}
}
