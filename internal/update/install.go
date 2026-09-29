package update

import (
	"errors"
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
	// Elevator is "pbrun" on machines that elevate privileges through it
	// (common in companies, instead of sudo). The package is then installed
	// from a terminal: pbrun needs one, and pkexec isn't allowed there.
	Elevator string
	// ClosePrompt is shown in the install terminal when the install fails,
	// in the user's language.
	ClosePrompt string
}

// ErrTerminalOpened means the install runs in a terminal PickRole opened;
// PickRole restarts when the new version is in place.
var ErrTerminalOpened = errors.New("the install runs in a terminal")

// ErrTerminalInstall means this machine elevates privileges through a tool
// that needs a terminal (pbrun), so the install is left to the user.
var ErrTerminalInstall = errors.New("this machine elevates privileges through pbrun")

// TerminalInstall reports whether the package is installed from a terminal
// rather than through the desktop password dialog.
func (i Installation) TerminalInstall() bool { return i.Elevator != "" }

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
// isn't possible (no pkexec, not allowed, or pbrun instead of sudo).
func (i Installation) ManualCommand(path string) string {
	elevate := "sudo"
	if i.Elevator != "" {
		elevate = i.Elevator
	}
	switch i.Format {
	case FormatRPM:
		return elevate + " dnf install " + shellQuote(path)
	case FormatDeb:
		return elevate + " apt install " + shellQuote(path)
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
