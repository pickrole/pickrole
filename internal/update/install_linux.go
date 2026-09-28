package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Detect: only the packaged install in /usr/bin updates itself, with the
// package manager that installed it. A .tar.gz or a local build doesn't.
func Detect() Installation {
	exe, err := os.Executable()
	if err != nil || runtime.GOARCH != "amd64" {
		return Installation{}
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	inst := Installation{Variant: variant, Exe: exe, Elevator: findPbrun()}
	if exe != "/usr/bin/pickrole" {
		return inst
	}
	switch _, dnfErr := exec.LookPath("dnf"); {
	case variant == "el8" || dnfErr == nil:
		inst.Format = FormatRPM
	default:
		if _, err := exec.LookPath("apt-get"); err == nil {
			inst.Format = FormatDeb
		}
	}
	return inst
}

// Apply installs the downloaded package through pkexec, which asks for the
// password in a desktop dialog. When that isn't possible it returns a
// *ManualError with the command to run instead.
func (i Installation) Apply(pkg string) error {
	args := i.installCommand(pkg)
	if args == nil {
		return errors.New("this installation can't be updated automatically")
	}
	manual := func(err error) error { return &ManualError{Err: err, Command: i.ManualCommand(pkg)} }
	if i.TerminalInstall() {
		return manual(ErrTerminalInstall)
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return manual(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput() // #nosec G204 -- fixed program and options, our own downloaded file
	if err != nil {
		// 126: the password dialog was dismissed or not allowed; 127: no
		// authentication agent. Either way, the terminal still works.
		return manual(fmt.Errorf("%w: %s", err, lastLine(out)))
	}
	return nil
}

// findPbrun returns "pbrun" when it is installed. An app started from the
// application menu may have a short PATH, so the usual places are checked too.
func findPbrun() string {
	if _, err := exec.LookPath("pbrun"); err == nil {
		return "pbrun"
	}
	for _, p := range []string{"/usr/bin/pbrun", "/usr/local/bin/pbrun", "/usr/sbin/pbrun"} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return "pbrun"
		}
	}
	return ""
}

// Cleanup has nothing to do on Linux: the package manager replaced the file.
func Cleanup() {}

// lastLine is the last line of a command's output, usually the error.
func lastLine(out []byte) string {
	s := strings.TrimRight(string(out), "\r\n")
	return s[strings.LastIndexByte(s, '\n')+1:]
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}
