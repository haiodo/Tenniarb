//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// guiRelease is a release newer than any build of a test: a checkout names a commit, which is older
// than every release.
const guiRelease = `{
  "tag_name": "v99.0.0",
  "name": "Tenniarb 99.0.0",
  "body": "## 99.0.0\n- notes from the test server",
  "draft": false,
  "prerelease": false,
  "html_url": "https://example.test/releases/tag/v99.0.0",
  "assets": [{"name": "tenniarb-99.0.0-linux-amd64.tar.gz", "browser_download_url": "https://example.test/linux.tar.gz", "size": 1, "digest": "sha256:aaa"}]
}`

// updateServer answers every request with status and body.
func updateServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type updateCheck struct {
	Checked bool   `json:"checked"`
	Version string `json:"version"`
	Error   string `json:"error"`
}

func updateOf(t *testing.T, app *qatest.Client) (updateCheck, bool) {
	t.Helper()
	var s struct {
		Update     updateCheck `json:"update"`
		UpdateOpen bool        `json:"updateOpen"`
	}
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s.Update, s.UpdateOpen
}

func bannerOf(t *testing.T, app *qatest.Client) string {
	t.Helper()
	var s struct{ Banner string }
	if err := json.Unmarshal(app.State("state"), &s); err != nil {
		t.Fatal(err)
	}
	return s.Banner
}

// waitFor polls until ok holds; a check runs off the UI thread, so the answer comes a moment later.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ok() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestUpdateCheck drives Help > Check for Updates against a local release server: a newer release
// opens the dialog with its notes, an older one says so, and a failed check is a line in the banner.
func TestUpdateCheck(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	cache := filepath.Join(cfgDir, "update-check.json")
	stale := func() {
		if err := os.Remove(cache); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	// the check the QA driver makes with the address of the test server
	srv := updateServer(t, http.StatusOK, guiRelease)
	app.State("update-check", "url", srv.URL)
	u, _ := updateOf(t, app)
	if !u.Checked || u.Version != "99.0.0" || u.Error != "" {
		t.Fatalf("update-check: %+v", u)
	}

	// Help > Check for Updates shows the release, its notes and the two answers
	menu(app, "Check for Updates...")
	waitFor(t, "the update dialog", func() bool {
		_, open := updateOf(t, app)
		return open
	})
	if notes := app.Find(qa.ID("update-notes")).Node().Value; !strings.Contains(notes, "notes from the test server") {
		t.Fatalf("the dialog does not show the release notes: %q", notes)
	}
	shotTitle(app, "30-update-available", "Update Available")
	app.Find(qa.ID("update-later")).Click()
	waitFor(t, "the dialog to close", func() bool {
		_, open := updateOf(t, app)
		return !open
	})

	// a release that is not newer is said in a box, not in the dialog
	same := updateServer(t, http.StatusOK, `{"tag_name":"nightly"}`)
	stale()
	app.State("update-check", "url", same.URL)
	if u, _ = updateOf(t, app); u.Error != "" || u.Version != "nightly" {
		t.Fatalf("update-check of nightly: %+v", u)
	}
	app.Answer(qa.Dialog{Kind: "message", Reply: "OK"})
	stale()
	menu(app, "Check for Updates...")
	app.Settle()
	waitFor(t, "the answer of the check", func() bool {
		_, open := updateOf(t, app)
		return !open
	})

	// a network error is a line in the banner, not a dialog
	broken := updateServer(t, http.StatusBadGateway, "no")
	app.State("update-check", "url", broken.URL)
	if u, _ = updateOf(t, app); u.Error == "" {
		t.Fatalf("a 502 is not an error: %+v", u)
	}
	stale()
	menu(app, "Check for Updates...")
	waitFor(t, "the error in the banner", func() bool {
		return strings.Contains(bannerOf(t, app), "Update check failed")
	})
	if _, open := updateOf(t, app); open {
		t.Fatal("an error opened the update dialog")
	}
}
