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
		// pbrun needs a terminal: open one running the command, so there is
		// nothing to copy (docs/adr/0028). Without a known terminal, the
		// command is shown instead.
		if err := openTerminal(i.terminalScript(pkg)); err != nil {
			return manual(ErrTerminalInstall)
		}
		return manual(ErrTerminalOpened)
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

// terminals are the emulators tried, in order, with the option that runs a
// command. gnome-terminal is RHEL's; x-terminal-emulator is Debian's choice.
var terminals = [][]string{
	{"gnome-terminal", "--"},
	{"konsole", "-e"},
	{"xfce4-terminal", "-x"},
	{"x-terminal-emulator", "-e"},
	{"xterm", "-e"},
}

// openTerminal starts a terminal window running script with sh.
func openTerminal(script string) error {
	for _, t := range terminals {
		path, err := exec.LookPath(t[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, t[1], "sh", "-c", script) // #nosec G204 -- a known terminal running our own install command
		if err := cmd.Start(); err != nil {
			continue
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
	return errors.New("no terminal found")
}

// terminalScript installs pkg and closes the terminal when it worked; when
// it didn't, the window stays open so the error can be read.
func (i Installation) terminalScript(pkg string) string {
	prompt := i.ClosePrompt
	if prompt == "" {
		prompt = "Press Enter to close."
	}
	return i.ManualCommand(pkg) + " || { echo; echo " + shellQuote(prompt) + "; read _; }"
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
