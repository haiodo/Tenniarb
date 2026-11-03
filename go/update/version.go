package update

import (
	"strconv"
	"strings"
)

// Compare reports how current stands against latest: -1 when latest is newer, 0 when both name the
// same version, 1 when latest is older. A tag may carry "v"; "-dirty", "+build" and the rest of a
// build suffix are ignored. A current that names no version at all (a commit of a checkout) is older
// than any release, and a latest that names no version is not newer.
func Compare(current, latest string) int {
	c, okCurrent := parseVersion(current)
	l, okLatest := parseVersion(latest)
	switch {
	case !okLatest:
		return 0
	case !okCurrent:
		return -1
	}
	for i := range c {
		switch {
		case l[i] > c[i]:
			return -1
		case l[i] < c[i]:
			return 1
		}
	}
	return 0
}

// parseVersion reads "2", "2.1" or "2.1.0" as numbers, the missing parts as 0. Anything else - a hash,
// "dev", a fourth number - is not a version.
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = trimV(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return v, false
	}
	parts := strings.Split(s, ".")
	if len(parts) > len(v) {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		// A leading zero is not a version: an all-digit commit hash such as 0548863 would otherwise
		// read as 548863 and look newer than every release.
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// trimV drops the tag prefix: releases are tagged "v2.1.0", the application reports "2.1.0".
func trimV(s string) string {
	if len(s) > 1 && (s[0] == 'v' || s[0] == 'V') && s[1] >= '0' && s[1] <= '9' {
		return s[1:]
	}
	return s
}
