package update

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testRelease = `{
  "tag_name": "v2.1.0",
  "name": "Tenniarb 2.1.0",
  "body": "## Changes\n- the update check",
  "draft": false,
  "prerelease": false,
  "html_url": "https://github.com/haiodo/tenniarb/releases/tag/v2.1.0",
  "assets": [
    {"name": "Tenniarb-2.1.0-universal.app.zip", "browser_download_url": "https://example.test/mac.zip", "size": 123, "digest": "sha256:aaa"},
    {"name": "Tenniarb-2.1.0-universal.dmg", "browser_download_url": "https://example.test/mac.dmg", "size": 124, "digest": "sha256:aab"},
    {"name": "tenniarb-2.1.0-windows-amd64.zip", "browser_download_url": "https://example.test/win.zip", "size": 456, "digest": "sha256:bbb"},
    {"name": "tenniarb-2.1.0-linux-amd64.tar.gz", "browser_download_url": "https://example.test/linux.tar.gz", "size": 789, "digest": "sha256:ccc"}
  ]
}`

// server answers every request with body and counts them.
func server(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent: GitHub refuses the request")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestLatest(t *testing.T) {
	srv, hits := server(t, http.StatusOK, testRelease)
	r, err := Latest(context.Background(), Options{API: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "2.1.0" || r.Name != "Tenniarb 2.1.0" || !strings.Contains(r.Notes, "update check") {
		t.Fatalf("release %+v", r)
	}
	if len(r.Assets) != 4 || r.Assets[0].URL != "https://example.test/mac.zip" || r.Assets[2].Digest != "sha256:bbb" {
		t.Fatalf("assets %+v", r.Assets)
	}
	if hits.Load() != 1 {
		t.Fatalf("%d requests", hits.Load())
	}
	if Compare("2.0.0", r.Version) != -1 {
		t.Fatal("2.1.0 is not newer than 2.0.0")
	}
}

func TestLatestErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		wantNo bool // errors.Is(err, ErrNoRelease)
	}{
		{"not found", http.StatusNotFound, `{"message":"Not Found"}`, true},
		{"server error", http.StatusInternalServerError, "boom", false},
		{"broken json", http.StatusOK, `{"tag_name":`, false},
		{"draft", http.StatusOK, `{"tag_name":"v2.1.0","draft":true}`, true},
		{"prerelease", http.StatusOK, `{"tag_name":"v2.1.0","prerelease":true}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := server(t, c.status, c.body)
			_, err := Latest(context.Background(), Options{API: srv.URL})
			if err == nil {
				t.Fatal("no error")
			}
			if errors.Is(err, ErrNoRelease) != c.wantNo {
				t.Fatalf("%v: ErrNoRelease is %v", err, !c.wantNo)
			}
		})
	}
}

func TestLatestPrerelease(t *testing.T) {
	srv, _ := server(t, http.StatusOK, `{"tag_name":"v2.1.0-rc1","prerelease":true}`)
	r, err := Latest(context.Background(), Options{API: srv.URL, Prerelease: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "2.1.0-rc1" || !r.Prerelease {
		t.Fatalf("release %+v", r)
	}
}

func TestLatestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	start := time.Now()
	_, err := Latest(context.Background(), Options{API: srv.URL, Timeout: 30 * time.Millisecond})
	if err == nil {
		t.Fatal("no error")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("the timeout did not end the request: %v", d)
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		current, latest string
		want            int
	}{
		{"2.0.0", "2.1.0", -1},
		{"2.0.0", "v2.1.0", -1},
		{"v2.0.0", "2.0.0", 0},
		{"2.1.0", "2.0.0", 1},
		{"2.0.0", "2.0.0", 0},
		{"2.0", "2.0.0", 0},
		{"2", "2.0.1", -1},
		{"2.0.0-dirty", "2.0.0", 0},
		{"2.0.0+abcd123", "2.0.0", 0},
		{"2.0.0", "2.1.0-beta", -1},
		{"2.9.0", "2.10.0", -1},
		{"1a2b3c4", "2.0.0", -1}, // a checkout names a commit, not a version
		{"dev", "2.0.0", -1},
		{"unknown", "2.0.0", -1},
		{"2.0.0", "not-a-version", 0},
		{"", "", 0},
	}
	for _, c := range cases {
		if got := Compare(c.current, c.latest); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.current, c.latest, got, c.want)
		}
	}
}

func TestPickAsset(t *testing.T) {
	var r Release
	var a apiRelease
	if err := json.Unmarshal([]byte(testRelease), &a); err != nil {
		t.Fatal(err)
	}
	for _, s := range a.Assets {
		r.Assets = append(r.Assets, Asset{Name: s.Name, URL: s.BrowserDownloadURL})
	}
	cases := []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "Tenniarb-2.1.0-universal.app.zip"},
		{"darwin", "amd64", "Tenniarb-2.1.0-universal.app.zip"},
		{"windows", "amd64", "tenniarb-2.1.0-windows-amd64.zip"},
		{"linux", "amd64", "tenniarb-2.1.0-linux-amd64.tar.gz"},
		{"linux", "arm64", ""},
		{"freebsd", "amd64", ""},
	}
	for _, c := range cases {
		got, ok := PickAsset(r, c.goos, c.goarch)
		if c.want == "" {
			if ok {
				t.Errorf("%s/%s: picked %q", c.goos, c.goarch, got.Name)
			}
			continue
		}
		if !ok || got.Name != c.want {
			t.Errorf("%s/%s: %q, want %q", c.goos, c.goarch, got.Name, c.want)
		}
	}
	if _, ok := PickAsset(Release{}, "linux", "amd64"); ok {
		t.Error("a release without assets has no file for a machine")
	}
}

// TestCache: one request a day, Force asks again, CacheOnly never does.
func TestCache(t *testing.T) {
	srv, hits := server(t, http.StatusOK, testRelease)
	dir := t.TempDir()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	check := func(o Options) (Release, error) {
		if o.Now.IsZero() {
			o.Now = now
		}
		o.API, o.Dir = srv.URL, dir
		return Latest(context.Background(), o)
	}

	if _, err := check(Options{}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("%d requests after the first check", hits.Load())
	}
	if r, err := check(Options{Now: now.Add(23 * time.Hour)}); err != nil || r.Version != "2.1.0" {
		t.Fatalf("fresh cache: %+v %v", r, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("%d requests, a fresh cache asked GitHub", hits.Load())
	}

	// a stale cache means the network, not an error
	if _, err := check(Options{Now: now.Add(25 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("%d requests, a stale cache did not ask GitHub", hits.Load())
	}

	// Force asks anyway
	if _, err := check(Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 3 {
		t.Fatalf("%d requests, Force did not ask GitHub", hits.Load())
	}

	// CacheOnly answers from the file, however old, and never asks
	if r, err := check(Options{CacheOnly: true}); err != nil || r.Version != "2.1.0" || hits.Load() != 3 {
		t.Fatalf("CacheOnly: %+v %v, %d requests", r, err, hits.Load())
	}

	// a broken cache file is a reason to ask, not an error
	if err := os.WriteFile(filepath.Join(dir, CacheFile), []byte(`{"checkedAt":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r, err := check(Options{}); err != nil || r.Version != "2.1.0" {
		t.Fatalf("broken cache: %+v %v", r, err)
	}

	// with GitHub unreachable, CacheOnly still answers from the last check
	srv.Close()
	if r, err := check(Options{CacheOnly: true}); err != nil || r.Version != "2.1.0" {
		t.Fatalf("CacheOnly offline: %+v %v", r, err)
	}
}

func TestCacheWithoutDir(t *testing.T) {
	srv, hits := server(t, http.StatusOK, testRelease)
	if _, err := Latest(context.Background(), Options{API: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := Latest(context.Background(), Options{API: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("%d requests, an empty Dir cached something", hits.Load())
	}
}

func TestCacheOnlyWithoutCache(t *testing.T) {
	srv, hits := server(t, http.StatusOK, testRelease)
	_, err := Latest(context.Background(), Options{API: srv.URL, Dir: t.TempDir(), CacheOnly: true})
	if err == nil {
		t.Fatal("CacheOnly without a cache file answered")
	}
	if hits.Load() != 0 {
		t.Fatalf("CacheOnly asked GitHub")
	}
}

// TestCacheFile checks the shape the plan asks for: {checkedAt, version, notes, assets}.
func TestCacheFile(t *testing.T) {
	srv, _ := server(t, http.StatusOK, testRelease)
	dir := t.TempDir()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	if _, err := Latest(context.Background(), Options{API: srv.URL, Dir: dir, Now: now}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, CacheFile))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		CheckedAt time.Time `json:"checkedAt"`
		Version   string    `json:"version"`
		Notes     string    `json:"notes"`
		Assets    []Asset   `json:"assets"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !got.CheckedAt.Equal(now) || got.Version != "2.1.0" || !strings.Contains(got.Notes, "update check") || len(got.Assets) != 4 {
		t.Fatalf("cache file: %+v", got)
	}
}
