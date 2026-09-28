//go:build !linux && !windows

package update

import "errors"

// Detect: other systems aren't targets; they only get the notice.
func Detect() Installation { return Installation{} }

// Apply is never called here: Detect reports no supported format.
func (i Installation) Apply(string) error {
	return errors.New("updates aren't supported on this system")
}

// Cleanup has nothing to do.
func Cleanup() {}

func processAlive(int) bool { return false }
