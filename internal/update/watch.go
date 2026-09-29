package update

import "os"

// Watch notices when the executable on disk is replaced, for instance by a
// package installed from a terminal while PickRole is open: the running
// process is still the old version until it restarts.
type Watch struct {
	path string
	info os.FileInfo
}

// NewWatch remembers the executable at path as it is now. It returns nil
// when there is nothing to watch.
func NewWatch(path string) *Watch {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return &Watch{path: path, info: info}
}

// Replaced reports whether the file at the path is no longer the one seen
// by NewWatch. A package manager writes a new file and renames it over the
// old one, so it is another file, with another size or time.
func (w *Watch) Replaced() bool {
	if w == nil {
		return false
	}
	now, err := os.Stat(w.path)
	if err != nil {
		return false // in the middle of the replacement
	}
	return !os.SameFile(w.info, now) || now.Size() != w.info.Size() || !now.ModTime().Equal(w.info.ModTime())
}
