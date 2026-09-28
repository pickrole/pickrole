package update

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

// Detect: the portable .exe updates itself wherever it is, as long as the
// folder is writable.
func Detect() Installation {
	exe, err := os.Executable()
	if err != nil || runtime.GOARCH != "amd64" {
		return Installation{}
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return Installation{Format: FormatZip, Exe: exe}
}

// Apply replaces the running .exe with the one in the downloaded .zip. A
// running .exe can't be overwritten on Windows, but it can be renamed: it
// becomes pickrole.exe.old, removed on the next start by Cleanup.
func (i Installation) Apply(zipPath string) error {
	newExe := i.Exe + ".new"
	if err := extractExe(zipPath, newExe); err != nil {
		return err
	}
	old := i.Exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(i.Exe, old); err != nil {
		_ = os.Remove(newExe)
		return err
	}
	if err := os.Rename(newExe, i.Exe); err != nil {
		_ = os.Rename(old, i.Exe) // put the running version back
		return err
	}
	return nil
}

// Cleanup removes the .old file left by the last update.
func Cleanup() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}
}

func extractExe(zipPath, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // read only
	for _, f := range r.File {
		if f.Name != "pickrole.exe" {
			continue
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		defer src.Close() //nolint:errcheck // read only

		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755) // #nosec G302 G304 -- an executable next to ours
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(src, maxAsset)); err != nil { // #nosec G110 -- limited, and the zip was checked against SHA256SUMS
			_ = out.Close()
			return err
		}
		return out.Close()
	}
	return errors.New("pickrole.exe not found in the download")
}

func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid)) // #nosec G115 -- a process id
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // best effort
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == uint32(windows.WAIT_TIMEOUT)
}
