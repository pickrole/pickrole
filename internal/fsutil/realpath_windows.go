//go:build windows

package fsutil

import (
	"strings"

	"golang.org/x/sys/windows"
)

// RealPath returns where an existing path really is, following symbolic
// links and junctions. filepath.EvalSymlinks does not follow junctions.
func RealPath(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	// FILE_FLAG_BACKUP_SEMANTICS lets CreateFile open directories too.
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read-only handle

	const size = windows.MAX_LONG_PATH
	buf := make([]uint16, size)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], size, 0)
	if err != nil {
		return "", err
	}
	final := windows.UTF16ToString(buf[:n])
	// The result is \\?\C:\… or \\?\UNC\server\share\….
	if rest, ok := strings.CutPrefix(final, `\\?\UNC\`); ok {
		return `\\` + rest, nil
	}
	return strings.TrimPrefix(final, `\\?\`), nil
}
