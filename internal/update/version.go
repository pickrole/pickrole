// Package update finds a newer PickRole release on GitHub, downloads the
// package for this installation, checks it against the release's
// SHA256SUMS and installs it (docs/adr/0024).
package update

import (
	"strconv"
	"strings"
)

// Version is a semantic version such as 0.2.0 or 0.2.0-beta.4.
type Version struct {
	Major, Minor, Patch int
	// Pre holds the dot-separated pre-release identifiers ("beta", "4");
	// empty for a final release.
	Pre []string
}

// ParseVersion reads "0.2.0-beta.4", with or without a leading "v". ok is
// false for anything else, such as the "dev" of a local build or a commit hash.
func ParseVersion(s string) (v Version, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 { // build metadata doesn't count
		s = s[:i]
	}
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return Version{}, false
		}
		nums[i] = n
	}
	v = Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}
	if hasPre {
		if pre == "" {
			return Version{}, false
		}
		v.Pre = strings.Split(pre, ".")
		for _, id := range v.Pre {
			if id == "" {
				return Version{}, false
			}
		}
	}
	return v, true
}

// Prerelease reports whether v is a pre-release (0.2.0-beta.4).
func (v Version) Prerelease() bool { return len(v.Pre) > 0 }

func (v Version) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
	if v.Prerelease() {
		s += "-" + strings.Join(v.Pre, ".")
	}
	return s
}

// Compare returns -1, 0 or 1 as v is older than, equal to or newer than w,
// following semver precedence: 0.2.0-beta.4 < 0.2.0-beta.10 < 0.2.0-rc.1 < 0.2.0.
func (v Version) Compare(w Version) int {
	for _, d := range []int{v.Major - w.Major, v.Minor - w.Minor, v.Patch - w.Patch} {
		if d != 0 {
			return sign(d)
		}
	}
	switch {
	case !v.Prerelease() && !w.Prerelease():
		return 0
	case !v.Prerelease():
		return 1
	case !w.Prerelease():
		return -1
	}
	for i := 0; i < len(v.Pre) && i < len(w.Pre); i++ {
		if c := compareIdent(v.Pre[i], w.Pre[i]); c != 0 {
			return c
		}
	}
	return sign(len(v.Pre) - len(w.Pre))
}

// compareIdent compares pre-release identifiers: numbers numerically, below
// any text; text in ASCII order.
func compareIdent(a, b string) int {
	na, errA := strconv.Atoi(a)
	nb, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return sign(na - nb)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
