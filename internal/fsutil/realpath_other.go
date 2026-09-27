//go:build !windows

package fsutil

import "path/filepath"

// RealPath returns where an existing path really is, following symbolic
// links.
func RealPath(path string) (string, error) { return filepath.EvalSymlinks(path) }
