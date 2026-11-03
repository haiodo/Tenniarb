// Package update checks the latest GitHub release of Tenniarb: its version, the notes and the files.
// The answer is cached in the settings directory, so the application does not ask GitHub on every start.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultAPI names the latest published release.
const DefaultAPI = "https://api.github.com/repos/haiodo/tenniarb/releases/latest"

// RepoURL is the page of the releases: where a file that is missing for this OS is found by hand.
const RepoURL = "https://github.com/haiodo/tenniarb"

// CacheFile holds the last check in the settings directory, next to settings.json.
const CacheFile = "update-check.json"

// Defaults of a check.
const (
	DefaultTimeout = 5 * time.Second
	DefaultMaxAge  = 24 * time.Hour
)

// ErrNoRelease is the answer of a repository without a release the check accepts: none at all, a
// draft, or a prerelease while the caller does not take them.
var ErrNoRelease = errors.New("no release")

// Release is a release as the check reports it.
type Release struct {
	Version    string  `json:"version"` // the tag without "v"
	Name       string  `json:"name"`
	Notes      string  `json:"notes"` // the body of the release: its changelog
	Prerelease bool    `json:"prerelease"`
	URL        string  `json:"url"` // the page of the release
	Assets     []Asset `json:"assets,omitempty"`
}

// Asset is one file of a release.
type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"url"` // browser_download_url
	Size   int64  `json:"size"`
	Digest string `json:"digest"` // "sha256:...", empty for an older release
}

// Page is the page of the release, for a download that no asset fits.
func (r Release) Page() string {
	if r.URL != "" {
		return r.URL
	}
	return RepoURL + "/releases"
}

// Options of a check.
type Options struct {
	Dir    string // the settings directory with CacheFile; "" keeps no cache
	API    string // DefaultAPI when empty
	Client *http.Client
	// Force ignores a fresh cache and asks GitHub; CacheOnly never asks and returns the cache even
	// when it is older than MaxAge.
	Force      bool
	CacheOnly  bool
	Prerelease bool // accept a prerelease as the latest release
	Timeout    time.Duration
	MaxAge     time.Duration
	Now        time.Time // fixed by a test; time.Now() when zero
}

// Latest is the latest release. A cache younger than MaxAge answers without the network; a stale
// cache only means the network is asked again, never an error.
func Latest(ctx context.Context, o Options) (Release, error) {
	cached, at, cacheErr := readCache(o.Dir)
	if o.CacheOnly {
		if cacheErr != nil {
			return Release{}, cacheErr
		}
		return cached, nil
	}
	if !o.Force && cacheErr == nil && nowOf(o).Sub(at) < durationOf(o.MaxAge, DefaultMaxAge) {
		return cached, nil
	}
	r, err := fetch(ctx, o)
	if err != nil {
		return Release{}, err
	}
	writeCache(o.Dir, r, nowOf(o))
	return r, nil
}

// apiRelease is the shape of GitHub's release object, of which Release keeps a part.
type apiRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	HTMLURL    string `json:"html_url"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

func fetch(ctx context.Context, o Options) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, durationOf(o.Timeout, DefaultTimeout))
	defer cancel()
	api := o.API
	if api == "" {
		api = DefaultAPI
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tenniarb-update-check")
	client := o.Client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Release{}, fmt.Errorf("%w: %s", ErrNoRelease, resp.Status)
	default:
		return Release{}, fmt.Errorf("github: %s", resp.Status)
	}
	var a apiRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&a); err != nil {
		return Release{}, fmt.Errorf("github: %w", err)
	}
	if a.Draft || (a.Prerelease && !o.Prerelease) {
		return Release{}, fmt.Errorf("%w: %s", ErrNoRelease, a.TagName)
	}
	r := Release{Version: trimV(a.TagName), Name: a.Name, Notes: a.Body, Prerelease: a.Prerelease, URL: a.HTMLURL}
	for _, s := range a.Assets {
		r.Assets = append(r.Assets, Asset{Name: s.Name, URL: s.BrowserDownloadURL, Size: s.Size, Digest: s.Digest})
	}
	return r, nil
}

// cacheEntry is CacheFile: the release and when it was fetched.
type cacheEntry struct {
	CheckedAt time.Time `json:"checkedAt"`
	Release
}

func readCache(dir string) (Release, time.Time, error) {
	if dir == "" {
		return Release{}, time.Time{}, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(dir, CacheFile))
	if err != nil {
		return Release{}, time.Time{}, err
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return Release{}, time.Time{}, err
	}
	return e.Release, e.CheckedAt, nil
}

// writeCache stores a check; a settings directory that cannot be written is not a failed check.
func writeCache(dir string, r Release, at time.Time) {
	if dir == "" {
		return
	}
	data, err := json.MarshalIndent(cacheEntry{CheckedAt: at, Release: r}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, CacheFile), data, 0o644)
}

func nowOf(o Options) time.Time {
	if o.Now.IsZero() {
		return time.Now()
	}
	return o.Now
}

func durationOf(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}
