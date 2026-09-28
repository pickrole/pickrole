package update

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Package formats PickRole is installed from.
const (
	FormatZip = "zip" // Windows, portable .exe
	FormatRPM = "rpm"
	FormatDeb = "deb"
)

// Installation is how the running copy was installed, which decides the
// package an update needs and how it's applied.
type Installation struct {
	// Format is FormatZip, FormatRPM or FormatDeb; empty when PickRole can't
	// update itself (a .tar.gz, a local build, another architecture).
	Format string
	// Variant is the Linux build: "el8" (webkit2gtk-4.0) or "webkit41".
	Variant string
	// Exe is the running executable.
	Exe string
}

// Supported reports whether PickRole can update this installation itself.
func (i Installation) Supported() bool { return i.Format != "" }

// Asset is the release file this installation needs for version v.
func (i Installation) Asset(v Version) string {
	switch {
	case i.Format == FormatZip:
		return fmt.Sprintf("pickrole_%s_windows_amd64.zip", v)
	case i.Format == FormatRPM && i.Variant == "el8":
		return fmt.Sprintf("pickrole_%s_el8_x86_64.rpm", v)
	case i.Format == FormatRPM:
		return fmt.Sprintf("pickrole_%s_fedora_x86_64.rpm", v)
	case i.Format == FormatDeb:
		return fmt.Sprintf("pickrole_%s_amd64.deb", v)
	}
	return ""
}

// installCommand is the command that installs a downloaded Linux package:
// pkexec asks for the password in a desktop dialog.
func (i Installation) installCommand(path string) []string {
	switch i.Format {
	case FormatRPM:
		return []string{"pkexec", "dnf", "install", "-y", path}
	case FormatDeb:
		return []string{"pkexec", "apt-get", "install", "-y", path}
	}
	return nil
}

// ManualCommand is what to run in a terminal when the automatic install
// isn't possible (no pkexec, or not allowed by the organization).
func (i Installation) ManualCommand(path string) string {
	switch i.Format {
	case FormatRPM:
		return "sudo dnf install " + shellQuote(path)
	case FormatDeb:
		return "sudo apt install " + shellQuote(path)
	}
	return ""
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// ManualError means the package was downloaded and verified but couldn't be
// installed automatically; Command installs it by hand.
type ManualError struct {
	Err     error
	Command string
}

func (e *ManualError) Error() string { return e.Err.Error() }
func (e *ManualError) Unwrap() error { return e.Err }

// ReplacesFlag is passed to the new version on restart, with the process id
// of the old one, which must exit first: PickRole runs a single instance.
const ReplacesFlag = "--replaces-pid="

// Relaunch starts exe, which waits for this process to exit.
func Relaunch(exe string) error {
	cmd := exec.Command(exe, ReplacesFlag+strconv.Itoa(os.Getpid())) // #nosec G204 -- our own executable, just installed
	return cmd.Start()
}

// WaitForPrevious, called first thing in main, waits (up to 15 seconds) for
// the version this process replaces to exit, when args ask for it.
func WaitForPrevious(args []string) {
	for _, a := range args {
		if pid, err := strconv.Atoi(strings.TrimPrefix(a, ReplacesFlag)); err == nil && strings.HasPrefix(a, ReplacesFlag) {
			deadline := time.Now().Add(15 * time.Second)
			for processAlive(pid) && time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
}
