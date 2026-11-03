package update

import "strings"

// PickAsset chooses the file of a release for an OS and architecture: the universal bundle on macOS,
// else the archive named after GOOS and GOARCH. A release without one for this machine gets false,
// and the caller opens the page of the release instead.
func PickAsset(r Release, goos, goarch string) (Asset, bool) {
	mac := goos == "darwin"
	want := "-" + goos + "-" + goarch
	var best Asset
	score := -1
	for _, a := range r.Assets {
		n := strings.ToLower(a.Name)
		if mac {
			if !strings.Contains(n, "universal") {
				continue
			}
		} else if !strings.Contains(n, want) {
			continue
		}
		s := 0
		if mac { // the zip is the bundle itself, the dmg asks for a drag
			switch {
			case strings.HasSuffix(n, ".app.zip"):
				s = 2
			case strings.HasSuffix(n, ".dmg"):
				s = 1
			}
		}
		if s > score {
			best, score = a, s
		}
	}
	return best, score >= 0
}
