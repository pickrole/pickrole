// Package fsutil holds small file helpers shared by the other packages.
package fsutil

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to a temporary file in the same directory and
// renames it over path, so a crash never leaves a half-written file behind.
// Missing parent directories are created with 0700.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() // the write error is the one that matters
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close() // the write error is the one that matters
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() // the write error is the one that matters
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// ReadJSON decodes the JSON file at path into v. A missing file is not an
// error: ok is false and v is left untouched.
func ReadJSON(path string, v any) (ok bool, err error) {
	data, err := os.ReadFile(path) // #nosec G304 -- only PickRole's own config, cache and state files
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, v)
}

// WriteJSON encodes v as indented JSON and writes it atomically.
func WriteJSON(path string, v any, perm fs.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'), perm)
}

// ExpandHome replaces a leading "~/" (or "~\" on Windows) with the user's
// home directory.
func ExpandHome(path string) string {
	if len(path) >= 2 && path[0] == '~' && (path[1] == '/' || path[1] == filepath.Separator) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
